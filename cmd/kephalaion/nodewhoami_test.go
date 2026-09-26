package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
	"github.com/kephalaion/kephalaion/internal/config"
	"github.com/kephalaion/kephalaion/internal/node/mcpnode"
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
	viaMCP := mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"bob", bob}, "fern": {"bob", bob}})

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
	for _, key := range []string{`"version"`, `"hubs"`, `"unknown_hubs"`, `"login": "ok"`, `"last_success"`} {
		if !strings.Contains(r.out, key) {
			t.Errorf("JSON ohne %s:\n%s", key, r.out)
		}
	}
}
