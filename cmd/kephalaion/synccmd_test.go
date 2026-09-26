package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kephalaion/kephalaion/internal/config"
	"github.com/kephalaion/kephalaion/internal/ident"
)

// syncSetup richtet Hub und Node in einer config ein: Collections wissen und
// team am Hub, Node laptop mit Recht auf wissen, am Node der Hub-Eintrag eigen
// über local, der beide will. Liefert die Option --config und die hub_id.
func syncSetup(t *testing.T, dir string) (c, hubID string) {
	t.Helper()
	cfg := setup(t, dir)
	c = "--config=" + cfg
	runT(t, "hub", "collection", "add", "wissen", c).want(t, 0)
	runT(t, "hub", "collection", "add", "team", c).want(t, 0)
	r := runT(t, "hub", "node", "add", "laptop", c)
	r.want(t, 0)
	tok := tokenFrom(t, r.out)
	runT(t, "hub", "node", "grant", "laptop", "wissen", c).want(t, 0)
	runIn(t, tok+"\n", "node", "hub", "add", "eigen", "--node", "laptop", "--transport", "local", "--token-stdin", c).
		want(t, 0)
	runT(t, "node", "collection", "add", "eigen:wissen", c).want(t, 0)
	runT(t, "node", "collection", "add", "eigen:team", c).want(t, 0)
	info, err := hubStore(t, cfg).Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return c, info.HubID
}

func TestNodeSyncFlow(t *testing.T) {
	dir := isolate(t)
	c, hubID := syncSetup(t, dir)
	src := filepath.Join(dir, "quelle")
	writeFile(t, filepath.Join(src, "a.md"), "A\n")
	writeFile(t, filepath.Join(src, "sub", "b.md"), "---\ntitel: b\n---\nB\n")
	runT(t, "hub", "import", "wissen", src, c).want(t, 0, "Revision 1")

	// Vor dem ersten Abgleich: keine Replica.
	runT(t, "status", c).want(t, 0, "eigen: local, als Node laptop\n      hub_id:      noch kein Abgleich",
		"Collections: team, wissen")
	runT(t, "node", "doc", "list", "eigen:wissen", c).
		want(t, 1, "Hub eigen: noch kein Abgleich; zuerst: kephalaion node sync eigen")
	runT(t, "node", "doc", "get", "eigen:wissen", "a.md", c).want(t, 1, "noch kein Abgleich")
	runT(t, "node", "doc", "list", "fehlt:wissen", c).want(t, 1, "Hub fehlt gibt es nicht")

	runT(t, "node", "sync", c).want(t, 0, "Hub eigen (hub_id "+hubID+"): 1 Seite",
		"wissen: abgeglichen, 2 Zeilen, Revision 1", "team: nicht erlaubt\n")

	r := runT(t, "node", "doc", "list", "eigen:wissen", c)
	r.want(t, 0, "NAME", "a.md", "sub/", "von admin")
	if strings.Contains(r.out, "b.md") {
		t.Errorf("list zeigt Unterverzeichnis-Inhalt:\n%s", r.out)
	}
	runT(t, "node", "doc", "list", "eigen:wissen", "sub", c).want(t, 0, "b.md")
	runT(t, "node", "doc", "list", "eigen:wissen", "leer", c).want(t, 0, "Keine Dokumente unter leer/ in eigen:wissen.")
	r = runT(t, "node", "doc", "get", "eigen:wissen", "sub/b.md", c)
	r.want(t, 0)
	if r.out != "---\ntitel: b\n---\nB\n" {
		t.Errorf("get = %q", r.out)
	}
	runT(t, "node", "doc", "get", "eigen:team", "a.md", c).want(t, 1, "Collection team gibt es in der Replica nicht")
	runT(t, "node", "doc", "get", "eigen:wissen", "fehlt.md", c).want(t, 1, "gibt es in der Replica nicht")
	runT(t, "node", "doc", "get", "eigen:wissen", "../a", c).want(t, 1, "'..'")
	runT(t, "node", "doc", "get", "ohne-doppelpunkt", "a.md", c).want(t, 1, "<hub>:<collection>")
	runT(t, "node", "doc", "get", "eigen:wissen", c).want(t, 2, "Es fehlt: <name>")

	runT(t, "status", c).want(t, 0, "      hub_id:      "+hubID, "        wissen: Revision 1, abgeglichen ",
		"        team: noch nicht abgeglichen")

	// Löschen am Hub: Die Löschmarke kommt an, das Dokument verschwindet.
	runT(t, "hub", "doc", "rm", "wissen", "a.md", c).want(t, 0, "Revision 2")
	runT(t, "node", "sync", "eigen", c).want(t, 0, "wissen: abgeglichen, 1 Zeile, Revision 2")
	runT(t, "node", "doc", "get", "eigen:wissen", "a.md", c).want(t, 1, "gibt es in der Replica nicht")
	r = runT(t, "node", "doc", "list", "eigen:wissen", c)
	r.want(t, 0, "sub/")
	if strings.Contains(r.out, "a.md") {
		t.Errorf("list nach rm:\n%s", r.out)
	}
	runT(t, "node", "sync", c).want(t, 0, "wissen: abgeglichen, 0 Zeilen, Revision 2")
	runT(t, "status", c).want(t, 0, "wissen: Revision 2")

	// Am Hub erlaubt, am Node nicht mehr gewünscht: aus der Replica entfernt.
	runT(t, "node", "collection", "rm", "eigen:wissen", c).want(t, 0)
	runT(t, "status", c).want(t, 0, "wissen: nicht mehr gewünscht, Revision 2")
	runT(t, "node", "sync", c).want(t, 0, "wissen: nicht mehr gewünscht, 2 Zeilen aus der Replica entfernt",
		"vom Hub außerdem erlaubt: wissen")
	runT(t, "node", "doc", "list", "eigen:wissen", c).want(t, 1, "Collection wissen gibt es in der Replica nicht")

	runT(t, "node", "sync", "fehlt", c).want(t, 1, "Hub fehlt gibt es nicht")
	runT(t, "node", "sync", "a", "b", c).want(t, 2, "Unerwartetes Argument: b")

	// node hub rm nimmt die Replica mit.
	runT(t, "node", "hub", "rm", "eigen", c).want(t, 0)
	runT(t, "node", "sync", c).want(t, 0, "Keine Hubs.")
}

// Scheitert ein Eintrag, laufen die übrigen weiter; der Exit-Code ist 1.
func TestNodeSyncPerEntryErrors(t *testing.T) {
	dir := isolate(t)
	c, hubID := syncSetup(t, dir)
	runIn(t, "x", "hub", "doc", "put", "wissen", "a.md", c).want(t, 0)
	tok, err := ident.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	runIn(t, tok, "node", "hub", "add", "fern", "--node", "laptop", "--transport", "https",
		"--address", "https://hub.example.org", "--token-stdin", c).want(t, 0)
	runT(t, "node", "collection", "add", "fern:wissen", c).want(t, 0)

	r := runT(t, "node", "sync", c)
	r.want(t, 1, "Hub eigen (hub_id "+hubID+"): 1 Seite", "wissen: abgeglichen, 1 Zeile, Revision 1",
		"Hub fern: gescheitert")
	if !strings.Contains(r.errOut, "node sync: Hub fern: Transport https wird noch nicht unterstützt") ||
		strings.Contains(r.errOut, "eigen") {
		t.Errorf("stderr:\n%s", r.errOut)
	}
	runT(t, "node", "doc", "get", "eigen:wissen", "a.md", c).want(t, 0, "x")
	runT(t, "node", "sync", "eigen", c).want(t, 0)

	// Gesperrt: Fehler dieses Eintrags, die Replica bleibt lesbar.
	runT(t, "hub", "node", "lock", "laptop", c).want(t, 0)
	runT(t, "node", "sync", "eigen", c).want(t, 1, "Hub eigen (hub_id "+hubID+"): gescheitert",
		"team: nicht abgeglichen", "node sync: Hub eigen: nicht angemeldet")
	runT(t, "node", "doc", "get", "eigen:wissen", "a.md", c).want(t, 0, "x")
	runT(t, "hub", "node", "unlock", "laptop", c).want(t, 0)

	// local ohne Abschnitt hub: in der config — Fehler für diesen Eintrag,
	// der andere kommt trotzdem dran.
	cfgPath := strings.TrimPrefix(c, "--config=")
	cfg, _, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetSection(config.Hub, nil)
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	r = runT(t, "node", "sync", c)
	r.want(t, 1, "node sync: Hub eigen: Transport local verlangt einen Hub in derselben config",
		"node sync: Hub fern: Transport https wird noch nicht unterstützt")
	// Die Replica bleibt; status zeigt ihren Stand.
	runT(t, "status", c).want(t, 0, "hub_id:      "+hubID, "wissen: Revision 1", "fern: https https://hub.example.org")
}

func TestNodeSyncUsage(t *testing.T) {
	isolate(t)
	runT(t, "node", "--help").want(t, 0, "kephalaion node sync", "node doc list|get")
	runT(t, "node", "sync", "--help").want(t, 0, "noch nicht unterstützt", "Exit-Code ist dann 1")
	runT(t, "node", "doc", "--help").want(t, 0, "node doc list", "SYSTEM:")
	runT(t, "node", "doc").want(t, 2, "kephalaion node doc get")
	runT(t, "node", "doc", "rm").want(t, 2, "Unbekanntes Kommando: node doc rm")
	runT(t, "help").want(t, 0, "sync, doc")
	runT(t, "node", "sync").want(t, 1, "der Node ist nicht eingerichtet")
}
