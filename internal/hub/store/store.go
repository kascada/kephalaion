// Package store kapselt die Datenbank des Hubs. Der Hub sieht nur die
// Schnittstelle Store; die erste Umsetzung ist SQLite, PostgreSQL soll folgen.
//
// Regel: Die Abfragen (DML) des Hubs stehen zentral in queries und laufen in
// SQLite und PostgreSQL — Platzhalter $n über sqlq.Bind, kein INSERT OR, kein
// PRAGMA, kein AUTOINCREMENT. Ein Test prüft das. Das DDL steht je Dialekt.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/kascada/kephalaion/internal/config"
	"github.com/kascada/kephalaion/internal/sqlitedb"
	"github.com/kascada/kephalaion/internal/sqlq"
)

// Role ist die Rolle, die in db_info steht.
const Role = string(config.Hub)

// SchemaVersion ist die Schemafassung, die dieses Binary erwartet. Es gibt
// noch keine Migrationen: Passt die Fassung nicht, ist die Datenbank neu
// anzulegen.
const SchemaVersion = 1

// keyRevision ist die Zeile in db_info, die die Revision des Hubs zählt.
const keyRevision = "revision"

// Info beschreibt eine geöffnete Hub-Datenbank.
type Info struct {
	SchemaVersion int
	Revision      int64
}

// Stats sind die Kennzahlen für status.
type Stats struct {
	// Documents zählt Dokumente ohne Löschmarken und ohne SYSTEM:-Zeilen.
	Documents int64
	// Collections zählt die verschiedenen collection-Werte.
	Collections int64
}

// Store ist der Zugriff des Hubs auf seine Datenbank.
type Store interface {
	Info(ctx context.Context) (Info, error)
	Stats(ctx context.Context) (Stats, error)
	Settings(ctx context.Context) (map[string]string, error)
	// ReplaceSettings ersetzt alle settings in einer Transaktion.
	ReplaceSettings(ctx context.Context, settings map[string]string) error
	Close() error
}

// queries sind die Abfragetexte des Hubs, Platzhalter $n.
var queries = struct {
	CountDocuments   string
	CountCollections string
}{
	CountDocuments: `SELECT COUNT(*) FROM documents
		WHERE deleted = 0 AND substr(name, 1, 7) <> 'SYSTEM:'`,
	CountCollections: `SELECT COUNT(DISTINCT collection) FROM documents`,
}

// sqliteSchema ist das DDL des Hubs für SQLite, nach „Datenmodell“ im
// Konzept. Das PostgreSQL-DDL entsteht mit der PostgreSQL-Umsetzung.
const sqliteSchema = `
CREATE TABLE documents (
  id          TEXT PRIMARY KEY,
  collection  TEXT NOT NULL,
  name        TEXT NOT NULL,
  content     TEXT,
  meta        TEXT,
  deleted     INTEGER NOT NULL DEFAULT 0,
  revision    INTEGER NOT NULL,
  created_at  INTEGER NOT NULL,
  created_by  TEXT NOT NULL,
  updated_at  INTEGER NOT NULL,
  updated_by  TEXT NOT NULL
);
CREATE UNIQUE INDEX documents_name ON documents(collection, name) WHERE deleted = 0;
CREATE INDEX documents_revision ON documents(collection, revision);
CREATE INDEX documents_system ON documents(name) WHERE name LIKE 'SYSTEM:%';

CREATE TABLE actions (
  at          INTEGER NOT NULL,
  account     TEXT NOT NULL,
  carrier     TEXT,
  action      TEXT NOT NULL,
  document_id TEXT,
  revision    INTEGER
);
`

// Open öffnet eine vorhandene Hub-Datenbank und prüft Rolle und
// Schemafassung. Eine fehlende Datei wird nicht angelegt.
func Open(ctx context.Context, addr config.DB) (Store, error) {
	switch addr.Kind {
	case config.SQLite:
		db, err := sqlitedb.Open(ctx, addr.Path)
		if err != nil {
			return nil, err
		}
		if _, err := sqlitedb.CheckInfo(ctx, db, Role, SchemaVersion); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("Datenbank %s: %w", addr.Path, err)
		}
		return &sqliteStore{db: db}, nil
	default:
		return nil, fmt.Errorf("Datenbankart %q: %w", addr.Kind, config.ErrUnsupported)
	}
}

// Create legt eine neue Hub-Datenbank samt Schema an. Existiert die Datei
// schon, bricht Create ab. Scheitert das Anlegen, bleibt keine Datei zurück.
func Create(ctx context.Context, addr config.DB) (Store, error) {
	switch addr.Kind {
	case config.SQLite:
		db, err := sqlitedb.Create(ctx, addr.Path)
		if err != nil {
			return nil, err
		}
		extra := map[string]string{keyRevision: "0"}
		if err := sqlitedb.CreateSchema(ctx, db, sqliteSchema, Role, SchemaVersion, extra); err != nil {
			_ = db.Close()
			_ = sqlitedb.Remove(addr.Path)
			return nil, err
		}
		return &sqliteStore{db: db}, nil
	default:
		return nil, fmt.Errorf("Datenbankart %q: %w", addr.Kind, config.ErrUnsupported)
	}
}

// sqliteStore ist die Umsetzung für SQLite.
type sqliteStore struct {
	db *sql.DB
}

func q(text string) string { return sqlq.Bind(sqlq.SQLite, text) }

func (s *sqliteStore) Info(ctx context.Context) (Info, error) {
	info, err := sqlitedb.ReadInfo(ctx, s.db)
	if err != nil {
		return Info{}, err
	}
	v, err := strconv.Atoi(info[sqlitedb.KeySchemaVersion])
	if err != nil {
		return Info{}, fmt.Errorf("db_info: Schemafassung %q: %w", info[sqlitedb.KeySchemaVersion], err)
	}
	rev, err := parseRevision(info[keyRevision])
	if err != nil {
		return Info{}, err
	}
	return Info{SchemaVersion: v, Revision: rev}, nil
}

func (s *sqliteStore) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	if err := s.db.QueryRowContext(ctx, q(queries.CountDocuments)).Scan(&st.Documents); err != nil {
		return Stats{}, fmt.Errorf("Dokumente zählen: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, q(queries.CountCollections)).Scan(&st.Collections); err != nil {
		return Stats{}, fmt.Errorf("Collections zählen: %w", err)
	}
	return st, nil
}

func (s *sqliteStore) Settings(ctx context.Context) (map[string]string, error) {
	return sqlitedb.Settings(ctx, s.db)
}

func (s *sqliteStore) ReplaceSettings(ctx context.Context, settings map[string]string) error {
	return sqlitedb.ReplaceSettings(ctx, s.db, settings)
}

func (s *sqliteStore) Close() error { return s.db.Close() }

// nextRevision zählt die Revision innerhalb der schreibenden Transaktion
// hoch und liefert die neue. Gelesen und geschrieben wird in derselben
// Transaktion, gerechnet im Code — keine SEQUENCE, kein Autoincrement, keine
// Umwandlung in SQL. Noch schreibt niemand; die Funktion steht für die
// Schreibvorgänge bereit.
func nextRevision(ctx context.Context, tx *sql.Tx) (int64, error) {
	v, err := sqlitedb.GetInfo(ctx, tx, keyRevision)
	if err != nil {
		return 0, err
	}
	rev, err := parseRevision(v)
	if err != nil {
		return 0, err
	}
	rev++
	if err := sqlitedb.SetInfo(ctx, tx, keyRevision, strconv.FormatInt(rev, 10)); err != nil {
		return 0, err
	}
	return rev, nil
}

func parseRevision(v string) (int64, error) {
	rev, err := strconv.ParseInt(v, 10, 64)
	if err != nil || rev < 0 {
		return 0, fmt.Errorf("db_info: Revision %q ist keine Zahl ≥ 0", v)
	}
	return rev, nil
}
