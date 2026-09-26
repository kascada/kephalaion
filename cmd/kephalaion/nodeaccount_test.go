package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kascada/kephalaion/internal/contract"
	"github.com/kascada/kephalaion/internal/contract/httpapi"
	"github.com/kascada/kephalaion/internal/hub/replication"
	"github.com/kascada/kephalaion/internal/node/replica"
)

// commEnv ist Hub und Node in einer config mit den Testaccounts: Collections
// team-x und privat; alice (read team-x), bob (write team-x, read privat),
// carol (supersede team-x). Der Node laptop (angelegt mit --create) erreicht
// den Hub über local als „eigen“, der Node laptop-http über HTTP als „fern“;
// beide dürfen beide Collections und wollen team-x.
type commEnv struct {
	dir, cfg, c string
	tokens      map[string]string
	url         string
}

func newCommEnv(t *testing.T) *commEnv {
	t.Helper()
	dir := isolate(t)
	e := &commEnv{dir: dir, cfg: setup(t, dir), tokens: map[string]string{}}
	e.c = "--config=" + e.cfg
	for _, coll := range []string{"team-x", "privat"} {
		e.run(t, "hub", "collection", "add", coll).want(t, 0)
	}
	for _, name := range []string{"alice", "bob", "carol"} {
		r := e.run(t, "hub", "account", "add", name)
		r.want(t, 0)
		e.tokens[name] = tokenFrom(t, r.out)
	}
	e.run(t, "hub", "account", "grant", "alice", "team-x").want(t, 0)
	e.run(t, "hub", "account", "grant", "bob", "team-x", "--write").want(t, 0)
	e.run(t, "hub", "account", "grant", "bob", "privat").want(t, 0)
	e.run(t, "hub", "account", "grant", "carol", "team-x", "--supersede").want(t, 0)

	e.run(t, "node", "hub", "add", "eigen", "--node", "laptop", "--transport", "local", "--create").want(t, 0,
		"Node laptop am Hub angelegt")
	r := e.run(t, "hub", "node", "add", "laptop-http")
	r.want(t, 0)
	nodeTok := tokenFrom(t, r.out)

	hs := hubStore(t, e.cfg)
	srv := httptest.NewServer(httpapi.NewHandler(replication.New(hs)))
	t.Cleanup(srv.Close)
	e.url = srv.URL
	runIn(t, nodeTok, "node", "hub", "add", "fern", "--node", "laptop-http", "--transport", "http",
		"--address", srv.URL, "--token-stdin", e.c).want(t, 0)
	for _, node := range []string{"laptop", "laptop-http"} {
		for _, coll := range []string{"team-x", "privat"} {
			e.run(t, "hub", "node", "grant", node, coll).want(t, 0)
		}
	}
	e.run(t, "node", "collection", "add", "eigen:team-x").want(t, 0)
	e.run(t, "node", "collection", "add", "fern:team-x").want(t, 0)
	return e
}

func (e *commEnv) run(t *testing.T, args ...string) result {
	t.Helper()
	return runIn(t, "", append(args, e.c)...)
}

func (e *commEnv) runIn(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	return runIn(t, stdin, append(args, e.c)...)
}

// tokenFile legt eine Token-Datei an.
func (e *commEnv) tokenFile(t *testing.T, name, token string) string {
	t.Helper()
	p := filepath.Join(e.dir, name+".token")
	if err := os.WriteFile(p, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readFileToken(t *testing.T, path string) string {
	t.Helper()
	tok, err := readTokenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// replicaAccount liest die Collections der Account-Zeilen aus der Replica
// eines Eintrags.
func (e *commEnv) replicaAccount(t *testing.T, alias, account string) []string {
	t.Helper()
	ns := nodeStore(t, e.cfg)
	r, err := replica.Open(context.Background(), ns.ReplicaPath(alias))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	rows, err := r.AccountRows(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, row := range rows {
		out = append(out, row.Collection)
	}
	return out
}

func TestNodeHubCreateAndCheck(t *testing.T) {
	e := newCommEnv(t)
	r := e.run(t, "hub", "node", "show", "laptop")
	r.want(t, 0, "Node laptop")
	// --create zeigt das Token nicht.
	e.run(t, "node", "hub", "add", "eigen2", "--node", "laptop", "--transport", "local", "--create").want(t, 1)
	e.run(t, "node", "hub", "add", "x", "--node", "neu", "--transport", "http", "--address", "http://localhost:1", "--create").
		want(t, 1, "nur mit --transport local")
	e.runIn(t, "keph_x", "node", "hub", "add", "x", "--node", "neu", "--transport", "local", "--create", "--token-stdin").
		want(t, 1, "--token-stdin passt nicht")
	if _, err := hubStore(t, e.cfg).Node(context.Background(), "neu"); err == nil {
		t.Error("abgelehntes --create hat den Node angelegt")
	}

	for _, alias := range []string{"eigen", "fern"} {
		r := e.run(t, "node", "hub", "check", alias)
		r.want(t, 0, "Hub "+alias+": erreichbar", "erster Kontakt, gemerkt", "erlaubt:      privat, team-x")
		if strings.Contains(r.out, "keph_") {
			t.Errorf("check zeigt ein Token:\n%s", r.out)
		}
		r = e.run(t, "node", "hub", "check", alias)
		if strings.Contains(r.out, "erster Kontakt") {
			t.Errorf("zweiter check: %s", r.out)
		}
	}
	info, _ := hubStore(t, e.cfg).Info(context.Background())
	e.run(t, "node", "hub", "show", "fern").want(t, 0, "hub_id:       "+info.HubID)
	// Gesperrter Node: check scheitert.
	e.run(t, "hub", "node", "lock", "laptop-http").want(t, 0)
	e.run(t, "node", "hub", "check", "fern").want(t, 1, "nicht angemeldet")
}

func TestNodeSyncOverHTTP(t *testing.T) {
	e := newCommEnv(t)
	e.runIn(t, "Inhalt", "hub", "doc", "put", "team-x", "a.md").want(t, 0)
	e.run(t, "node", "sync", "fern").want(t, 0, "Hub fern", "team-x: abgeglichen")
	e.run(t, "node", "doc", "get", "fern:team-x", "a.md").want(t, 0, "Inhalt")
	// Die Account-Zeilen kamen mit.
	if got := e.replicaAccount(t, "fern", "bob"); !slices.Equal(got, []string{"team-x"}) {
		t.Errorf("bob in der Replica: %v", got)
	}
}

func TestAccountRotateBothTransports(t *testing.T) {
	e := newCommEnv(t)
	// alice über local, mit Datei.
	file := e.tokenFile(t, "alice", e.tokens["alice"])
	r := e.run(t, "node", "account", "rotate", "eigen", "alice", "--token-file", file)
	r.want(t, 0, "Account alice: Token ersetzt", "Replica eigen: Account alice bekannt in team-x")
	newAlice := readFileToken(t, file)
	if newAlice == e.tokens["alice"] || strings.Contains(r.out, newAlice) {
		t.Errorf("Datei nicht ersetzt oder Token angezeigt:\n%s", r.out)
	}
	if ok, _ := fileExists(file + ".pending"); ok {
		t.Error(".pending liegt noch")
	}
	if fi, _ := os.Stat(file); fi.Mode().Perm() != 0o600 {
		t.Errorf("Token-Datei %v", fi.Mode().Perm())
	}
	if got := e.replicaAccount(t, "eigen", "alice"); !slices.Equal(got, []string{"team-x"}) {
		t.Errorf("alice in der Replica: %v", got)
	}
	e.run(t, "node", "account", "check", "eigen", "alice", "--token-file", file).want(t, 0, "gilt", "team-x")
	e.runIn(t, e.tokens["alice"], "node", "account", "check", "eigen", "alice", "--token-stdin").want(t, 1, "gilt am Hub eigen nicht")

	// bob über http, mit stdin; bob hat team-x und privat, der Node will nur
	// team-x.
	r = e.runIn(t, e.tokens["bob"], "node", "account", "rotate", "fern", "bob", "--token-stdin")
	r.want(t, 0, "Neues Token", "Replica fern: Account bob bekannt in team-x")
	newBob := tokenFrom(t, r.out)
	if got := e.replicaAccount(t, "fern", "bob"); !slices.Equal(got, []string{"team-x"}) {
		t.Errorf("bob in der Replica: %v", got)
	}
	e.runIn(t, newBob, "node", "account", "check", "fern", "bob", "--token-stdin").want(t, 0, "privat, team-x")
	// Das alte Token taugt nicht für einen zweiten rotate.
	e.runIn(t, e.tokens["bob"], "node", "account", "rotate", "fern", "bob", "--token-stdin").
		want(t, 1, "rotate gescheitert, das alte Token gilt weiter", "Account nicht angemeldet")

	// Nichts gewünscht: Der Node meldet es, die Replica bleibt.
	e.run(t, "node", "collection", "rm", "eigen:team-x").want(t, 0)
	e.runIn(t, e.tokens["carol"], "node", "account", "rotate", "eigen", "carol", "--token-stdin").
		want(t, 0, "will keine der Collections")
}

// hookHTTP ersetzt connectHTTP für einen Test.
func hookHTTP(t *testing.T, fn func(address string) (contract.Hub, error)) {
	t.Helper()
	old := connectHTTP
	connectHTTP = fn
	t.Cleanup(func() { connectHTTP = old })
}

// lostAnswer führt rotate am echten Hub aus (oder nicht) und meldet dann
// einen unklaren Ausgang — wie eine Antwort, die unterwegs verloren ging.
type lostAnswer struct {
	contract.Hub
	apply bool
}

func (l lostAnswer) Rotate(ctx context.Context, req contract.RotateRequest) (contract.RotateResponse, error) {
	if l.apply {
		if _, err := l.Hub.Rotate(ctx, req); err != nil {
			return contract.RotateResponse{}, err
		}
	}
	return contract.RotateResponse{}, contract.ErrOutcomeUnknown
}

func TestAccountRotatePending(t *testing.T) {
	e := newCommEnv(t)
	apply := true
	hookHTTP(t, func(address string) (contract.Hub, error) {
		c, err := httpapi.NewClient(address)
		return lostAnswer{Hub: c, apply: apply}, err
	})

	// Eindeutig gescheitert: Datei bleibt, .pending weg.
	file := e.tokenFile(t, "bob", e.tokens["alice"])
	hookHTTP(t, func(address string) (contract.Hub, error) { return httpapi.NewClient(address) })
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 1, "Token-Datei bleibt unverändert")
	if readFileToken(t, file) != e.tokens["alice"] {
		t.Error("Datei verändert")
	}
	if ok, _ := fileExists(file + ".pending"); ok {
		t.Error(".pending nach eindeutigem Fehler")
	}

	// Unklar, der Hub hat rotiert: beide bleiben, rotate ist gesperrt, check
	// übernimmt das neue.
	hookHTTP(t, func(address string) (contract.Hub, error) {
		c, err := httpapi.NewClient(address)
		return lostAnswer{Hub: c, apply: apply}, err
	})
	file = e.tokenFile(t, "bob", e.tokens["bob"])
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 1, "Ausgang unklar", "Beide Dateien bleiben",
		"node account check fern bob --token-file")
	pending := readFileToken(t, file+".pending")
	if readFileToken(t, file) != e.tokens["bob"] {
		t.Error("Datei trotz unklarem Ausgang ersetzt")
	}
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 1, "liegt noch", "node account check")
	e.run(t, "node", "account", "check", "fern", "bob", "--token-file", file).want(t, 0, "Das neue Token gilt")
	if readFileToken(t, file) != pending {
		t.Error("check hat die Datei nicht ersetzt")
	}
	if ok, _ := fileExists(file + ".pending"); ok {
		t.Error(".pending nach check")
	}

	// Unklar, der Hub hat nicht rotiert: check behält das alte.
	apply = false
	before := readFileToken(t, file)
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 1, "Ausgang unklar")
	e.run(t, "node", "account", "check", "fern", "bob", "--token-file", file).want(t, 0, "Das alte Token gilt")
	if readFileToken(t, file) != before {
		t.Error("check hat das alte Token ersetzt")
	}
	if ok, _ := fileExists(file + ".pending"); ok {
		t.Error(".pending nach check")
	}

	// Unklar über stdin: das neue Token kommt, markiert.
	r := e.runIn(t, before, "node", "account", "rotate", "fern", "bob", "--token-stdin")
	r.want(t, 1, "UNKLAR — PRÜFEN", "node account check fern bob --token-stdin")
	tokenFrom(t, r.out)

	// Keines gilt (gesperrt): beide bleiben.
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 1, "Ausgang unklar")
	e.run(t, "hub", "account", "lock", "bob").want(t, 0)
	e.run(t, "node", "account", "check", "fern", "bob", "--token-file", file).want(t, 1, "beide Dateien bleiben")
	if ok, _ := fileExists(file + ".pending"); !ok {
		t.Error(".pending ist weg")
	}
}

// Ein Fehler beim Schreiben der Replica nach Erfolg kippt den Erfolg nicht:
// Die Datei ist ersetzt, der Abgleich holt nach.
func TestAccountRotateReplicaFailure(t *testing.T) {
	e := newCommEnv(t)
	ns := nodeStore(t, e.cfg)
	path := ns.ReplicaPath("eigen")
	if err := os.MkdirAll(path, 0o700); err != nil { // ein Verzeichnis, wo die Datei hin soll
		t.Fatal(err)
	}
	file := e.tokenFile(t, "alice", e.tokens["alice"])
	e.run(t, "node", "account", "rotate", "eigen", "alice", "--token-file", file).want(t, 0,
		"Token ersetzt", "Replica nicht geschrieben", "node sync eigen")
	if readFileToken(t, file) == e.tokens["alice"] {
		t.Error("Datei nicht ersetzt")
	}
}

// rotate wird nie wiederholt: Ein Hub, der mit 500 antwortet, sieht genau
// einen Aufruf, und der Ausgang ist unklar.
func TestAccountRotateNoRetry(t *testing.T) {
	e := newCommEnv(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "kaputt", http.StatusInternalServerError)
	}))
	defer srv.Close()
	e.run(t, "node", "hub", "set", "fern", "--address", srv.URL).want(t, 0)
	file := e.tokenFile(t, "bob", e.tokens["bob"])
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 1, "Ausgang unklar")
	if n := calls.Load(); n != 1 {
		t.Errorf("%d Aufrufe, erwartet 1", n)
	}
	// Ein Abgleich dagegen wird wiederholt.
	calls.Store(0)
	hookHTTP(t, func(address string) (contract.Hub, error) {
		c, err := httpapi.NewClient(address)
		if err == nil {
			c.Backoff = 1
		}
		return c, err
	})
	e.run(t, "node", "sync", "fern").want(t, 1)
	if n := calls.Load(); n != 4 {
		t.Errorf("sync: %d Aufrufe, erwartet 4", n)
	}
}
