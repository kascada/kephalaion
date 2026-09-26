package store

import (
	"context"
	"testing"

	"github.com/kascada/kephalaion/internal/contract"
	"github.com/kascada/kephalaion/internal/sqlq"
)

func TestSyncRowsQueryPortable(t *testing.T) {
	for _, n := range []int{1, 2, 7} {
		for _, limit := range []bool{false, true} {
			if err := sqlq.Check(syncRowsQuery(n, limit)); err != nil {
				t.Errorf("syncRowsQuery(%d, %v): %v", n, limit, err)
			}
		}
	}
	want := `(collection = $2 AND revision > $3) OR (collection = $4 AND revision > $5)) ORDER BY revision, id LIMIT $6`
	if got := syncRowsQuery(2, true); len(got) < len(want) || got[len(got)-len(want):] != want {
		t.Errorf("syncRowsQuery(2, true) = %q", got)
	}
}

// TestSyncRows prüft die Zeilen des Abgleichs: alle Spalten, NULL als nil,
// Löschmarken und SYSTEM:-Zeilen, Ordnung, obere Revision und Grenze.
func TestSyncRows(t *testing.T) {
	ctx := context.Background()
	s := newDocStore(t)
	if _, err := s.PutDocument(ctx, "a", "x.md", "eins"); err != nil { // Revision 1
		t.Fatal(err)
	}
	if _, err := s.PutDocument(ctx, "b", "y.md", "zwei"); err != nil { // 2
		t.Fatal(err)
	}
	if _, err := s.DeleteDocument(ctx, "a", "x.md"); err != nil { // 3
		t.Fatal(err)
	}
	// Eine SYSTEM:-Zeile mit meta, wie sie später der Hub selbst schreibt.
	if _, err := s.(*sqliteStore).db.Exec(`INSERT INTO documents
		(id, collection, name, content, meta, deleted, revision, created_at, created_by, updated_at, updated_by)
		VALUES ('01SYS', 'a', 'SYSTEM:A:carol', '{}', '{"k":1}', 0, 4, 1, 'admin', 2, 'admin')`); err != nil {
		t.Fatal(err)
	}
	all := []contract.Since{{Collection: "a", Since: 0}, {Collection: "b", Since: 0}}

	rows, err := s.SyncRows(ctx, all, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("Zeilen = %+v", rows)
	}
	if rows[0].Name != "y.md" || rows[0].Revision != 2 || rows[0].Content == nil || *rows[0].Content != "zwei" || rows[0].Meta != nil {
		t.Errorf("Zeile 0 = %+v", rows[0])
	}
	del := rows[1]
	if del.Name != "x.md" || !del.Deleted || del.Content != nil || del.Meta != nil || del.Revision != 3 ||
		del.CreatedBy != Admin || del.UpdatedBy != Admin || del.CreatedAt == 0 || del.UpdatedAt < del.CreatedAt {
		t.Errorf("Löschmarke = %+v", del)
	}
	sys := rows[2]
	if sys.ID != "01SYS" || sys.Name != "SYSTEM:A:carol" || sys.Meta == nil || *sys.Meta != `{"k":1}` ||
		sys.Content == nil || *sys.Content != "{}" || sys.CreatedAt != 1 || sys.UpdatedAt != 2 {
		t.Errorf("SYSTEM:-Zeile = %+v", sys)
	}

	// Obere Revision, seit je Collection, Grenze.
	rows, err = s.SyncRows(ctx, []contract.Since{{Collection: "a", Since: 0}}, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Revision != 3 {
		t.Errorf("bis 3, nur a = %+v", rows)
	}
	rows, err = s.SyncRows(ctx, []contract.Since{{Collection: "a", Since: 3}, {Collection: "b", Since: 2}}, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != "01SYS" {
		t.Errorf("seit 3/2 = %+v", rows)
	}
	rows, err = s.SyncRows(ctx, all, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Revision != 2 || rows[1].Revision != 3 {
		t.Errorf("Grenze 2 = %+v", rows)
	}
	rows, err = s.SyncRows(ctx, nil, 4, 0)
	if err != nil || len(rows) != 0 {
		t.Errorf("ohne Collections = %+v, %v", rows, err)
	}
}
