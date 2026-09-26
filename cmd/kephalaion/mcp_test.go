package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
	"github.com/kephalaion/kephalaion/internal/config"
	"github.com/kephalaion/kephalaion/internal/node/mcpnode"
)

// headerRT setzt die Header, die ein Client aus seiner MCP-Konfiguration
// schickt.
type headerRT struct{ h http.Header }

func (rt headerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range rt.h {
		r.Header[k] = v
	}
	return http.DefaultTransport.RoundTrip(r)
}

// mcpWhoami ruft whoami am MCP-Eingang unter endpoint, mit einem Header-Paar
// je Eintrag in pairs (Alias → Account, Token).
func mcpWhoami(t *testing.T, endpoint string, pairs map[string][2]string) mcpnode.WhoamiOutput {
	t.Helper()
	h := http.Header{}
	for alias, p := range pairs {
		h.Set("X-Keph-Account-"+alias, p[0])
		h.Set("X-Keph-Token-"+alias, p[1])
	}
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint,
		HTTPClient: &http.Client{Transport: headerRT{h}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "whoami"})
	if err != nil || res.IsError {
		t.Fatalf("whoami: %+v, %v", res, err)
	}
	var out mcpnode.WhoamiOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Über serve: whoami per MCP gilt nach rotate und nennt den User, auch nach
// set --user und sync; nach lock am Hub und sync am Node gilt es nicht mehr.
func TestMCPWhoamiLockAndSync(t *testing.T) {
	e := newCommEnv(t)
	file := e.tokenFile(t, "carol", e.tokens["carol"])
	e.run(t, "node", "account", "rotate", "eigen", "carol", "--token-file", file).want(t, 0)
	carol := readFileToken(t, file)
	// Ohne Abgleich im Hintergrund: Der Test steuert ihn mit node sync.
	e.run(t, "config", "set", "node", "sync_interval", "0").want(t, 0)

	cfg, _, err := config.Load(e.cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hub = nil // der Hub läuft schon als httptest-Server
	cfg.Node.Listen = "127.0.0.1:0"
	srv := startServe(t, cfg)
	endpoint := "http://" + srv.addrs[config.Node] + mcpnode.Path

	out := mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"carol", carol}})
	if len(out.Hubs) != 2 || out.Hubs[0].Hub != "eigen" || out.Hubs[0].Login != mcpnode.LoginOK ||
		out.Hubs[0].Account != "carol" || len(out.Hubs[0].Collections) != 1 ||
		out.Hubs[0].Collections[0].Address != "eigen:team-x" || out.Hubs[1].Login != mcpnode.LoginMissing {
		t.Fatalf("vor lock: %+v", out)
	}
	if out.Hubs[0].User != "carol" || out.Version != buildinfo.Get().Version || out.Hubs[0].Node != "laptop" {
		t.Errorf("vor set: %+v", out)
	}
	// set --user am Hub kommt mit dem Abgleich an.
	e.run(t, "hub", "account", "set", "carol", "--user", "kleist").want(t, 0)
	e.run(t, "node", "sync", "eigen").want(t, 0)
	if out := mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"carol", carol}}); out.Hubs[0].User != "kleist" {
		t.Errorf("User nach set und sync: %+v", out.Hubs[0])
	}
	e.run(t, "hub", "account", "lock", "carol").want(t, 0)
	// Ohne Abgleich weiß der Node noch nichts davon.
	if out := mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"carol", carol}}); out.Hubs[0].Login != mcpnode.LoginOK {
		t.Error("gesperrt schon vor dem Abgleich")
	}
	e.run(t, "node", "sync", "eigen").want(t, 0)
	out = mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"carol", carol}})
	if out.Hubs[0].Login != mcpnode.LoginInvalid || out.Hubs[0].Account != "" || out.Hubs[0].User != "" {
		t.Errorf("nach lock und sync: %+v", out)
	}
	srv.stop(t)
	if log := srv.log.String(); !contains(log, "node POST /mcp 200", "account=carol") {
		t.Errorf("Log:\n%s", log)
	}
}

func contains(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}

// whoami über serve mit dem Abgleich im Hintergrund: alle Hubs, der Stand
// aus hub_sync und Replica; ein Header-Paar für einen Hub ohne Replica ist
// invalid mit „noch nie abgeglichen“; ein unbekannter Alias wird gemeldet.
func TestMCPWhoamiWithBackgroundSync(t *testing.T) {
	e := newCommEnv(t)
	file := e.tokenFile(t, "bob", e.tokens["bob"])
	e.run(t, "node", "account", "rotate", "fern", "bob", "--token-file", file).want(t, 0)
	bob := readFileToken(t, file)
	// Ein Eintrag über https: nie abgeglichen.
	e.runIn(t, bob, "node", "hub", "add", "extern", "--node", "laptop", "--transport", "https",
		"--address", "https://hub.example.org", "--token-stdin").want(t, 0)
	e.run(t, "config", "set", "node", "sync_interval", "1s").want(t, 0)
	srv := startServe(t, portZero(t, e.cfg))
	ns := nodeStore(t, e.cfg)
	eventually(t, "Abgleich von eigen und fern", func() bool {
		return syncStatus(t, ns, "eigen").OKAt != 0 && syncStatus(t, ns, "fern").OKAt != 0
	})
	endpoint := "http://" + srv.addrs[config.Node] + mcpnode.Path
	out := mcpWhoami(t, endpoint, map[string][2]string{"fern": {"bob", bob}, "extern": {"bob", bob}, "tippfehler": {"bob", bob}})
	if len(out.Hubs) != 3 || fmt.Sprint(out.UnknownHubs) != "[tippfehler]" {
		t.Fatalf("whoami: %+v", out)
	}
	byHub := map[string]mcpnode.HubInfo{}
	for _, h := range out.Hubs {
		byHub[h.Hub] = h
	}
	if h := byHub["fern"]; h.Login != mcpnode.LoginOK || h.User != "kleist" || h.Node != "laptop-http" ||
		h.Sync.LastSuccess == "" || h.Sync.Revision == nil || *h.Sync.Revision == 0 || h.Sync.LastError != "" {
		t.Errorf("fern: %+v", h)
	}
	if h := byHub["eigen"]; h.Login != mcpnode.LoginMissing || h.Sync.LastSuccess == "" {
		t.Errorf("eigen: %+v", h)
	}
	if h := byHub["extern"]; h.Login != mcpnode.LoginInvalid || !h.Sync.NeverSynced || h.Sync.Revision != nil {
		t.Errorf("extern: %+v", h)
	}
}
