package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kascada/kephalaion/internal/config"
	"github.com/kascada/kephalaion/internal/sqlitedb"
	"github.com/kascada/kephalaion/internal/sqlq"
)

func TestQueriesPortable(t *testing.T) {
	for name, text := range sqlq.Texts(&queries) {
		if err := sqlq.Check(text); err != nil {
			t.Errorf("queries.%s: %v", name, err)
		}
	}
}

func newDB(t *testing.T) config.DB {
	t.Helper()
	return config.SQLiteDB(filepath.Join(t.TempDir(), "hub.db"))
}

func TestCreateAndReopen(t *testing.T) {
	ctx := context.Background()
	addr := newDB(t)
	s, err := Create(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSettings(ctx, map[string]string{"listen": ":8080"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Create(ctx, addr); !errors.Is(err, sqlitedb.ErrExists) {
		t.Fatalf("zweites Create: %v", err)
	}

	s, err = Open(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	info, err := s.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.SchemaVersion != SchemaVersion || info.Revision != 0 {
		t.Errorf("Info = %+v", info)
	}
	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st != (Stats{}) {
		t.Errorf("Stats = %+v", st)
	}
	settings, err := s.Settings(ctx)
	if err != nil || settings["listen"] != ":8080" {
		t.Errorf("Settings = %v, %v", settings, err)
	}
}

func TestOpenMissing(t *testing.T) {
	addr := newDB(t)
	if _, err := Open(context.Background(), addr); !errors.Is(err, sqlitedb.ErrNotFound) {
		t.Fatalf("Open: %v", err)
	}
	if _, err := os.Stat(addr.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Open hat die Datei angelegt: %v", err)
	}
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	addr := newDB(t)
	s, err := Create(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	db := s.(*sqliteStore).db
	for _, row := range []struct {
		id, coll, name string
		deleted        int
	}{
		{"1", "a", "x.md", 0},
		{"2", "a", "y.md", 1},
		{"3", "b", "SYSTEM:A:kleist", 0},
		{"4", "b", "z.md", 0},
		{"5", "c", "system:klein", 0},
	} {
		_, err := db.ExecContext(ctx, `INSERT INTO documents
			(id, collection, name, deleted, revision, created_at, created_by, updated_at, updated_by)
			VALUES (?, ?, ?, ?, 1, 0, 'x', 0, 'x')`, row.id, row.coll, row.name, row.deleted)
		if err != nil {
			t.Fatal(err)
		}
	}
	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Stats{Documents: 3, Collections: 3}); st != want {
		t.Errorf("Stats = %+v, erwartet %+v", st, want)
	}
}

func TestNextRevision(t *testing.T) {
	ctx := context.Background()
	s, err := Create(ctx, newDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	db := s.(*sqliteStore).db
	for want := int64(1); want <= 2; want++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := nextRevision(ctx, tx)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("nextRevision = %d, erwartet %d", got, want)
		}
	}
	// Eine zurückgerollte Transaktion lässt die Revision stehen.
	tx, _ := db.BeginTx(ctx, nil)
	if _, err := nextRevision(ctx, tx); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	info, err := s.Info(ctx)
	if err != nil || info.Revision != 2 {
		t.Errorf("Revision = %d, %v, erwartet 2", info.Revision, err)
	}
}

func TestWrongSchemaVersion(t *testing.T) {
	ctx := context.Background()
	addr := newDB(t)
	s, err := Create(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlitedb.SetInfo(ctx, s.(*sqliteStore).db, sqlitedb.KeySchemaVersion, "99"); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	_, err = Open(ctx, addr)
	var sv *sqlitedb.SchemaVersionError
	if !errors.As(err, &sv) {
		t.Fatalf("Open: %v, erwartet SchemaVersionError", err)
	}
}

func TestWrongRole(t *testing.T) {
	ctx := context.Background()
	addr := newDB(t)
	db, err := sqlitedb.Create(ctx, addr.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlitedb.CreateSchema(ctx, db, "", "node", SchemaVersion, nil); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	_, err = Open(ctx, addr)
	var wr *sqlitedb.WrongRoleError
	if !errors.As(err, &wr) {
		t.Fatalf("Open: %v, erwartet WrongRoleError", err)
	}
}
