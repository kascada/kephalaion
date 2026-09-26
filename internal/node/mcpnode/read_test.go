package mcpnode

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (e *docEnv) read(t *testing.T, h map[string][]string, in ReadInput) (ReadOutput, *mcp.CallToolResult, string) {
	t.Helper()
	var out ReadOutput
	res, errText := e.call(t, h, "read", in, &out)
	if errText == "" {
		raw, _ := json.Marshal(res)
		if strings.Contains(string(raw), "SYSTEM") {
			t.Errorf("SYSTEM: in der Antwort: %s", raw)
		}
	}
	return out, res, errText
}

func no() *bool { b := false; return &b }

func TestReadDocument(t *testing.T) {
	e := newDocEnv(t)
	ids := e.fillWissen(t)
	out, res, errText := e.read(t, e.anna(), ReadInput{Collection: "keph:wissen", Name: "dir/c.md"})
	if errText != "" || out.Kind != KindDocument || out.Address != "keph:wissen" || out.Name != "dir/c.md" ||
		out.ID != ids["dir/c.md"] || out.Revision == 0 || out.Size == nil || *out.Size != int64(len("Inhalt von dir/c.md")) ||
		out.Created == nil || out.Updated == nil || out.Writable == nil || !*out.Writable {
		t.Fatalf("per Name: %+v, %s", out, errText)
	}
	if got := textOf(res); got != "Inhalt von dir/c.md" {
		t.Errorf("Inhalt: %q", got)
	}
	// Per id, ohne Hub-Teil (otto ist nur an keph angemeldet), ohne Recht
	// write.
	out, res, _ = e.read(t, e.otto(), ReadInput{ID: ids["a.md"]})
	if out.Kind != KindDocument || out.Name != "a.md" || out.Address != "keph:wissen" || out.Writable == nil ||
		*out.Writable || textOf(res) != "neu" {
		t.Errorf("per id: %+v, %q", out, textOf(res))
	}
	// Mit mehreren Hubs braucht id den Hub.
	if _, _, errText := e.read(t, e.anna(), ReadInput{ID: ids["a.md"]}); !strings.Contains(errText, "angemeldet an keph, team") {
		t.Errorf("id ohne Hub: %s", errText)
	}
	out, _, _ = e.read(t, e.anna(), ReadInput{Collection: "keph:", ID: ids["a.md"]})
	if out.Kind != KindDocument || out.Name != "a.md" {
		t.Errorf("id mit Hub: %+v", out)
	}
	// content: false — nur die Angaben, als JSON im Text.
	out, res, _ = e.read(t, e.anna(), ReadInput{Collection: "keph:wissen", Name: "a.md", Content: no()})
	if out.Kind != KindDocument || out.Size == nil || *out.Size != 3 || strings.Contains(textOf(res), `"neu"`) ||
		!strings.Contains(textOf(res), `"kind":"document"`) {
		t.Errorf("ohne Inhalt: %+v, %q", out, textOf(res))
	}
	// Leerer Inhalt ist ein Dokument mit Größe 0.
	e.hubs["keph"].put("wissen", "leer.md", "")
	e.sync(t)
	out, res, _ = e.read(t, e.anna(), ReadInput{Collection: "keph:wissen", Name: "leer.md"})
	if out.Kind != KindDocument || *out.Size != 0 || textOf(res) != "" {
		t.Errorf("leer: %+v, %q", out, textOf(res))
	}
}

func TestReadDirectoryAndNone(t *testing.T) {
	e := newDocEnv(t)
	ids := e.fillWissen(t)
	cases := []struct {
		in   ReadInput
		kind string
		name string
	}{
		{ReadInput{Collection: "keph:wissen", Name: "dir"}, KindDirectory, "dir"},
		{ReadInput{Collection: "keph:wissen", Name: "dir/sub/"}, KindDirectory, "dir/sub"},
		{ReadInput{Collection: "keph:wissen"}, KindDirectory, ""},
		{ReadInput{Collection: "wissen", Name: ""}, KindDirectory, ""},
		{ReadInput{Collection: "keph:"}, KindDirectory, ""},
		{ReadInput{Collection: "keph:wissen", Name: "fehlt.md"}, KindNone, "fehlt.md"},
		{ReadInput{Collection: "keph:wissen", Name: "di"}, KindNone, "di"},
		{ReadInput{Collection: "keph:wissen", Name: "a.md/x"}, KindNone, "a.md/x"},
		// Eine Löschmarke ist nichts, per Name wie per id.
		{ReadInput{Collection: "keph:wissen", Name: "gone.md"}, KindNone, "gone.md"},
		{ReadInput{ID: ids["gone.md"]}, KindNone, ""},
		{ReadInput{ID: "01ZZZZZZZZZZZZZZZZZZZZZZZZ"}, KindNone, ""},
		// Die id eines Dokuments aus einer anderen Collection ist dort nichts.
		{ReadInput{Collection: "keph:privat", ID: ids["a.md"]}, KindNone, ""},
	}
	for _, c := range cases {
		out, _, errText := e.read(t, pairOf(e, "keph", "anna"), c.in)
		if errText != "" || out.Kind != c.kind || out.Name != c.name {
			t.Errorf("%+v: %+v, %s", c.in, out, errText)
		}
	}
	out, _, _ := e.read(t, e.anna(), ReadInput{Collection: "keph:wissen", Name: "dir"})
	if out.Writable == nil || !*out.Writable || out.ID != "" || out.Size != nil {
		t.Errorf("Verzeichnis: %+v", out)
	}
	out, _, _ = e.read(t, e.anna(), ReadInput{Collection: "keph:"})
	if out.Address != "keph:" || out.Writable != nil {
		t.Errorf("Wurzel des Hubs: %+v", out)
	}
	// otto darf privat nicht lesen: per id nichts, per Name nicht lesbar —
	// wie eine Collection, die es nicht gibt.
	geheim := e.hubs["keph"].put("privat", "zwei.md", "x")
	e.sync(t)
	if out, _, _ := e.read(t, e.otto(), ReadInput{ID: geheim}); out.Kind != KindNone {
		t.Errorf("fremde id: %+v", out)
	}
	for _, in := range []ReadInput{
		{Collection: "keph:privat", Name: "zwei.md"}, {Collection: "keph:privat"}, {Collection: "keph:gibts-nicht"},
	} {
		if _, _, errText := e.read(t, e.otto(), in); !strings.Contains(errText, "nicht lesbar") {
			t.Errorf("otto %+v: %s", in, errText)
		}
	}
	for _, in := range []ReadInput{
		{Collection: "keph:wissen", Name: "SYSTEM:A:anna"}, {Collection: "keph:wissen", Name: "/a.md"},
		{Collection: "keph:wissen", Name: "a.md", ID: ids["a.md"]}, {Name: "a.md"}, {Collection: "keph:", Name: "a.md"},
	} {
		if _, _, errText := e.read(t, e.anna(), in); errText == "" {
			t.Errorf("%+v ohne Fehler", in)
		}
	}
}

// pairOf sind die Header eines Accounts an einem Hub.
func pairOf(e *docEnv, hub, account string) map[string][]string {
	return pair(hub, account, e.tokens[hub+"/"+account])
}
