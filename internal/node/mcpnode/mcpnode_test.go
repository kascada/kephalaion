package mcpnode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oklog/ulid/v2"

	"github.com/kascada/kephalaion/internal/config"
	"github.com/kascada/kephalaion/internal/contract"
	"github.com/kascada/kephalaion/internal/ident"
	"github.com/kascada/kephalaion/internal/node/replica"
	"github.com/kascada/kephalaion/internal/node/store"
)

// env ist ein Node mit zwei Hub-Einträgen und ihren Replicas: keph mit
// alice (read team-x) und bob (write team-x, read privat), team.x_y mit bob
// (supersede notizen), jeweils mit eigenem Token.
type env struct {
	nodes  store.Store
	url    string
	tokens map[string]string
}

func token(t *testing.T) string {
	t.Helper()
	tok, err := ident.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	nodes, err := store.Create(ctx, config.SQLiteDB(filepath.Join(t.TempDir(), "node.db")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = nodes.Close() })
	e := &env{nodes: nodes, tokens: map[string]string{
		"keph/alice": token(t), "keph/bob": token(t), "team.x_y/bob": token(t)}}
	hubs := map[string][]struct {
		account, collection string
		rights              contract.Rights
	}{
		"keph":     {{"alice", "team-x", contract.Rights{}}, {"bob", "team-x", contract.Rights{Write: true}}, {"bob", "privat", contract.Rights{}}},
		"team.x_y": {{"bob", "notizen", contract.Rights{Supersede: true}}},
	}
	for alias, grants := range hubs {
		if err := nodes.AddHub(ctx, store.Hub{Name: alias, NodeName: "laptop", Transport: store.TransportHTTPS,
			Address: "https://hub.example.org", Token: token(t)}, false); err != nil {
			t.Fatal(err)
		}
		var rows []contract.Row
		for i, g := range grants {
			if err := nodes.AddCollection(ctx, alias, g.collection); err != nil && !strings.Contains(err.Error(), "gibt es schon") {
				t.Fatal(err)
			}
			content, err := contract.EncodeAccountContent(contract.AccountContent{
				Hash: ident.HashToken(e.tokens[alias+"/"+g.account]), Rights: g.rights})
			if err != nil {
				t.Fatal(err)
			}
			rows = append(rows, contract.Row{ID: ulid.Make().String(), Collection: g.collection,
				Name: contract.AccountRowName(g.account), Content: &content, Revision: int64(i + 1),
				CreatedBy: "admin", UpdatedBy: "admin"})
		}
		h, err := nodes.Hub(ctx, alias)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := replica.WriteAccountRows(ctx, nodes, h, ulid.Make().String(), rows); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(NewHandler(nodes, "test"))
	t.Cleanup(srv.Close)
	e.url = srv.URL
	return e
}

// headerTransport setzt die Header, die ein Client aus seiner
// MCP-Konfiguration schickt.
type headerTransport struct {
	header http.Header
}

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range h.header {
		r.Header[k] = v
	}
	return http.DefaultTransport.RoundTrip(r)
}

// whoami verbindet sich mit dem MCP-Client des SDK, ruft whoami und liefert
// die strukturierte Antwort und den rohen JSON-Text.
func (e *env) whoami(t *testing.T, header http.Header) (WhoamiOutput, string) {
	t.Helper()
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: e.url + Path,
		HTTPClient: &http.Client{Transport: headerTransport{header}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "whoami" {
		t.Fatalf("Werkzeuge: %+v, %v", tools, err)
	}
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "whoami"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("whoami: Fehler %+v", res.Content)
	}
	raw, _ := json.Marshal(res)
	var out WhoamiOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out, string(raw)
}

func pair(alias, account, tok string) http.Header {
	h := http.Header{}
	h.Set("X-Keph-Account-"+alias, account)
	h.Set("X-Keph-Token-"+alias, tok)
	return h
}

func merge(hs ...http.Header) http.Header {
	out := http.Header{}
	for _, h := range hs {
		for k, v := range h {
			out[k] = v
		}
	}
	return out
}

func TestWhoamiWithoutHeaders(t *testing.T) {
	e := newEnv(t)
	out, raw := e.whoami(t, nil)
	if len(out.Hubs) != 0 || !strings.Contains(raw, "Keine Zugangsdaten") {
		t.Errorf("ohne Header: %+v\n%s", out, raw)
	}
}

func TestWhoami(t *testing.T) {
	e := newEnv(t)
	bob := e.tokens["keph/bob"]
	out, raw := e.whoami(t, pair("keph", "bob", bob))
	want := []HubLogin{{Hub: "keph", Authenticated: true, Account: "bob", Collections: []CollectionRights{
		{Collection: "privat", Address: "keph:privat", Rights: []string{"read"}},
		{Collection: "team-x", Address: "keph:team-x", Rights: []string{"read", "write"}},
	}}}
	if !reflect.DeepEqual(out.Hubs, want) {
		t.Errorf("bob: %+v", out.Hubs)
	}
	for _, secret := range []string{bob, ident.HashToken(bob), "keph_"} {
		if strings.Contains(raw, secret) {
			t.Errorf("Antwort enthält %q:\n%s", secret, raw)
		}
	}

	// Falsches Token und unbekannter Account: dieselbe Antwort.
	wrong, rawWrong := e.whoami(t, pair("keph", "bob", e.tokens["keph/alice"]))
	unknown, rawUnknown := e.whoami(t, pair("keph", "dave", bob))
	no := []HubLogin{{Hub: "keph"}}
	if !reflect.DeepEqual(wrong.Hubs, no) || !reflect.DeepEqual(unknown.Hubs, no) || rawWrong != rawUnknown {
		t.Errorf("falsch %+v, unbekannt %+v", wrong.Hubs, unknown.Hubs)
	}
	if strings.Contains(rawWrong, "bob") || strings.Contains(rawWrong, "dave") {
		t.Errorf("Antwort nennt den vorgelegten Account:\n%s", rawWrong)
	}

	// Unbekannter Alias: nein für diesen Alias, kein Fehler; die anderen
	// gelten getrennt. Groß- und Kleinschreibung der Header zählt nicht, ein
	// Alias mit '.' und '_' geht.
	h := merge(pair("keph", "alice", e.tokens["keph/alice"]), pair("fremd", "bob", bob))
	h["X-KEPH-ACCOUNT-TEAM.X_Y"] = []string{"bob"}
	h["x-keph-token-team.x_y"] = []string{e.tokens["team.x_y/bob"]}
	out, _ = e.whoami(t, h)
	if len(out.Hubs) != 3 || out.Hubs[0].Hub != "fremd" || out.Hubs[0].Authenticated ||
		out.Hubs[1].Hub != "keph" || out.Hubs[1].Account != "alice" || len(out.Hubs[1].Collections) != 1 ||
		out.Hubs[2].Hub != "team.x_y" || !out.Hubs[2].Authenticated ||
		!reflect.DeepEqual(out.Hubs[2].Collections[0].Rights, []string{"read", "supersede"}) {
		t.Errorf("drei Hubs: %+v", out.Hubs)
	}
	// Das Token eines Hubs gilt nicht am anderen.
	out, _ = e.whoami(t, pair("team.x_y", "bob", bob))
	if out.Hubs[0].Authenticated {
		t.Error("Token von keph gilt an team.x_y")
	}
	// Nur ein Header des Paars: nein.
	half := http.Header{}
	half.Set("X-Keph-Account-keph", "bob")
	out, _ = e.whoami(t, half)
	if !reflect.DeepEqual(out.Hubs, no) {
		t.Errorf("halbes Paar: %+v", out.Hubs)
	}
}

func TestHostAndOrigin(t *testing.T) {
	e := newEnv(t)
	_, port, _ := strings.Cut(strings.TrimPrefix(e.url, "http://"), ":")
	cases := []struct {
		host, origin string
		status       int
	}{
		{"127.0.0.1:" + port, "", http.StatusOK},
		{"localhost:" + port, "http://localhost:" + port, http.StatusOK},
		{"localhost:" + port, "http://127.0.0.1:3000", http.StatusOK},
		{"evil.example:" + port, "", http.StatusForbidden},
		{"localhost:1", "", http.StatusForbidden},
		{"localhost", "", http.StatusForbidden},
		{"127.0.0.1:" + port, "http://evil.example", http.StatusForbidden},
		{"127.0.0.1:" + port, "https://localhost:" + port, http.StatusForbidden},
		{"127.0.0.1:" + port, "null", http.StatusForbidden},
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
	for _, c := range cases {
		req, _ := http.NewRequest(http.MethodPost, e.url+Path, strings.NewReader(body))
		req.Host = c.host
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.status {
			t.Errorf("Host %q, Origin %q: HTTP %d, erwartet %d", c.host, c.origin, resp.StatusCode, c.status)
		}
	}
	resp, err := http.Get(e.url + "/anderes")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("anderer Pfad: HTTP %d", resp.StatusCode)
	}
}

func TestHubHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("X-Keph-Account-my-hub", "alice")
	h.Set("X-Keph-Token-my-hub", "keph_x")
	h.Set("X-Keph-Account-Böse", "x")
	h.Add("X-Keph-Token-doppelt", "a")
	h.Add("X-Keph-Token-doppelt", "b")
	h.Set("X-Keph-Account-doppelt", "bob")
	got := HubHeaders(h)
	want := []HubHeader{{Alias: "doppelt", Account: "bob"}, {Alias: "my-hub", Account: "alice", Token: "keph_x", Complete: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("HubHeaders = %+v", got)
	}
}
