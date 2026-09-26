package replication

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/kephalaion/kephalaion/internal/contract"
	"github.com/kephalaion/kephalaion/internal/contract/httpapi"
	"github.com/kephalaion/kephalaion/internal/hub/store"
	"github.com/kephalaion/kephalaion/internal/ident"
	"github.com/kephalaion/kephalaion/internal/sqlitedb"
)

// accounts legt die Testaccounts an: alice (read a), bob (write a, read c),
// carol ohne Collection. laptop darf a und b abgleichen. Liefert die Tokens.
func (f *fixture) accounts(t *testing.T) map[string]string {
	t.Helper()
	ctx := context.Background()
	tokens := map[string]string{}
	for _, name := range []string{"alice", "bob", "carol"} {
		tok, err := f.st.AddAccount(ctx, name, "")
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = tok
	}
	grants := []struct {
		name, coll string
		r          contract.Rights
	}{
		{"alice", "a", contract.Rights{}},
		{"bob", "a", contract.Rights{Write: true}},
		{"bob", "c", contract.Rights{}},
	}
	for _, g := range grants {
		if _, err := f.st.GrantAccount(ctx, g.name, g.coll, g.r); err != nil {
			t.Fatal(err)
		}
	}
	return tokens
}

func (f *fixture) node() contract.NodeAuth { return contract.NodeAuth{Node: "laptop", Token: f.token} }

func (f *fixture) whoami(t *testing.T, acc *contract.AccountAuth) contract.WhoamiResponse {
	t.Helper()
	resp, err := f.api().Whoami(context.Background(), contract.WhoamiRequest{Version: contract.Version, Auth: f.node(), Account: acc})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestWhoami(t *testing.T) {
	forTransports(t, func(t *testing.T, newFixture func(*testing.T) *fixture) {
		f := newFixture(t)
		tokens := f.accounts(t)
		info, _ := f.st.Info(context.Background())

		resp := f.whoami(t, nil)
		if resp.HubID != info.HubID || resp.Version != contract.Version || resp.Node != "laptop" ||
			!slices.Equal(resp.Allowed, []string{"a", "b"}) || resp.Account != nil {
			t.Errorf("whoami = %+v", resp)
		}
		// bob hat a und c; laptop hält nur a davon.
		resp = f.whoami(t, &contract.AccountAuth{Account: "bob", Token: tokens["bob"]})
		if resp.Account == nil || !resp.Account.Valid || resp.Account.Account != "bob" ||
			!slices.Equal(resp.Account.Collections, []string{"a"}) {
			t.Errorf("bob = %+v", resp.Account)
		}
		// Falsches Token, unbekannt, gesperrt: dieselbe Antwort.
		if err := f.st.SetAccountLocked(context.Background(), "alice", true); err != nil {
			t.Fatal(err)
		}
		for name, acc := range map[string]contract.AccountAuth{
			"falsches Token": {Account: "bob", Token: tokens["alice"]},
			"unbekannt":      {Account: "dave", Token: tokens["bob"]},
			"gesperrt":       {Account: "alice", Token: tokens["alice"]},
			"ungültig":       {Account: "Bob\n", Token: "x"},
		} {
			resp := f.whoami(t, &acc)
			if resp.Account == nil || resp.Account.Valid || len(resp.Account.Collections) != 0 || resp.Account.Collections == nil {
				t.Errorf("%s: %+v", name, resp.Account)
			}
		}
		// Ein Node, der nicht angemeldet ist, bekommt keine Auskunft.
		_, err := f.api().Whoami(context.Background(), contract.WhoamiRequest{Version: contract.Version,
			Auth: contract.NodeAuth{Node: "laptop", Token: tokens["bob"]}, Account: &contract.AccountAuth{Account: "bob", Token: tokens["bob"]}})
		if !errors.Is(err, contract.ErrUnauthenticated) {
			t.Errorf("Node mit Account-Token: %v", err)
		}
		_, err = f.api().Whoami(context.Background(), contract.WhoamiRequest{Version: 2, Auth: f.node()})
		if !errors.Is(err, contract.ErrUnsupportedVersion) {
			t.Errorf("Fassung 2: %v", err)
		}
	})
}

// hubState ist, was rotate am Hub ändert: Revision, Hash in accounts, die
// Zeilen des Accounts, die Zeilen rotate in actions.
type hubState struct {
	revision int64
	hash     string
	rows     string
	rotates  int
}

func (f *fixture) state(t *testing.T, account string) hubState {
	t.Helper()
	ctx := context.Background()
	info, err := f.st.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	st := hubState{revision: info.Revision}
	if a, err := f.st.Account(ctx, account); err == nil {
		st.hash = a.TokenHash
	}
	db := f.raw(t)
	rows, err := db.QueryContext(ctx, `SELECT collection, COALESCE(content, ''), deleted, revision FROM documents
		WHERE name = ? ORDER BY collection`, contract.AccountRowName(account))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var c, content string
		var deleted, rev int64
		if err := rows.Scan(&c, &content, &deleted, &rev); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s|%s|%d|%d\n", c, content, deleted, rev)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM actions WHERE action = 'rotate'`).Scan(&st.rotates); err != nil {
		t.Fatal(err)
	}
	st.rows = b.String()
	return st
}

// raw öffnet die Datenbank des Hubs daneben, nur zum Lesen im Test.
func (f *fixture) raw(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), f.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRotate(t *testing.T) {
	forTransports(t, func(t *testing.T, newFixture func(*testing.T) *fixture) {
		f := newFixture(t)
		ctx := context.Background()
		tokens := f.accounts(t)
		newTok, _ := ident.NewToken()
		req := func(account, token string) contract.RotateRequest {
			return contract.RotateRequest{Version: contract.Version, Auth: f.node(), Account: account, Token: token,
				NewHash: ident.HashToken(newTok)}
		}

		// Fehler ändern nichts und stehen nicht in actions.
		before := f.state(t, "bob")
		failures := []struct {
			name string
			req  contract.RotateRequest
			want error
		}{
			{"falsches Token", req("bob", tokens["alice"]), contract.ErrAccountUnauthenticated},
			{"unbekannt", req("dave", tokens["bob"]), contract.ErrAccountUnauthenticated},
			{"Hash kaputt", func() contract.RotateRequest { r := req("bob", tokens["bob"]); r.NewHash = "xyz"; return r }(), contract.ErrInvalid},
			{"Node falsch", func() contract.RotateRequest { r := req("bob", tokens["bob"]); r.Auth.Token = tokens["bob"]; return r }(), contract.ErrUnauthenticated},
			{"Fassung 2", func() contract.RotateRequest { r := req("bob", tokens["bob"]); r.Version = 2; return r }(), contract.ErrUnsupportedVersion},
		}
		for _, c := range failures {
			if _, err := f.api().Rotate(ctx, c.req); !errors.Is(err, c.want) {
				t.Errorf("%s: %v, erwartet %v", c.name, err, c.want)
			}
		}
		if after := f.state(t, "bob"); after != before {
			t.Errorf("Fehlversuche haben geändert:\n%+v\n%+v", before, after)
		}
		// carol hat keine Collection: no_shared_collection, ohne Änderung.
		beforeCarol := f.state(t, "carol")
		if _, err := f.api().Rotate(ctx, req("carol", tokens["carol"])); !errors.Is(err, contract.ErrNoSharedCollection) {
			t.Errorf("carol: %v", err)
		}
		if after := f.state(t, "carol"); after != beforeCarol {
			t.Errorf("no_shared_collection hat geändert:\n%+v\n%+v", beforeCarol, after)
		}

		// bob: ein Schreibvorgang, beide Zeilen, eine Zeile in actions; die
		// Antwort trägt nur a — c darf laptop nicht abgleichen.
		resp, err := f.api().Rotate(ctx, req("bob", tokens["bob"]))
		if err != nil {
			t.Fatal(err)
		}
		after := f.state(t, "bob")
		info, _ := f.st.Info(ctx)
		if after.revision != before.revision+1 || after.hash != ident.HashToken(newTok) || after.rotates != 1 ||
			strings.Count(after.rows, ident.HashToken(newTok)) != 2 || strings.Contains(after.rows, before.hash) {
			t.Errorf("nach rotate: %+v", after)
		}
		if resp.HubID != info.HubID || resp.Version != contract.Version || len(resp.Rows) != 1 ||
			resp.Rows[0].Collection != "a" || resp.Rows[0].Name != "SYSTEM:A:bob" || resp.Rows[0].Revision != after.revision {
			t.Errorf("Antwort = %+v", resp)
		}
		var a sql.NullString
		if err := f.raw(t).QueryRow(`SELECT carrier FROM actions WHERE action = 'rotate'`).Scan(&a); err != nil || a.String != "laptop" {
			t.Errorf("carrier = %v, %v", a, err)
		}
		// Das alte Token gilt nicht mehr, das neue schon.
		if _, err := f.api().Rotate(ctx, req("bob", tokens["bob"])); !errors.Is(err, contract.ErrAccountUnauthenticated) {
			t.Errorf("altes Token: %v", err)
		}
		if w := f.whoami(t, &contract.AccountAuth{Account: "bob", Token: newTok}); !w.Account.Valid {
			t.Error("neues Token gilt nicht")
		}

		// Gesperrt: kein rotate; gesperrter Node ebenso.
		if err := f.st.SetAccountLocked(ctx, "alice", true); err != nil {
			t.Fatal(err)
		}
		if _, err := f.api().Rotate(ctx, req("alice", tokens["alice"])); !errors.Is(err, contract.ErrAccountUnauthenticated) {
			t.Errorf("gesperrter Account: %v", err)
		}
		if err := f.st.SetAccountLocked(ctx, "alice", false); err != nil {
			t.Fatal(err)
		}
		if err := f.st.SetNodeLocked(ctx, "laptop", true); err != nil {
			t.Fatal(err)
		}
		if _, err := f.api().Rotate(ctx, req("alice", tokens["alice"])); !errors.Is(err, contract.ErrUnauthenticated) {
			t.Errorf("gesperrter Node: %v", err)
		}
	})
}

// TestHTTPStatus prüft über HTTP jeden Fehlercode mit seinem Status, die
// fremde Fassung und einen zu großen Body, gegen den echten Hub.
func TestHTTPStatus(t *testing.T) {
	f := newFixtureOver(t, "http")
	tokens := f.accounts(t)
	post := func(path string, node, token string, body any) (int, string) {
		t.Helper()
		var data []byte
		switch b := body.(type) {
		case []byte:
			data = b
		default:
			data, _ = json.Marshal(b)
		}
		req, _ := http.NewRequest(http.MethodPost, f.url+path, bytes.NewReader(data))
		req.Header.Set(httpapi.HeaderNode, node)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var eb struct{ Code string }
		_ = json.NewDecoder(resp.Body).Decode(&eb)
		return resp.StatusCode, eb.Code
	}
	newHash := ident.HashToken("keph_x")
	cases := []struct {
		name, path, node, token string
		body                    any
		status                  int
		code                    string
	}{
		{"ok", "/v1/whoami", "laptop", f.token, map[string]any{}, 200, ""},
		{"unauthenticated", "/v1/sync", "laptop", "keph_falsch", map[string]any{"page_size": 1}, 401, "unauthenticated"},
		{"account_unauthenticated", "/v1/rotate", "laptop", f.token,
			map[string]any{"account": "bob", "token": "keph_falsch", "new_hash": newHash}, 403, "account_unauthenticated"},
		{"no_shared_collection", "/v1/rotate", "laptop", f.token,
			map[string]any{"account": "carol", "token": tokens["carol"], "new_hash": newHash}, 409, "no_shared_collection"},
		{"invalid", "/v1/sync", "laptop", f.token, map[string]any{"page_size": 0}, 400, "invalid"},
		{"kein JSON", "/v1/sync", "laptop", f.token, []byte("{kaputt"), 400, "invalid"},
		{"fremde Fassung", "/v2/sync", "", "", map[string]any{}, 404, "unsupported_version"},
		{"unbekannter Vorgang", "/v1/gibtsnicht", "laptop", f.token, map[string]any{}, 404, "invalid"},
		{"zu groß", "/v1/sync", "laptop", f.token, bytes.Repeat([]byte(" "), httpapi.MaxBodyBytes+1), 413, "invalid"},
	}
	for _, c := range cases {
		status, code := post(c.path, c.node, c.token, c.body)
		if status != c.status || code != c.code {
			t.Errorf("%s: HTTP %d %q, erwartet %d %q", c.name, status, code, c.status, c.code)
		}
	}
	for _, code := range contract.Codes {
		if s := httpapi.Status(code); s < 400 || s == 500 {
			t.Errorf("Code %s ohne eigenen Status: %d", code, s)
		}
	}
}

// infoAfterRotate lässt Info scheitern, sobald RotateAccount gelaufen ist —
// wie eine Datenbank, die nach dem Commit ausfällt.
type infoAfterRotate struct {
	store.Store
	rotated bool
}

func (s *infoAfterRotate) RotateAccount(ctx context.Context, name, oldHash, newHash, carrier string, shared []string) ([]contract.Row, error) {
	rows, err := s.Store.RotateAccount(ctx, name, oldHash, newHash, carrier, shared)
	s.rotated = s.rotated || err == nil
	return rows, err
}

func (s *infoAfterRotate) Info(ctx context.Context) (store.Info, error) {
	if s.rotated {
		return store.Info{}, errors.New("Datenbank weg")
	}
	return s.Store.Info(ctx)
}

// Nach dem Commit liest Rotate nichts mehr: Die Antwort steht vorher fest,
// ein Fehler danach kann den Erfolg nicht mehr kippen.
func TestRotateNothingAfterCommit(t *testing.T) {
	f := newLocalFixture(t)
	tokens := f.accounts(t)
	st := &infoAfterRotate{Store: f.st}
	newTok, _ := ident.NewToken()
	resp, err := New(st).Rotate(context.Background(), contract.RotateRequest{Version: contract.Version, Auth: f.node(),
		Account: "bob", Token: tokens["bob"], NewHash: ident.HashToken(newTok)})
	if err != nil {
		t.Fatalf("Rotate nach dem Commit gescheitert: %v", err)
	}
	info, _ := f.st.Info(context.Background())
	if !st.rotated || resp.HubID != info.HubID || len(resp.Rows) != 1 {
		t.Errorf("Antwort = %+v", resp)
	}
}
