// Package store kapselt die Datenbank des Nodes (node.db): seine
// Einstellungen. Die Replicas kommen später als eigene Dateien daneben. Anders
// als beim Hub ist hier SQLite-Eigenes erlaubt — dort kommt später FTS5.
//
// Der Node kennt den Hub nicht über dessen Pakete: Kein Paket unter
// internal/node importiert eines unter internal/hub, und umgekehrt.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/kascada/kephalaion/internal/config"
	"github.com/kascada/kephalaion/internal/sqlitedb"
)

// Role ist die Rolle, die in db_info steht.
const Role = string(config.Node)

// SchemaVersion ist die Schemafassung, die dieses Binary erwartet. Es gibt
// noch keine Migrationen: Passt die Fassung nicht, ist die Datenbank neu
// anzulegen.
const SchemaVersion = 2

// Info beschreibt eine geöffnete Node-Datenbank.
type Info struct {
	SchemaVersion int
}

// Store ist der Zugriff des Nodes auf seine Datenbank.
type Store interface {
	Info(ctx context.Context) (Info, error)
	Settings(ctx context.Context) (map[string]string, error)
	// ReplaceSettings ersetzt alle settings in einer Transaktion.
	ReplaceSettings(ctx context.Context, settings map[string]string) error
	Close() error
}

// sqliteSchema ist das DDL des Nodes über den Unterbau hinaus, nach
// „Datenmodell“ im Konzept: seine Hubs und die Collections, die er von ihnen
// haben will. Beides gleicht sich nicht ab.
const sqliteSchema = `
CREATE TABLE hubs (
  name        TEXT PRIMARY KEY,
  transport   TEXT NOT NULL,
  address     TEXT,
  token       TEXT,
  ssh_key     TEXT,
  hub_id      TEXT
);
CREATE TABLE hub_collections (
  hub         TEXT NOT NULL REFERENCES hubs(name),
  collection  TEXT NOT NULL,
  PRIMARY KEY (hub, collection)
);
`

// Open öffnet eine vorhandene Node-Datenbank und prüft Rolle und
// Schemafassung. Eine fehlende Datei wird nicht angelegt.
func Open(ctx context.Context, addr config.DB) (Store, error) {
	if addr.Kind != config.SQLite {
		return nil, fmt.Errorf("Datenbankart %q: %w", addr.Kind, config.ErrUnsupported)
	}
	db, err := sqlitedb.Open(ctx, addr.Path)
	if err != nil {
		return nil, err
	}
	if _, err := sqlitedb.CheckInfo(ctx, db, Role, SchemaVersion); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Datenbank %s: %w", addr.Path, err)
	}
	return &sqliteStore{db: db}, nil
}

// Create legt eine neue Node-Datenbank samt Schema an. Existiert die Datei
// schon, bricht Create ab. Scheitert das Anlegen, bleibt keine Datei zurück.
func Create(ctx context.Context, addr config.DB) (Store, error) {
	if addr.Kind != config.SQLite {
		return nil, fmt.Errorf("Datenbankart %q: %w", addr.Kind, config.ErrUnsupported)
	}
	db, err := sqlitedb.Create(ctx, addr.Path)
	if err != nil {
		return nil, err
	}
	if err := sqlitedb.CreateSchema(ctx, db, sqliteSchema, Role, SchemaVersion, nil); err != nil {
		_ = db.Close()
		_ = sqlitedb.Remove(addr.Path)
		return nil, err
	}
	return &sqliteStore{db: db}, nil
}

type sqliteStore struct {
	db *sql.DB
}

func (s *sqliteStore) Info(ctx context.Context) (Info, error) {
	v, err := sqlitedb.GetInfo(ctx, s.db, sqlitedb.KeySchemaVersion)
	if err != nil {
		return Info{}, err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return Info{}, fmt.Errorf("db_info: Schemafassung %q: %w", v, err)
	}
	return Info{SchemaVersion: n}, nil
}

func (s *sqliteStore) Settings(ctx context.Context) (map[string]string, error) {
	return sqlitedb.Settings(ctx, s.db)
}

func (s *sqliteStore) ReplaceSettings(ctx context.Context, settings map[string]string) error {
	return sqlitedb.ReplaceSettings(ctx, s.db, settings)
}

func (s *sqliteStore) Close() error { return s.db.Close() }
