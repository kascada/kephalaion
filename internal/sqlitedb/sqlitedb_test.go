package sqlitedb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kascada/kephalaion/internal/sqlq"
)

func TestQueriesPortable(t *testing.T) {
	for name, text := range sqlq.Texts(&Queries) {
		if err := sqlq.Check(text); err != nil {
			t.Errorf("Queries.%s: %v", name, err)
		}
	}
}

func TestOpenMissingCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fehlt.db")
	if _, err := Open(context.Background(), path); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open: %v, erwartet ErrNotFound", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("Open hat angelegt: %v", entries)
	}
}

func TestCreateOpenCheck(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "x.db")
	db, err := Create(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateSchema(ctx, db, "CREATE TABLE extra (a TEXT);", "hub", 3, map[string]string{"revision": "0"}); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q, %v", mode, err)
	}
	var fk, busy int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d, %v", fk, err)
	}
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil || busy != busyTimeout {
		t.Errorf("busy_timeout = %d, %v", busy, err)
	}
	_ = db.Close()

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("Rechte %v, erwartet 0600", fi.Mode().Perm())
	}

	if _, err := Create(ctx, path); !errors.Is(err, ErrExists) {
		t.Fatalf("zweites Create: %v, erwartet ErrExists", err)
	}

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	info, err := CheckInfo(ctx, db, "hub", 3)
	if err != nil {
		t.Fatal(err)
	}
	if info[KeyRole] != "hub" || info["revision"] != "0" || info[KeyCreatedAt] == "" {
		t.Errorf("db_info = %v", info)
	}
	var wr *WrongRoleError
	if _, err := CheckInfo(ctx, db, "node", 3); !errors.As(err, &wr) {
		t.Errorf("falsche Rolle: %v", err)
	}
	var sv *SchemaVersionError
	if _, err := CheckInfo(ctx, db, "hub", 4); !errors.As(err, &sv) {
		t.Errorf("falsche Schemafassung: %v", err)
	}

	if err := SetInfo(ctx, db, "revision", "7"); err != nil {
		t.Fatal(err)
	}
	if v, err := GetInfo(ctx, db, "revision"); err != nil || v != "7" {
		t.Errorf("GetInfo = %q, %v", v, err)
	}
	if err := SetInfo(ctx, db, "gibtsnicht", "1"); err == nil {
		t.Error("SetInfo auf fehlende Zeile hätte scheitern sollen")
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "x.db")
	db, err := Create(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := CreateSchema(ctx, db, "", "node", 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceSettings(ctx, db, map[string]string{"a": "1", "b": "2"}); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceSettings(ctx, db, map[string]string{"b": "3", "c": "4"}); err != nil {
		t.Fatal(err)
	}
	got, err := Settings(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["b"] != "3" || got["c"] != "4" {
		t.Errorf("Settings = %v", got)
	}
}

func TestNotADatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "leer.db")
	db, err := Create(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := CheckInfo(ctx, db, "hub", 1); err == nil {
		t.Fatal("leere Datenbank ohne db_info hätte scheitern sollen")
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.db")
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Remove(path); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("liegen geblieben: %v", entries)
	}
}

func TestPathWithSpecialChars(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "a b?c#d%e")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "x.db")
	db, err := Create(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateSchema(ctx, db, "", "hub", 1, nil); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if e.Name() == "x.db" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Datei nicht am erwarteten Ort: %v", entries)
	}
}
