package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kascada/kephalaion/internal/config"
	"github.com/kascada/kephalaion/internal/sqlitedb"
)

func newDB(t *testing.T) config.DB {
	t.Helper()
	return config.SQLiteDB(filepath.Join(t.TempDir(), "node.db"))
}

func TestCreateAndReopen(t *testing.T) {
	ctx := context.Background()
	addr := newDB(t)
	s, err := Create(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSettings(ctx, map[string]string{"listen": "127.0.0.1:7070"}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	if _, err := Create(ctx, addr); !errors.Is(err, sqlitedb.ErrExists) {
		t.Fatalf("zweites Create: %v", err)
	}

	s, err = Open(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	info, err := s.Info(ctx)
	if err != nil || info.SchemaVersion != SchemaVersion {
		t.Errorf("Info = %+v, %v", info, err)
	}
	settings, err := s.Settings(ctx)
	if err != nil || settings["listen"] != "127.0.0.1:7070" {
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
	var sv *sqlitedb.SchemaVersionError
	if _, err := Open(ctx, addr); !errors.As(err, &sv) {
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
	if err := sqlitedb.CreateSchema(ctx, db, "", "hub", SchemaVersion, nil); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	var wr *sqlitedb.WrongRoleError
	if _, err := Open(ctx, addr); !errors.As(err, &wr) {
		t.Fatalf("Open: %v, erwartet WrongRoleError", err)
	}
}
