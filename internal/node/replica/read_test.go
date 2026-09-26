package replica

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/kephalaion/kephalaion/internal/contract"
	"github.com/kephalaion/kephalaion/internal/sqlitedb"
)

// readReplica legt eine Replica mit Zeilen an: rows je Collection mit
// Revision; sync_state jeder Collection auf until.
func readReplica(t *testing.T, until int64, rows ...contract.Row) *Replica {
	t.Helper()
	ctx := context.Background()
	r, err := Create(ctx, filepath.Join(t.TempDir(), "replicas", "keph.db"), ulid.Make().String(), ulid.Make().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	p := page{rows: rows, advance: map[string]int64{}, until: until, now: 1}
	for _, row := range rows {
		p.advance[row.Collection] = 0
	}
	if _, err := r.apply(ctx, p); err != nil {
		t.Fatal(err)
	}
	return r
}

// row ist eine Zeile mit Revision rev; angelegt zu Zeit created, geändert zu
// rev·1000.
func row(id, coll, name string, rev, created int64, content string) contract.Row {
	return contract.Row{ID: id, Collection: coll, Name: name, Content: &content, Revision: rev,
		CreatedAt: created, CreatedBy: "anna", UpdatedAt: rev * 1000, UpdatedBy: "bert"}
}

func tomb(id, coll, name string, rev int64) contract.Row {
	return contract.Row{ID: id, Collection: coll, Name: name, Deleted: true, Revision: rev,
		CreatedAt: 1, CreatedBy: "anna", UpdatedAt: rev * 1000, UpdatedBy: "bert"}
}

// Die generation wechselt bei jeder Neuanlage und jedem reset — auch bei
// gleicher hub_id —, sonst nicht; ohne sie ist die Replica unlesbar.
func TestGeneration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "replicas", "keph.db")
	hubID, entryID := ulid.Make().String(), ulid.Make().String()
	r, err := Create(ctx, path, hubID, entryID)
	if err != nil {
		t.Fatal(err)
	}
	g1 := r.Generation()
	if g1 == "" {
		t.Fatal("ohne generation")
	}
	if err := r.reset(ctx, hubID); err != nil {
		t.Fatal(err)
	}
	g2 := r.Generation()
	_ = r.Close()
	if g2 == g1 {
		t.Error("reset mit gleicher hub_id behält die generation")
	}
	r, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Generation() != g2 {
		t.Errorf("nach Öffnen %s, erwartet %s", r.Generation(), g2)
	}
	_ = r.Close()
	if err := Remove(path); err != nil {
		t.Fatal(err)
	}
	r, err = Create(ctx, path, hubID, entryID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Generation() == g2 || r.Generation() == g1 {
		t.Error("Neuanlage behält die generation")
	}
	_ = r.Close()

	db, err := sqlitedb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM db_info WHERE key = ?`, KeyGeneration); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if _, err := Open(ctx, path); !errors.Is(err, errNoIDs) || !unreadable(err) {
		t.Errorf("ohne generation: %v", err)
	}
}

func TestEntryLookups(t *testing.T) {
	ctx := context.Background()
	acc := "{}"
	r := readReplica(t, 9,
		row("A", "wissen", "a.md", 1, 10, "äh"), // 3 Bytes
		row("B", "wissen", "dir/b.md", 2, 20, "b"),
		tomb("C", "wissen", "weg.md", 3),
		row("D", "wissen", contract.AccountRowName("bob"), 4, 1, acc),
		row("E", "wissen", "dir/sub/c.md", 5, 30, ""),
		// Zwei lebende Zeilen gleichen Namens: Es gilt die jüngste.
		row("F", "wissen", "doppelt.md", 6, 40, "alt"),
		row("G", "wissen", "doppelt.md", 7, 50, "neu"),
		row("H", "andere", "a.md", 8, 60, "x"),
	)
	e, ok, err := r.EntryByName(ctx, "wissen", "a.md", true)
	if err != nil || !ok || e.ID != "A" || e.Size != 3 || e.Content == nil || *e.Content != "äh" ||
		e.CreatedAt != 10 || e.CreatedBy != "anna" || e.UpdatedAt != 1000 || e.UpdatedBy != "bert" || e.Revision != 1 {
		t.Errorf("a.md: %+v, %v, %v", e, ok, err)
	}
	if e, ok, err := r.EntryByName(ctx, "wissen", "a.md", false); err != nil || !ok || e.Content != nil || e.Size != 3 {
		t.Errorf("a.md ohne Inhalt: %+v, %v, %v", e, ok, err)
	}
	if e, ok, _ := r.EntryByName(ctx, "wissen", "doppelt.md", true); !ok || e.ID != "G" || *e.Content != "neu" {
		t.Errorf("doppelt.md: %+v", e)
	}
	for _, name := range []string{"weg.md", "dir", "fehlt.md"} {
		if _, ok, err := r.EntryByName(ctx, "wissen", name, true); ok || err != nil {
			t.Errorf("%s: %v, %v", name, ok, err)
		}
	}
	if _, _, err := r.EntryByName(ctx, "wissen", contract.AccountRowName("bob"), true); err == nil {
		t.Error("SYSTEM:-Name ohne Fehler")
	}
	if e, ok, _ := r.EntryByID(ctx, "C", true); !ok || !e.Deleted || e.Content != nil || e.Size != 0 {
		t.Errorf("Löschmarke per id: %+v, %v", e, ok)
	}
	if e, ok, _ := r.EntryByID(ctx, "E", true); !ok || e.Name != "dir/sub/c.md" || e.Content == nil || *e.Content != "" {
		t.Errorf("per id: %+v, %v", e, ok)
	}
	for _, id := range []string{"D", "fehlt"} {
		if _, ok, err := r.EntryByID(ctx, id, true); ok || err != nil {
			t.Errorf("id %s: %v, %v", id, ok, err)
		}
	}
	for prefix, want := range map[string]bool{"dir/": true, "dir/sub/": true, "di/": false, "a.md/": false, "weg.md/": false} {
		if got, err := r.HasUnder(ctx, "wissen", prefix); err != nil || got != want {
			t.Errorf("HasUnder %s: %v, %v", prefix, got, err)
		}
	}
	if dirs, err := r.ChildDirs(ctx, "wissen", ""); err != nil || !reflect.DeepEqual(dirs, []string{"dir"}) {
		t.Errorf("ChildDirs: %v, %v", dirs, err)
	}
	if dirs, err := r.ChildDirs(ctx, "wissen", "dir/"); err != nil || !reflect.DeepEqual(dirs, []string{"sub"}) {
		t.Errorf("ChildDirs dir/: %v, %v", dirs, err)
	}
	if rev, ok, err := r.StateOf(ctx, "wissen"); err != nil || !ok || rev != 9 {
		t.Errorf("StateOf: %d, %v, %v", rev, ok, err)
	}
	if _, ok, err := r.StateOf(ctx, "fehlt"); err != nil || ok {
		t.Errorf("StateOf fehlt: %v, %v", ok, err)
	}
}
