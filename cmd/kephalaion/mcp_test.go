package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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

// Über serve: whoami per MCP gilt nach rotate; nach lock am Hub und sync am
// Node gilt es nicht mehr.
func TestMCPWhoamiLockAndSync(t *testing.T) {
	e := newCommEnv(t)
	file := e.tokenFile(t, "carol", e.tokens["carol"])
	e.run(t, "node", "account", "rotate", "eigen", "carol", "--token-file", file).want(t, 0)
	carol := readFileToken(t, file)

	cfg, _, err := config.Load(e.cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hub = nil // der Hub läuft schon als httptest-Server
	cfg.Node.Listen = "127.0.0.1:0"
	srv := startServe(t, cfg)
	endpoint := "http://" + srv.addrs[config.Node] + mcpnode.Path

	out := mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"carol", carol}})
	if len(out.Hubs) != 1 || !out.Hubs[0].Authenticated || out.Hubs[0].Account != "carol" ||
		len(out.Hubs[0].Collections) != 1 || out.Hubs[0].Collections[0].Address != "eigen:team-x" {
		t.Fatalf("vor lock: %+v", out)
	}
	e.run(t, "hub", "account", "lock", "carol").want(t, 0)
	// Ohne Abgleich weiß der Node noch nichts davon.
	if out := mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"carol", carol}}); !out.Hubs[0].Authenticated {
		t.Error("gesperrt schon vor dem Abgleich")
	}
	e.run(t, "node", "sync", "eigen").want(t, 0)
	out = mcpWhoami(t, endpoint, map[string][2]string{"eigen": {"carol", carol}})
	if len(out.Hubs) != 1 || out.Hubs[0].Authenticated || out.Hubs[0].Account != "" {
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
