package mcpnode

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/kephalaion/kephalaion/internal/contract"
	"github.com/kephalaion/kephalaion/internal/ident"
)

// restore spielt den Hub auf eine frühere Revision zurück, mit gleicher
// hub_id — wie eine Sicherung.
func (f *docHub) restore(rev int64) {
	for id, r := range f.docs {
		if r.Revision > rev {
			delete(f.docs, id)
		}
	}
	f.rev = rev
}

func (e *docEnv) changes(t *testing.T, h map[string][]string, in ChangesInput) (ChangesOutput, string) {
	t.Helper()
	var out ChangesOutput
	res, errText := e.call(t, h, "changes", in, &out)
	if errText == "" {
		raw, _ := json.Marshal(res)
		if strings.Contains(string(raw), "SYSTEM") {
			t.Errorf("SYSTEM: in der Antwort: %s", raw)
		}
		if out.Cursor == "" {
			t.Errorf("ohne cursor: %s", raw)
		}
	}
	return out, errText
}

// changed sind die Änderungen als „Adresse Name“, gelöscht mit „-“ davor.
func changed(cs []Change) []string {
	out := []string{}
	for _, c := range cs {
		s := c.Address + " " + c.Name
		if c.Deleted {
			s = "-" + s
		}
		out = append(out, s)
	}
	return out
}

// drain fragt mit limit weiter, bis more nicht mehr gilt, und liefert alle
// Änderungen und den letzten Stand.
func (e *docEnv) drain(t *testing.T, h map[string][]string, in ChangesInput, limit int) ([]Change, ChangesOutput) {
	t.Helper()
	in.Limit = limit
	var all []Change
	for {
		out, errText := e.changes(t, h, in)
		if errText != "" {
			t.Fatal(errText)
		}
		if len(out.Changes) > limit || (out.More && len(out.Changes) != limit) {
			t.Fatalf("Seite: %+v", out)
		}
		all = append(all, out.Changes...)
		in.Cursor, in.Since = out.Cursor, ""
		if !out.More {
			return all, out
		}
	}
}

func TestChangesFromNow(t *testing.T) {
	e := newDocEnv(t)
	e.fillWissen(t)
	out, errText := e.changes(t, e.anna(), ChangesInput{})
	if errText != "" || len(out.Changes) != 0 || out.More || out.Reset != nil || out.Dropped != nil {
		t.Fatalf("ab jetzt: %+v, %s", out, errText)
	}
	cur := out.Cursor
	// Ohne neue Änderung: nichts, derselbe Stand.
	if out, _ := e.changes(t, e.anna(), ChangesInput{Cursor: cur}); len(out.Changes) != 0 || out.Cursor != cur {
		t.Errorf("unverändert: %+v", out)
	}
	f := e.hubs["keph"]
	id := f.put("wissen", "neu.md", "1")
	e.hubs["team"].put("notizen", "n.md", "x")
	e.sync(t)
	out, _ = e.changes(t, e.anna(), ChangesInput{Cursor: cur})
	if got := changed(out.Changes); !reflect.DeepEqual(got, []string{"keph:wissen neu.md", "team:notizen n.md"}) {
		t.Errorf("nach put: %v", got)
	}
	c := out.Changes[0]
	if c.ID != id || c.Revision != f.docs[id].Revision || c.Updated.By != "kleist" || c.Updated.At != formatMillis(f.docs[id].UpdatedAt) {
		t.Errorf("Eintrag: %+v", c)
	}
	// Ein cursor gehört zu seiner Anfrage.
	for _, bad := range []ChangesInput{{Cursor: cur, Collection: "keph:wissen"}, {Cursor: cur, Path: "dir"},
		{Cursor: cur, Since: "2026-01-01T00:00:00Z"}, {Cursor: "x"}, {Since: "gestern"}} {
		if _, errText := e.changes(t, e.anna(), bad); errText == "" {
			t.Errorf("%+v ohne Fehler", bad)
		}
	}
}

// Löschmarke, Umbenennen mit gleicher id, mehrfach geändert = einmal; ohne
// SYSTEM:-Zeilen, auch wenn sich Rechte ändern.
func TestChangesKinds(t *testing.T) {
	e := newDocEnv(t)
	ids := e.fillWissen(t)
	out, _ := e.changes(t, e.otto(), ChangesInput{Collection: "wissen"})
	cur := out.Cursor
	f := e.hubs["keph"]
	f.rm(ids["b.txt"])
	f.change(ids["dir/c.md"], func(r *contract.Row) { r.Name = "dir/umbenannt.md" })
	for i := range 3 {
		f.put("wissen", "0001-x.md", strings.Repeat("x", i+1))
	}
	f.grant("otto", "wissen", e.tokens["keph/otto"], contract.Rights{Write: true})
	e.sync(t)
	out, _ = e.changes(t, e.otto(), ChangesInput{Collection: "wissen", Cursor: cur})
	want := []string{"-keph:wissen b.txt", "keph:wissen dir/umbenannt.md", "keph:wissen 0001-x.md"}
	if got := changed(out.Changes); !reflect.DeepEqual(got, want) {
		t.Fatalf("Änderungen: %v", got)
	}
	if out.Changes[1].ID != ids["dir/c.md"] || out.Changes[2].ID != ids["0001-x.md"] ||
		out.Changes[2].Revision != f.docs[ids["0001-x.md"]].Revision {
		t.Errorf("ids: %+v", out.Changes)
	}
	// path als Präfix.
	all, _ := e.drain(t, e.otto(), ChangesInput{Collection: "keph:wissen", Path: "dir", Since: "2000-01-01T00:00:00Z"}, 100)
	if got := changed(all); !reflect.DeepEqual(got, []string{"keph:wissen dir/sub/d.md", "keph:wissen dir/umbenannt.md"}) {
		t.Errorf("path: %v", got)
	}
}

// Der cursor trägt den Stand je Collection: über zwei Collections mit
// verschiedenem Stand, in kleinen Seiten, kommt jede Änderung genau einmal;
// eine Collection, die neu lesbar wird, liefert alles, auch wenn ihre
// Revisionen unter dem Stand der anderen liegen.
func TestChangesCursorPerCollection(t *testing.T) {
	e := newDocEnv(t)
	_, first := e.drain(t, e.anna(), ChangesInput{}, 100)
	f := e.hubs["keph"]
	var want []string
	for i := range 4 {
		name := "w" + string(rune('a'+i)) + ".md"
		f.put("wissen", name, "w")
		want = append(want, "keph:wissen "+name)
	}
	// Eine Revision mit mehreren Zeilen: das Ende einer Seite fällt mitten
	// hinein.
	f.write(func(rev, at int64) {
		for _, n := range []string{"p1.md", "p2.md", "p3.md"} {
			c := "p"
			id := ulid.Make().String()
			f.docs[id] = contract.Row{ID: id, Collection: "privat", Name: n, Content: &c, Revision: rev, CreatedAt: at,
				CreatedBy: "kleist", UpdatedAt: at, UpdatedBy: "kleist"}
			want = append(want, "keph:privat "+n)
		}
	})
	e.sync(t)
	sort.Strings(want)
	for _, limit := range []int{1, 2, 3, 100} {
		all, _ := e.drain(t, e.anna(), ChangesInput{Cursor: first.Cursor}, limit)
		got := changed(all)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("limit %d: %v", limit, got)
		}
	}

	// Neue Collection am Node, mit Recht: ihre Zeilen haben kleinere
	// Revisionen als der Stand von wissen und kommen trotzdem.
	_, now := e.drain(t, e.anna(), ChangesInput{}, 100)
	f.allowed["extra"] = true
	f.write(func(rev, at int64) {}) // eine Revision ohne Zeilen
	ctx := context.Background()
	if err := e.nodes.AddCollection(ctx, "keph", "extra"); err != nil {
		t.Fatal(err)
	}
	// Die Zeilen von extra entstehen vor allem anderen: kleine Revisionen.
	extraRev := int64(1)
	c := "e"
	id := ulid.Make().String()
	f.docs[id] = contract.Row{ID: id, Collection: "extra", Name: "alt.md", Content: &c, Revision: extraRev,
		CreatedAt: 1, CreatedBy: "kleist", UpdatedAt: 1, UpdatedBy: "kleist"}
	hash := contract.AccountContent{Hash: ident.HashToken(e.tokens["keph/anna"]), User: "anna"}
	enc, _ := contract.EncodeAccountContent(hash)
	f.docs["ACC"] = contract.Row{ID: "ACC", Collection: "extra", Name: contract.AccountRowName("anna"), Content: &enc,
		Revision: extraRev, CreatedAt: 1, CreatedBy: "admin", UpdatedAt: 1, UpdatedBy: "admin"}
	e.sync(t)
	all, _ := e.drain(t, e.anna(), ChangesInput{Cursor: now.Cursor}, 1)
	if got := changed(all); !reflect.DeepEqual(got, []string{"keph:extra alt.md"}) {
		t.Errorf("neue Collection: %v", got)
	}
}

// Wird die Replica neu angelegt oder geleert, meldet changes reset — je Hub;
// die anderen laufen lückenlos weiter. Auch nach einer Sicherung mit
// gleicher hub_id.
func TestChangesReset(t *testing.T) {
	e := newDocEnv(t)
	e.fillWissen(t)
	_, now := e.drain(t, e.anna(), ChangesInput{}, 100)
	f := e.hubs["keph"]

	// Andere hub_id.
	f.id = ulid.Make().String()
	e.hubs["team"].put("notizen", "t.md", "t")
	e.sync(t)
	out, _ := e.changes(t, e.anna(), ChangesInput{Cursor: now.Cursor})
	if !reflect.DeepEqual(out.Reset, []string{"keph"}) || !reflect.DeepEqual(changed(out.Changes), []string{"team:notizen t.md"}) ||
		out.Dropped != nil {
		t.Fatalf("hub_id: %+v", out)
	}
	cur := out.Cursor
	if out, _ := e.changes(t, e.anna(), ChangesInput{Cursor: cur}); out.Reset != nil || len(out.Changes) != 0 {
		t.Errorf("nach reset: %+v", out)
	}

	// Sicherung: gleiche hub_id, die Revisionen kommen neu.
	back := f.rev
	f.put("wissen", "spaeter.md", "weg")
	e.sync(t)
	out, _ = e.changes(t, e.anna(), ChangesInput{Cursor: cur})
	if got := changed(out.Changes); !reflect.DeepEqual(got, []string{"keph:wissen spaeter.md"}) {
		t.Fatalf("vor der Sicherung: %v", got)
	}
	cur = out.Cursor
	f.restore(back)
	e.sync(t)
	out, _ = e.changes(t, e.anna(), ChangesInput{Cursor: cur})
	if !reflect.DeepEqual(out.Reset, []string{"keph"}) || len(out.Changes) != 0 {
		t.Fatalf("Sicherung: %+v", out)
	}
	// Nur der Hub mit reset; mit collection: nur ihrer.
	_, one := e.drain(t, e.anna(), ChangesInput{Collection: "team:notizen"}, 100)
	f.id = ulid.Make().String()
	e.hubs["team"].put("notizen", "t2.md", "t")
	e.sync(t)
	out, _ = e.changes(t, e.anna(), ChangesInput{Collection: "team:notizen", Cursor: one.Cursor})
	if out.Reset != nil || !reflect.DeepEqual(changed(out.Changes), []string{"team:notizen t2.md"}) {
		t.Errorf("anderer Hub: %+v", out)
	}
}

// Fällt eine Collection aus dem cursor, meldet changes sie als weggefallen:
// Recht entzogen, am Node abgewählt, Hub-Eintrag entfernt.
func TestChangesDropped(t *testing.T) {
	e := newDocEnv(t)
	e.fillWissen(t)
	_, now := e.drain(t, e.anna(), ChangesInput{}, 100)
	_, one := e.drain(t, e.anna(), ChangesInput{Collection: "keph:privat"}, 100)
	f := e.hubs["keph"]
	f.grant("anna", "privat", "", contract.Rights{}) // revoke
	e.sync(t)
	out, _ := e.changes(t, e.anna(), ChangesInput{Cursor: now.Cursor})
	if !reflect.DeepEqual(out.Dropped, []string{"keph:privat"}) || len(out.Changes) != 0 {
		t.Fatalf("Recht entzogen: %+v", out)
	}
	if out, errText := e.changes(t, e.anna(), ChangesInput{Collection: "keph:privat", Cursor: one.Cursor}); errText != "" ||
		!reflect.DeepEqual(out.Dropped, []string{"keph:privat"}) {
		t.Errorf("einzelne Collection: %+v, %s", out, errText)
	}
	// Einmal gemeldet, danach nicht mehr im cursor.
	if out, _ := e.changes(t, e.anna(), ChangesInput{Cursor: out.Cursor}); out.Dropped != nil {
		t.Errorf("zweimal: %+v", out)
	}
	// Ohne cursor ist die einzelne Collection nicht lesbar.
	if _, errText := e.changes(t, e.anna(), ChangesInput{Collection: "keph:privat"}); !strings.Contains(errText, "nicht lesbar") {
		t.Errorf("ohne cursor: %s", errText)
	}

	_, now = e.drain(t, e.anna(), ChangesInput{}, 100)
	ctx := context.Background()
	if err := e.nodes.RemoveCollection(ctx, "keph", "wissen"); err != nil {
		t.Fatal(err)
	}
	e.sync(t)
	out, _ = e.changes(t, e.anna(), ChangesInput{Cursor: now.Cursor})
	if !reflect.DeepEqual(out.Dropped, []string{"keph:wissen"}) {
		t.Errorf("abgewählt: %+v", out)
	}
	_, now = e.drain(t, e.anna(), ChangesInput{}, 100)
	if err := e.nodes.RemoveHub(ctx, "team"); err != nil {
		t.Fatal(err)
	}
	out, _ = e.changes(t, e.anna(), ChangesInput{Cursor: now.Cursor})
	if !reflect.DeepEqual(out.Dropped, []string{"team:notizen"}) {
		t.Errorf("Hub entfernt: %+v", out)
	}
}

// since ist die Zeit des Hubs beim Schreiben.
func TestChangesSince(t *testing.T) {
	e := newDocEnv(t)
	f := e.hubs["keph"]
	f.put("wissen", "vorher.md", "1")
	mid := time.UnixMilli(f.clock + 500).UTC().Format(time.RFC3339Nano)
	f.put("wissen", "nachher.md", "2")
	e.hubs["team"].clock = f.clock
	e.hubs["team"].put("notizen", "n.md", "3")
	e.sync(t)
	all, last := e.drain(t, e.anna(), ChangesInput{Since: mid}, 1)
	if got := changed(all); !reflect.DeepEqual(got, []string{"keph:wissen nachher.md", "team:notizen n.md"}) {
		t.Errorf("since: %v", got)
	}
	f.put("wissen", "danach.md", "4")
	e.sync(t)
	out, _ := e.changes(t, e.anna(), ChangesInput{Cursor: last.Cursor})
	if got := changed(out.Changes); !reflect.DeepEqual(got, []string{"keph:wissen danach.md"}) {
		t.Errorf("weiter nach since: %v", got)
	}
	future := time.UnixMilli(f.clock + 10000).UTC().Format(time.RFC3339)
	if out, _ := e.changes(t, e.anna(), ChangesInput{Since: future}); len(out.Changes) != 0 {
		t.Errorf("Zukunft: %v", changed(out.Changes))
	}
}

// Eine unlesbare Replica betrifft nur ihren Hub: Ihr Stand bleibt im
// cursor, die anderen laufen weiter; die Antwort nennt keinen Pfad.
func TestChangesUnreadableReplica(t *testing.T) {
	e := newDocEnv(t)
	_, now := e.drain(t, e.anna(), ChangesInput{}, 100)
	e.hubs["keph"].put("wissen", "w.md", "w")
	e.hubs["team"].put("notizen", "n.md", "n")
	e.sync(t)
	e.breakReplica(t, "team")
	out, errText := e.changes(t, e.anna(), ChangesInput{Cursor: now.Cursor})
	if errText != "" || !reflect.DeepEqual(out.UnreadableHubs, []string{"team"}) || out.Dropped != nil ||
		!reflect.DeepEqual(changed(out.Changes), []string{"keph:wissen w.md"}) {
		t.Fatalf("unlesbar: %+v, %s", out, errText)
	}
	var c changesCursor
	if err := decodeCursor(out.Cursor, &c); err != nil {
		t.Fatal(err)
	}
	var old changesCursor
	_ = decodeCursor(now.Cursor, &old)
	if !reflect.DeepEqual(c.Hubs["team"], old.Hubs["team"]) {
		t.Errorf("Stand von team: %+v, vorher %+v", c.Hubs["team"], old.Hubs["team"])
	}
	if _, errText := e.changes(t, e.anna(), ChangesInput{Collection: "team:notizen"}); errText != "Hub team: Replica nicht lesbar" {
		t.Errorf("einzelne Collection: %s", errText)
	}
}
