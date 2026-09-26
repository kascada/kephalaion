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

	"github.com/oklog/ulid/v2"

	"github.com/kascada/kephalaion/internal/config"
	"github.com/kascada/kephalaion/internal/sqlitedb"
	"github.com/kascada/kephalaion/internal/sqlq"
)

// Role ist die Rolle, die in db_info steht.
const Role = string(config.Hub)

// SchemaVersion ist die Schemafassung, die dieses Binary erwartet. Es gibt
// noch keine Migrationen: Passt die Fassung nicht, ist die Datenbank neu
// anzulegen.
const SchemaVersion = 2

// Zeilen in db_info, die nur der Hub hat.
const (
	// keyRevision zählt die Revision des Hubs.
	keyRevision = "revision"
	// keyHubID ist die Kennung des Hubs, eine ULID, vergeben beim Anlegen.
	keyHubID = "hub_id"
)

// Info beschreibt eine geöffnete Hub-Datenbank.
type Info struct {
	SchemaVersion int
	Revision      int64
	HubID         string
}

// Stats sind die Kennzahlen für status.
type Stats struct {
	// Documents zählt Dokumente ohne Löschmarken und ohne SYSTEM:-Zeilen.
	Documents int64
}

// Store ist der Zugriff des Hubs auf seine Datenbank. Jede Änderung läuft in
// einer Transaktion und schreibt eine Zeile in actions, als Account admin.
type Store interface {
	Info(ctx context.Context) (Info, error)
	Stats(ctx context.Context) (Stats, error)
	Settings(ctx context.Context) (map[string]string, error)
	// ReplaceSettings ersetzt alle settings in einer Transaktion.
	ReplaceSettings(ctx context.Context, settings map[string]string) error

	Collections(ctx context.Context) ([]Collection, error)
	AddCollection(ctx context.Context, name, description string) error
	SetCollectionDescription(ctx context.Context, name, description string) error
	// RemoveCollection entfernt eine Collection, die kein Node erlaubt hat und
	// in der keine Dokumente stehen.
	RemoveCollection(ctx context.Context, name string) error

	Nodes(ctx context.Context) ([]Node, error)
	Node(ctx context.Context, name string) (Node, error)
	// AddNode legt einen Node an und liefert sein Token. Gespeichert wird nur
	// der Hash; das Token gibt es danach nicht wieder.
	AddNode(ctx context.Context, name, description string) (token string, err error)
	SetNodeDescription(ctx context.Context, name, description string) error
	SetNodeLocked(ctx context.Context, name string, locked bool) error
	// NewNodeToken ersetzt das Token eines Nodes und liefert das neue.
	NewNodeToken(ctx context.Context, name string) (token string, err error)
	// RemoveNode entfernt einen Node samt seinen erlaubten Collections.
	RemoveNode(ctx context.Context, name string) error
	Grant(ctx context.Context, node, collection string) error
	Revoke(ctx context.Context, node, collection string) error

	// Tables liest die lokalen Tabellen für den Export.
	Tables(ctx context.Context) (Tables, error)
	// Import ersetzt in einer Transaktion die settings und, wenn tables nicht
	// nil ist, die lokalen Tabellen, und schreibt config.import in actions.
	Import(ctx context.Context, settings map[string]string, tables *Tables) error

	// PutDocument legt ein Dokument an oder ersetzt seinen Inhalt — der
	// Admin-Upsert der CLI. Unveränderter Inhalt schreibt nichts und zählt
	// keine Revision.
	PutDocument(ctx context.Context, collection, name, content string) (PutResult, error)
	// Document liest ein lebendes Dokument; eine Löschmarke gilt als nicht
	// vorhanden.
	Document(ctx context.Context, collection, name string) (Document, error)
	// Documents liest die lebenden Dokumente unter einem Verzeichnis (samt
	// Unterverzeichnissen, "" für alle), nach Name sortiert, ohne
	// SYSTEM:-Zeilen.
	Documents(ctx context.Context, collection, dir string) ([]Document, error)
	// DeleteDocument setzt eine Löschmarke: Inhalt NULL, deleted = 1, neue
	// Revision.
	DeleteDocument(ctx context.Context, collection, name string) (Document, error)
	// ImportDocuments legt an oder ersetzt wie PutDocument, alle Dokumente in
	// einer Transaktion und unter einer Revision. Was fehlt, bleibt.
	ImportDocuments(ctx context.Context, collection string, docs []DocumentInput) (ImportResult, error)

	Close() error
}

// queries sind die Abfragetexte des Hubs, Platzhalter $n.
var queries = struct {
	CountDocuments string
	LockRevision   string

	ActionInsert         string
	ActionInsertDocument string

	DocumentLive       string
	DocumentsAll       string
	DocumentsInDir     string
	DocumentLiveCount  string
	DocumentsUnderLive string
	DocumentInsert     string
	DocumentReplace    string
	DocumentDelete     string

	CollectionsAll        string
	CollectionGet         string
	CollectionInsert      string
	CollectionSetDesc     string
	CollectionDelete      string
	CollectionsDeleteAll  string
	CollectionCountDocs   string
	CollectionGrantedNode string

	NodesAll       string
	NodeGet        string
	NodeInsert     string
	NodeSetDesc    string
	NodeSetLocked  string
	NodeSetToken   string
	NodeDelete     string
	NodesDeleteAll string
	AccountRows    string

	GrantsAll       string
	GrantsOfNode    string
	GrantGet        string
	GrantInsert     string
	GrantDelete     string
	GrantsDeleteOf  string
	GrantsDeleteAll string
}{
	CountDocuments: `SELECT COUNT(*) FROM documents
		WHERE deleted = 0 AND substr(name, 1, 7) <> 'SYSTEM:'`,
	// LockRevision sperrt die Zeile der Revision schreibend, bevor sie gelesen
	// wird — für PostgreSQL, wo sonst zwei Schreiber dieselbe Revision
	// vergäben. Unter SQLite sperrt schon BEGIN IMMEDIATE.
	LockRevision: `UPDATE db_info SET value = value WHERE key = $1`,

	ActionInsert: `INSERT INTO actions (at, account, action, subject) VALUES ($1, $2, $3, $4)`,
	ActionInsertDocument: `INSERT INTO actions (at, account, action, document_id, revision)
		VALUES ($1, $2, $3, $4, $5)`,

	// Dokumente: documentColumns in dieser Reihenfolge, gelesen mit scanDocument.
	DocumentLive: `SELECT ` + documentColumns + ` FROM documents
		WHERE collection = $1 AND name = $2 AND deleted = 0`,
	// DocumentsAll und DocumentsInDir lesen die lebenden Dokumente einer
	// Collection bzw. unter einem Verzeichnis, ohne SYSTEM:-Zeilen, nach Name.
	// Das Verzeichnis grenzt ein Bereich ein: name >= 'tasks/' AND name <
	// 'tasks0' ('0' folgt auf '/') — nutzt den Index auf (collection, name).
	DocumentsAll: `SELECT ` + documentColumns + ` FROM documents
		WHERE collection = $1 AND deleted = 0 AND substr(name, 1, 7) <> 'SYSTEM:'
		ORDER BY name`,
	DocumentsInDir: `SELECT ` + documentColumns + ` FROM documents
		WHERE collection = $1 AND deleted = 0 AND name >= $2 AND name < $3
		AND substr(name, 1, 7) <> 'SYSTEM:'
		ORDER BY name`,
	DocumentLiveCount: `SELECT COUNT(*) FROM documents
		WHERE collection = $1 AND name = $2 AND deleted = 0`,
	DocumentsUnderLive: `SELECT COUNT(*) FROM documents
		WHERE collection = $1 AND deleted = 0 AND name >= $2 AND name < $3`,
	// meta bleibt NULL; der Hub deutet es nicht, und die CLI setzt es nicht.
	DocumentInsert: `INSERT INTO documents
		(id, collection, name, content, meta, deleted, revision, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, NULL, 0, $5, $6, $7, $8, $9)`,
	DocumentReplace: `UPDATE documents SET content = $2, revision = $3, updated_at = $4, updated_by = $5
		WHERE id = $1`,
	// Die Löschmarke behält id, Collection und Name, verliert Inhalt und meta.
	DocumentDelete: `UPDATE documents SET content = NULL, meta = NULL, deleted = 1,
		revision = $2, updated_at = $3, updated_by = $4
		WHERE id = $1`,

	CollectionsAll: `SELECT name, COALESCE(description, ''), created_at, created_by
		FROM collections ORDER BY name`,
	CollectionGet: `SELECT name, COALESCE(description, ''), created_at, created_by
		FROM collections WHERE name = $1`,
	CollectionInsert: `INSERT INTO collections (name, description, created_at, created_by)
		VALUES ($1, $2, $3, $4)`,
	CollectionSetDesc:    `UPDATE collections SET description = $2 WHERE name = $1`,
	CollectionDelete:     `DELETE FROM collections WHERE name = $1`,
	CollectionsDeleteAll: `DELETE FROM collections`,
	// Jede Zeile zählt: auch Löschmarken und SYSTEM:-Zeilen.
	CollectionCountDocs:   `SELECT COUNT(*) FROM documents WHERE collection = $1`,
	CollectionGrantedNode: `SELECT node FROM node_collections WHERE collection = $1 ORDER BY node`,

	NodesAll: `SELECT name, COALESCE(description, ''), token_hash, locked, created_at, created_by
		FROM nodes ORDER BY name`,
	NodeGet: `SELECT name, COALESCE(description, ''), token_hash, locked, created_at, created_by
		FROM nodes WHERE name = $1`,
	NodeInsert: `INSERT INTO nodes (name, description, token_hash, locked, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)`,
	NodeSetDesc:    `UPDATE nodes SET description = $2 WHERE name = $1`,
	NodeSetLocked:  `UPDATE nodes SET locked = $2 WHERE name = $1`,
	NodeSetToken:   `UPDATE nodes SET token_hash = $2 WHERE name = $1`,
	NodeDelete:     `DELETE FROM nodes WHERE name = $1`,
	NodesDeleteAll: `DELETE FROM nodes`,
	// Jede Zeile eines Accounts belegt den Namen, in jeder Collection, auch
	// eine Löschmarke.
	AccountRows: `SELECT COUNT(*) FROM documents WHERE name = $1`,

	GrantsAll:       `SELECT node, collection FROM node_collections ORDER BY node, collection`,
	GrantsOfNode:    `SELECT collection FROM node_collections WHERE node = $1 ORDER BY collection`,
	GrantGet:        `SELECT COUNT(*) FROM node_collections WHERE node = $1 AND collection = $2`,
	GrantInsert:     `INSERT INTO node_collections (node, collection) VALUES ($1, $2)`,
	GrantDelete:     `DELETE FROM node_collections WHERE node = $1 AND collection = $2`,
	GrantsDeleteOf:  `DELETE FROM node_collections WHERE node = $1`,
	GrantsDeleteAll: `DELETE FROM node_collections`,
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
  subject     TEXT,
  revision    INTEGER
);

CREATE TABLE collections (
  name        TEXT PRIMARY KEY,
  description TEXT,
  created_at  INTEGER NOT NULL,
  created_by  TEXT NOT NULL
);
CREATE TABLE nodes (
  name        TEXT PRIMARY KEY,
  description TEXT,
  token_hash  TEXT NOT NULL,
  locked      INTEGER NOT NULL DEFAULT 0,
  created_at  INTEGER NOT NULL,
  created_by  TEXT NOT NULL
);
CREATE TABLE node_collections (
  node        TEXT NOT NULL REFERENCES nodes(name),
  collection  TEXT NOT NULL REFERENCES collections(name),
  PRIMARY KEY (node, collection)
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
		extra := map[string]string{keyRevision: "0", keyHubID: ulid.Make().String()}
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
	return Info{SchemaVersion: v, Revision: rev, HubID: info[keyHubID]}, nil
}

func (s *sqliteStore) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	if err := s.db.QueryRowContext(ctx, q(queries.CountDocuments)).Scan(&st.Documents); err != nil {
		return Stats{}, fmt.Errorf("Dokumente zählen: %w", err)
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
// hoch und liefert die neue. Gesperrt, gelesen und geschrieben wird in
// derselben Transaktion, gerechnet im Code — keine SEQUENCE, kein
// Autoincrement, keine Umwandlung in SQL. Die Zeile wird zuerst schreibend
// gesperrt (für PostgreSQL; unter SQLite hält die Transaktion die Sperre seit
// BEGIN IMMEDIATE). Schreibvorgänge an Dokumenten holen sie über
// lazyRevision, höchstens einmal je Transaktion.
func nextRevision(ctx context.Context, tx *sql.Tx) (int64, error) {
	if _, err := tx.ExecContext(ctx, q(queries.LockRevision), keyRevision); err != nil {
		return 0, fmt.Errorf("Revision sperren: %w", err)
	}
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
