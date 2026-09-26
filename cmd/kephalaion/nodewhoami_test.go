package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
	"github.com/kephalaion/kephalaion/internal/config"
	"github.com/kephalaion/kephalaion/internal/node/mcpnode"
	"github.com/kephalaion/kephalaion/internal/node/replica"
	nodestore "github.com/kephalaion/kephalaion/internal/node/store"
	"github.com/kephalaion/kephalaion/internal/sqlitedb"
	"github.com/kephalaion/kephalaion/internal/upgrade"
)

// node whoami ohne Account: Hubs mit Node-Name und Stand, die Accounts aus
// den Replicas — nach rotate sofort, nach dem Abgleich alle; ein gesperrter
// fehlt nach dem nächsten Abgleich.
func TestNodeWhoamiList(t *testing.T) {
	e := newCommEnv(t)
	r := e.run(t, "node", "whoami")
	r.want(t, 0, "kephalaion "+buildinfo.Get().Version, "Hubs:\n  eigen (Node laptop): noch nie abgeglichen",
		"  fern (Node laptop-http): noch nie abgeglichen", "Accounts: keine bekannt")

	file := e.tokenFile(t, "alice", e.tokens["alice"])
	e.run(t, "node", "account", "rotate", "eigen", "alice", "--token-file", file).want(t, 0)
	r = e.run(t, "node", "whoami")
	r.want(t, 0, "HUB    ACCOUNT  USER   COLLECTIONS", "eigen  alice    alice  eigen:team-x (read)")
	if strings.Contains(r.out, "bob") || strings.Contains(r.out, "keph_") {
		t.Errorf("vor dem Abgleich:\n%s", r.out)
	}

	e.run(t, "node", "sync").want(t, 0)
	r = e.run(t, "node", "whoami")
	r.want(t, 0, "eigen (Node laptop): abgeglichen ", ", Revision 5", "eigen  bob", "kleist  eigen:team-x (read, write)",
		"fern   carol", "fern:team-x (read, supersede)")
	e.run(t, "node", "whoami", "--hub", "fern").want(t, 0, "fern  alice")
	if r := e.run(t, "node", "whoami", "--hub", "fern"); strings.Contains(r.out, "eigen") {
		t.Errorf("--hub fern:\n%s", r.out)
	}

	e.run(t, "hub", "account", "lock", "bob").want(t, 0)
	e.run(t, "node", "sync").want(t, 0)
	if r := e.run(t, "node", "whoami"); strings.Contains(r.out, "bob") || !strings.Contains(r.out, "carol") {
		t.Errorf("nach lock:\n%s", r.out)
	}

	e.run(t, "node", "whoami", "--json").want(t, 1, "--json gibt es nur mit <account>")
	e.run(t, "node", "whoami", "--hub", "fremd").want(t, 1, "Hub fremd gibt es nicht")
	e.run(t, "node", "whoami", "Bob").want(t, 1)
	e.run(t, "node", "whoami", "a", "b").want(t, 2, "Unerwartetes Argument: b")
	e.run(t, "node", "whoami", "--help").want(t, 0, "node whoami <account>", "node account check")
	e.run(t, "node", "--help").want(t, 0, "node whoami")
}

// node whoami <account>: dieselbe Antwort wie das Werkzeug für einen Client
// mit gültigen Zugangsdaten dieses Accounts; --json ist gleich der
// strukturierten MCP-Antwort.
func TestNodeWhoamiAccount(t *testing.T) {
	e := newCommEnv(t)
	file := e.tokenFile(t, "bob", e.tokens["bob"])
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 0)
	bob := readFileToken(t, file)
	e.run(t, "node", "sync").want(t, 0)
	e.run(t, "config", "set", "node", "sync_interval", "0").want(t, 0)

	r := e.run(t, "node", "whoami", "bob")
	r.want(t, 0, "eigen (Node laptop): angemeldet als bob (User kleist): eigen:team-x (read, write); abgeglichen ",
		"fern (Node laptop-http): angemeldet als bob (User kleist): fern:team-x (read, write)")
	if strings.Contains(r.out, bob) || strings.Contains(r.out, "keph_") {
		t.Errorf("Token in der Ausgabe:\n%s", r.out)
	}
	r = e.run(t, "node", "whoami", "bob", "--hub", "fern")
	if strings.Contains(r.out, "eigen") || !strings.Contains(r.out, "fern (Node laptop-http)") {
		t.Errorf("--hub fern:\n%s", r.out)
	}
	// Ein Account, den der Node nicht kennt: überall missing.
	e.run(t, "node", "whoami", "dave").want(t, 0, "eigen (Node laptop): keine Zugangsdaten",
		"fern (Node laptop-http): keine Zugangsdaten")

	cfg, _, err := config.Load(e.cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hub = nil
	cfg.Node.Listen = "127.0.0.1:0"
	srv := startServe(t, cfg)
	endpoint := "http://" + srv.addrs[config.Node] + mcpnode.Path
	viaMCP := mcpWhoamiChecked(t, endpoint, map[string][2]string{"eigen": {"bob", bob}, "fern": {"bob", bob}})

	r = e.run(t, "node", "whoami", "bob", "--json")
	r.want(t, 0)
	var viaCLI mcpnode.WhoamiOutput
	if err := json.Unmarshal([]byte(r.out), &viaCLI); err != nil {
		t.Fatalf("%v:\n%s", err, r.out)
	}
	if !reflect.DeepEqual(viaCLI, viaMCP) {
		t.Errorf("CLI\n%+v\nMCP\n%+v", viaCLI, viaMCP)
	}
	if viaCLI.Hubs[0].Login != mcpnode.LoginOK || viaCLI.Hubs[1].Login != mcpnode.LoginOK {
		t.Errorf("nicht angemeldet: %+v", viaCLI)
	}
	if viaCLI.Update.State != upgrade.StateOK || viaCLI.Update.Latest != "v0.2.0" {
		t.Errorf("update: %+v", viaCLI.Update)
	}
	for _, key := range []string{`"version"`, `"update"`, `"latest": "v0.2.0"`, `"hubs"`, `"unknown_hubs"`,
		`"login": "ok"`, `"last_success"`} {
		if !strings.Contains(r.out, key) {
			t.Errorf("JSON ohne %s:\n%s", key, r.out)
		}
	}
}

// Eine Replica, die sich nicht lesen lässt, betrifft in node whoami (Liste,
// Einzelansicht, --json) und über MCP nur ihren Hub: eigen gesund, fern mit
// Replica alter Schemafassung (zuletzt gelungen, danach ein Fehler), dritt
// mit Müll und nie gelungen. stdout nennt keinen Pfad; die volle Meldung
// steht auf stderr bzw. im Log von serve. status gibt weiter je Hub aus.
func TestNodeWhoamiUnreadableReplica(t *testing.T) {
	e := newCommEnv(t)
	file := e.tokenFile(t, "bob", e.tokens["bob"])
	e.run(t, "node", "account", "rotate", "eigen", "bob", "--token-file", file).want(t, 0)
	bob := readFileToken(t, file)
	e.run(t, "node", "sync").want(t, 0)
	e.run(t, "config", "set", "node", "sync_interval", "0").want(t, 0)
	r := e.run(t, "hub", "node", "add", "laptop-2")
	r.want(t, 0)
	e.runIn(t, tokenFrom(t, r.out), "node", "hub", "add", "dritt", "--node", "laptop-2", "--transport", "http",
		"--address", e.url, "--token-stdin").want(t, 0)

	ctx := context.Background()
	ns := nodeStore(t, e.cfg)
	db, err := sqlitedb.Open(ctx, ns.ReplicaPath("fern"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlitedb.SetInfo(ctx, db, sqlitedb.KeySchemaVersion, "2"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	for _, alias := range []string{"fern", "dritt"} {
		h, err := ns.Hub(ctx, alias)
		if err != nil {
			t.Fatal(err)
		}
		if err := ns.RecordSync(ctx, alias, h.EntryID, nodestore.SyncRecord{At: sqlitedb.NowMillis(),
			Err: "Hub " + e.url + " weg", ErrKind: string(replica.KindUnreachable)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(ns.ReplicaPath("dritt"), []byte(strings.Repeat("Müll ", 1000)), 0o600); err != nil {
		t.Fatal(err)
	}
	noPath := func(what, out string) {
		t.Helper()
		for _, not := range []string{e.dir, "replicas", "Schemafassung", "database", e.url, "nicht erreichbar"} {
			if strings.Contains(out, not) {
				t.Errorf("%s nennt %q:\n%s", what, not, out)
			}
		}
	}
	stderrNames := func(what, errOut string) {
		t.Helper()
		for _, alias := range []string{"fern", "dritt"} {
			if countLines(errOut, "node whoami: Hub "+alias+": ") != 1 || countLines(errOut, ns.ReplicaPath(alias)) != 1 {
				t.Errorf("%s: stderr ohne Meldung zu %s (einmal):\n%s", what, alias, errOut)
			}
		}
	}

	r = e.run(t, "node", "whoami")
	r.want(t, 0, "  eigen (Node laptop): abgeglichen ", "  fern (Node laptop-http): abgeglichen ",
		"; letzter Fehler: Replica nicht lesbar", "  dritt (Node laptop-2): noch nie abgeglichen; letzter Fehler: Replica nicht lesbar",
		"eigen  bob")
	if strings.Contains(r.out, "fern:team-x") || strings.Contains(r.out, "dritt:") {
		t.Errorf("Accounts aus kaputter Replica:\n%s", r.out)
	}
	noPath("node whoami", r.out)
	stderrNames("node whoami", r.errOut)
	r = e.run(t, "node", "whoami", "--hub", "fern")
	r.want(t, 0, "fern (Node laptop-http): abgeglichen ", "Accounts: keine bekannt")
	if strings.Contains(r.errOut, "dritt") {
		t.Errorf("--hub fern meldet dritt:\n%s", r.errOut)
	}

	r = e.run(t, "node", "whoami", "bob")
	r.want(t, 0, "eigen (Node laptop): angemeldet als bob (User kleist)",
		"fern (Node laptop-http): Anmeldung nicht prüfbar; abgeglichen ", "; letzter Fehler: Replica nicht lesbar",
		"dritt (Node laptop-2): Anmeldung nicht prüfbar; noch nie abgeglichen; letzter Fehler: Replica nicht lesbar")
	noPath("node whoami bob", r.out)
	stderrNames("node whoami bob", r.errOut)

	r = e.run(t, "node", "whoami", "bob", "--json")
	r.want(t, 0)
	noPath("node whoami bob --json", r.out)
	stderrNames("node whoami bob --json", r.errOut)
	var viaCLI mcpnode.WhoamiOutput
	if err := json.Unmarshal([]byte(r.out), &viaCLI); err != nil {
		t.Fatalf("%v:\n%s", err, r.out)
	}
	byHub := map[string]mcpnode.HubInfo{}
	for _, h := range viaCLI.Hubs {
		byHub[h.Hub] = h
	}
	if h := byHub["eigen"]; h.Login != mcpnode.LoginOK || h.Sync.Revision == nil {
		t.Errorf("eigen: %+v", h)
	}
	if h := byHub["fern"]; h.Login != mcpnode.LoginMissing || h.Account != "" || h.Sync.NeverSynced ||
		h.Sync.LastSuccess == "" || h.Sync.Revision != nil || h.Sync.LastError != mcpnode.ReplicaUnreadable ||
		h.Sync.LastErrorAt != "" {
		t.Errorf("fern: %+v", h)
	}
	if h := byHub["dritt"]; h.Login != mcpnode.LoginMissing || !h.Sync.NeverSynced || h.Sync.LastSuccess != "" ||
		h.Sync.LastError != mcpnode.ReplicaUnreadable {
		t.Errorf("dritt: %+v", h)
	}
	for _, not := range []string{"hub_id", "entry_id", "last_error_at", "keph_"} {
		if strings.Contains(r.out, not) {
			t.Errorf("JSON nennt %q:\n%s", not, r.out)
		}
	}

	// status: je Hub, mit der vollen Meldung, bricht nicht ab.
	r = e.run(t, "status")
	r.want(t, 0, "Replica:     Replica "+ns.ReplicaPath("fern"), ns.ReplicaPath("dritt")+" lässt sich nicht öffnen")
	if n := countLines(r.out, "(der nächste Abgleich legt sie neu an)"); n != 2 {
		t.Errorf("%d unlesbare Replicas in status:\n%s", n, r.out)
	}
	if n := countLines(r.out, "      hub_id:      "); n != 1 {
		t.Errorf("%d lesbare Replicas in status:\n%s", n, r.out)
	}

	// Über MCP dieselbe Antwort, die Meldungen im Log von serve.
	cfg, _, err := config.Load(e.cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hub = nil
	cfg.Node.Listen = "127.0.0.1:0"
	srv := startServe(t, cfg)
	viaMCP := mcpWhoamiChecked(t, "http://"+srv.addrs[config.Node]+mcpnode.Path,
		map[string][2]string{"eigen": {"bob", bob}, "fern": {"bob", bob}, "dritt": {"bob", bob}})
	if !reflect.DeepEqual(viaCLI, viaMCP) {
		t.Errorf("CLI\n%+v\nMCP\n%+v", viaCLI, viaMCP)
	}
	eventuallyLog(t, srv, "Meldungen im Log", func() bool {
		return contains(srv.log.String(), `error="Hub fern: Replica `+ns.ReplicaPath("fern"), `error="Hub dritt: `,
			ns.ReplicaPath("dritt"))
	})
}
