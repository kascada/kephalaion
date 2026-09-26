package mcpnode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fillWissen legt in keph:wissen Dokumente an, gleicht ab und liefert die
// ids nach Name. a.md wird zuletzt geändert: angelegt und geändert ordnen
// verschieden.
func (e *docEnv) fillWissen(t *testing.T) map[string]string {
	t.Helper()
	f := e.hubs["keph"]
	ids := map[string]string{}
	for _, name := range []string{"a.md", "b.txt", "0001-x.md", "0002-y.md", "dir/c.md", "dir/sub/d.md", "zdir/e.md",
		"gone.md"} {
		ids[name] = f.put("wissen", name, "Inhalt von "+name)
	}
	f.put("privat", "geheim.md", "nur anna")
	f.rm(ids["gone.md"])
	f.put("wissen", "a.md", "neu")
	e.sync(t)
	return ids
}

func names(entries []ListEntry) []string {
	out := []string{}
	for _, en := range entries {
		n := en.Name
		switch en.Kind {
		case KindDirectory:
			n += "/"
		case KindCollection:
			n = en.Address
		}
		out = append(out, n)
	}
	return out
}

func (e *docEnv) list(t *testing.T, h map[string][]string, in ListInput) (ListOutput, string) {
	t.Helper()
	var out ListOutput
	res, errText := e.call(t, h, "list", in, &out)
	if errText == "" {
		raw, _ := json.Marshal(res)
		if strings.Contains(string(raw), "SYSTEM") {
			t.Errorf("SYSTEM: in der Antwort: %s", raw)
		}
	}
	return out, errText
}

// listAll blättert mit limit bis zum Ende und liefert alle Namen.
func (e *docEnv) listAll(t *testing.T, in ListInput, limit int) []string {
	t.Helper()
	in.Limit = limit
	var all []string
	for i := 0; ; i++ {
		out, errText := e.list(t, e.anna(), in)
		if errText != "" {
			t.Fatal(errText)
		}
		if len(out.Entries) > limit || (out.More && len(out.Entries) != limit) || (out.More != (out.Cursor != "")) {
			t.Fatalf("Seite %d: %+v", i, out)
		}
		all = append(all, names(out.Entries)...)
		if !out.More {
			return all
		}
		in.Cursor = out.Cursor
	}
}

func TestListCollections(t *testing.T) {
	e := newDocEnv(t)
	out, errText := e.list(t, e.anna(), ListInput{})
	if want := []string{"keph:privat", "keph:wissen", "team:notizen"}; errText != "" || !reflect.DeepEqual(names(out.Entries), want) || out.More {
		t.Errorf("Wurzel: %v, %s", names(out.Entries), errText)
	}
	if en := out.Entries[0]; en.Kind != KindCollection || en.Name != "privat" || en.Address != "keph:privat" || en.ID != "" {
		t.Errorf("Eintrag: %+v", en)
	}
	out, _ = e.list(t, e.anna(), ListInput{Collection: "team:"})
	if want := []string{"team:notizen"}; !reflect.DeepEqual(names(out.Entries), want) {
		t.Errorf("team: %v", names(out.Entries))
	}
	out, _ = e.list(t, e.otto(), ListInput{})
	if want := []string{"keph:wissen"}; !reflect.DeepEqual(names(out.Entries), want) {
		t.Errorf("otto: %v", names(out.Entries))
	}
	if got := e.listAll(t, ListInput{}, 1); !reflect.DeepEqual(got, []string{"keph:privat", "keph:wissen", "team:notizen"}) {
		t.Errorf("geblättert: %v", got)
	}
	// Ohne Anmeldung: keine Collections, kein Fehler; team: ist nicht lesbar.
	out, errText = e.list(t, nil, ListInput{})
	if errText != "" || len(out.Entries) != 0 {
		t.Errorf("ohne Anmeldung: %+v, %s", out, errText)
	}
	if _, errText := e.list(t, e.otto(), ListInput{Collection: "team:"}); !strings.Contains(errText, "team: nicht lesbar") {
		t.Errorf("fremder Hub: %s", errText)
	}
}

func TestListDirectories(t *testing.T) {
	e := newDocEnv(t)
	ids := e.fillWissen(t)
	out, errText := e.list(t, e.otto(), ListInput{Collection: "wissen"})
	want := []string{"dir/", "zdir/", "0001-x.md", "0002-y.md", "a.md", "b.txt"}
	if errText != "" || !reflect.DeepEqual(names(out.Entries), want) {
		t.Fatalf("Wurzel: %v, %s", names(out.Entries), errText)
	}
	if d := out.Entries[0]; d.Kind != KindDirectory || d.ID != "" || d.Size != nil || d.Created != nil {
		t.Errorf("Verzeichnis: %+v", d)
	}
	a := out.Entries[4]
	if a.Kind != KindDocument || a.ID != ids["a.md"] || a.Revision == 0 || a.Size == nil || *a.Size != 3 ||
		a.Created == nil || a.Created.By != "kleist" || a.Updated == nil || a.Updated.At <= a.Created.At ||
		!strings.HasSuffix(a.Updated.At, "Z") {
		t.Errorf("Dokument: %+v", a)
	}
	out, _ = e.list(t, e.otto(), ListInput{Collection: "keph:wissen", Path: "dir"})
	if want := []string{"dir/sub/", "dir/c.md"}; !reflect.DeepEqual(names(out.Entries), want) {
		t.Errorf("dir: %v", names(out.Entries))
	}
	out, _ = e.list(t, e.otto(), ListInput{Collection: "keph:wissen", Path: "dir/sub/"})
	if want := []string{"dir/sub/d.md"}; !reflect.DeepEqual(names(out.Entries), want) {
		t.Errorf("dir/sub/: %v", names(out.Entries))
	}
	out, _ = e.list(t, e.otto(), ListInput{Collection: "keph:wissen", Path: "gibt-es-nicht"})
	if len(out.Entries) != 0 {
		t.Errorf("leeres Verzeichnis: %v", names(out.Entries))
	}
	out, _ = e.list(t, e.otto(), ListInput{Collection: "keph:wissen", Recursive: true})
	want = []string{"0001-x.md", "0002-y.md", "a.md", "b.txt", "dir/c.md", "dir/sub/d.md", "zdir/e.md"}
	if !reflect.DeepEqual(names(out.Entries), want) {
		t.Errorf("rekursiv: %v", names(out.Entries))
	}
	for _, in := range []ListInput{{Collection: "keph:wissen", Path: "/x"}, {Path: "dir"}, {Collection: "keph:", Path: "dir"}} {
		if _, errText := e.list(t, e.otto(), in); errText == "" {
			t.Errorf("%+v ohne Fehler", in)
		}
	}
	// Ohne Recht dieselbe Meldung wie für eine unbekannte Collection.
	_, e1 := e.list(t, e.otto(), ListInput{Collection: "keph:privat"})
	_, e2 := e.list(t, e.otto(), ListInput{Collection: "keph:privat2"})
	if !strings.HasPrefix(e1, "keph:privat nicht lesbar") || strings.TrimPrefix(e1, "keph:privat") != strings.TrimPrefix(e2, "keph:privat2") {
		t.Errorf("nicht lesbar: %q, %q", e1, e2)
	}
}

func TestListSortAndPaging(t *testing.T) {
	e := newDocEnv(t)
	e.fillWissen(t)
	flat := ListInput{Collection: "keph:wissen", Recursive: true}
	byName := []string{"0001-x.md", "0002-y.md", "a.md", "b.txt", "dir/c.md", "dir/sub/d.md", "zdir/e.md"}
	byCreated := []string{"a.md", "b.txt", "0001-x.md", "0002-y.md", "dir/c.md", "dir/sub/d.md", "zdir/e.md"}
	byUpdated := []string{"b.txt", "0001-x.md", "0002-y.md", "dir/c.md", "dir/sub/d.md", "zdir/e.md", "a.md"}
	rev := func(s []string) []string {
		out := append([]string(nil), s...)
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		return out
	}
	cases := []struct {
		sort, order string
		want        []string
	}{
		{"", "", byName}, {"name", "desc", rev(byName)}, {"created", "asc", byCreated}, {"created", "desc", rev(byCreated)},
		{"updated", "", byUpdated}, {"updated", "desc", rev(byUpdated)},
	}
	for _, c := range cases {
		in := flat
		in.Sort, in.Order = c.sort, c.order
		for _, limit := range []int{1, 2, 3, 100} {
			if got := e.listAll(t, in, limit); !reflect.DeepEqual(got, c.want) {
				t.Errorf("%s %s limit %d: %v", c.sort, c.order, limit, got)
			}
		}
	}
	// Verzeichnisse zuerst, auch über Seiten hinweg; sort gilt nur für
	// Dokumente.
	in := ListInput{Collection: "keph:wissen", Sort: "updated", Order: "desc"}
	want := []string{"dir/", "zdir/", "a.md", "0002-y.md", "0001-x.md", "b.txt"}
	for _, limit := range []int{1, 2, 3} {
		if got := e.listAll(t, in, limit); !reflect.DeepEqual(got, want) {
			t.Errorf("mit Verzeichnissen, limit %d: %v", limit, got)
		}
	}
	// Ohne limit höchstens DefaultLimit, mehr wird gekürzt.
	for _, limit := range []int{0, MaxLimit + 1} {
		out, errText := e.list(t, e.anna(), ListInput{Collection: "keph:wissen", Limit: limit})
		if errText != "" || len(out.Entries) != 6 || out.More {
			t.Errorf("limit %d: %+v, %s", limit, out, errText)
		}
	}
	// Ein Cursor gilt nur für dieselbe Anfrage.
	out, _ := e.list(t, e.anna(), ListInput{Collection: "keph:wissen", Limit: 1})
	for _, bad := range []ListInput{
		{Collection: "keph:wissen", Limit: 1, Cursor: out.Cursor, Sort: "updated"},
		{Collection: "keph:wissen", Limit: 1, Cursor: out.Cursor, Recursive: true},
		{Collection: "keph:wissen", Limit: 1, Cursor: "kaputt!"},
		{Collection: "keph:wissen", Sort: "size"},
		{Collection: "keph:wissen", Order: "up"},
		{Collection: "keph:wissen", Limit: -1},
	} {
		if _, errText := e.list(t, e.anna(), bad); errText == "" {
			t.Errorf("%+v ohne Fehler", bad)
		}
	}
}

func TestListMask(t *testing.T) {
	e := newDocEnv(t)
	e.fillWissen(t)
	cases := []struct {
		in   ListInput
		want []string
	}{
		{ListInput{Mask: "*.md"}, []string{"dir/", "zdir/", "0001-x.md", "0002-y.md", "a.md"}},
		{ListInput{Mask: "0*-*.md"}, []string{"dir/", "zdir/", "0001-x.md", "0002-y.md"}},
		{ListInput{Mask: "*.md", Recursive: true}, []string{"0001-x.md", "0002-y.md", "a.md", "dir/c.md", "dir/sub/d.md", "zdir/e.md"}},
		// * geht nicht über '/': das letzte Segment von dir/c.md ist c.md.
		{ListInput{Mask: "dir*", Recursive: true}, []string{}},
		{ListInput{Mask: "c.md", Recursive: true}, []string{"dir/c.md"}},
		{ListInput{Mask: "?.*", Recursive: true, Sort: "name", Order: "desc"}, []string{"zdir/e.md", "dir/sub/d.md", "dir/c.md", "b.txt", "a.md"}},
	}
	for _, c := range cases {
		c.in.Collection = "keph:wissen"
		for _, limit := range []int{1, 100} {
			if got := e.listAll(t, c.in, limit); !reflect.DeepEqual(got, c.want) && !(len(got) == 0 && len(c.want) == 0) {
				t.Errorf("mask %q, limit %d: %v", c.in.Mask, limit, got)
			}
		}
	}
	for _, bad := range []string{"[", "dir/*.md"} {
		if _, errText := e.list(t, e.anna(), ListInput{Collection: "keph:wissen", Mask: bad}); errText == "" {
			t.Errorf("mask %q ohne Fehler", bad)
		}
	}
}

// breakReplica ersetzt die Replica eines Hubs durch Müll.
func (e *docEnv) breakReplica(t *testing.T, alias string) {
	t.Helper()
	if err := os.WriteFile(e.nodes.ReplicaPath(alias), []byte(strings.Repeat("kein SQLite ", 500)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Eine unlesbare Replica betrifft nur ihren Hub; die Antwort nennt keinen
// Pfad.
func TestListUnreadableReplica(t *testing.T) {
	e := newDocEnv(t)
	e.breakReplica(t, "team")
	var out ListOutput
	res, errText := e.call(t, e.anna(), "list", ListInput{}, &out)
	raw, _ := json.Marshal(res)
	if errText != "" || !reflect.DeepEqual(names(out.Entries), []string{"keph:privat", "keph:wissen"}) ||
		!reflect.DeepEqual(out.UnreadableHubs, []string{"team"}) {
		t.Errorf("Wurzel: %+v, %s", out, errText)
	}
	if _, errText := e.list(t, e.anna(), ListInput{Collection: "team:notizen"}); errText != "Hub team: Replica nicht lesbar" {
		t.Errorf("team:notizen: %s", errText)
	}
	dir := filepath.Dir(e.nodes.ReplicaPath("team"))
	if strings.Contains(string(raw), dir) || strings.Contains(string(raw), ".db") {
		t.Errorf("Pfad in der Antwort: %s", raw)
	}
}
