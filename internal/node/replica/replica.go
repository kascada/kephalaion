// Package replica ist die Replica des Nodes: je Hub-Eintrag eine eigene
// SQLite-Datei unter replicas/<alias>.db neben node.db, dazu der Abgleich,
// der sie über den Vertrag (internal/contract) füllt.
//
// Die Replica ist abgeleitet: Sie enthält genau die Zeilen, die der Hub
// geliefert hat, und lässt sich jederzeit neu abgleichen. Deshalb ist sie die
// einzige Datenbank, die nicht init anlegt, sondern der erste Abgleich. Auf
// dem Node zählt die id eines Dokuments, nicht sein Name: documents hat
// keinen eindeutigen Index auf den Namen, so dass Umbenennungen in jeder
// Reihenfolge ankommen dürfen.
//
// Wie der Node-Store darf die Replica SQLite-Eigenes benutzen. Sie kennt den
// Hub nur über internal/contract.
package replica

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kascada/kephalaion/internal/contract"
	"github.com/kascada/kephalaion/internal/ident"
	"github.com/kascada/kephalaion/internal/sqlitedb"
)

// Role ist die Rolle, die in db_info einer Replica steht. Keine Rolle der
// config: Eine Replica gehört zum Node.
const Role = "replica"

// SchemaVersion ist die Schemafassung der Replica. Passt sie nicht, verwirft
// der Abgleich die Replica und legt sie neu an; sie ist abgeleitet.
const SchemaVersion = 2

// KeyHubID ist der Schlüssel der hub_id in db_info. Sie ist maßgeblich; die
// Spalte hubs.hub_id in node.db ist nur Kopie.
const KeyHubID = "hub_id"

// ErrNotFound meldet eine Collection oder ein Dokument, das es in der
// Replica nicht gibt.
var ErrNotFound = errors.New("gibt es in der Replica nicht")

// schema ist das DDL der Replica über den Unterbau hinaus: documents wie am
// Hub, aber ohne eindeutigen Index auf den Namen — auf dem Node zählt die
// id —, und der Stand des Abgleichs je Collection. documents_system findet
// die Zeilen eines Accounts quer über die Collections; der Node fragt ihn
// bei jeder Anfrage eines Clients (siehe AccountRows).
const schema = `
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
CREATE INDEX documents_name ON documents(collection, name);
CREATE INDEX documents_revision ON documents(collection, revision);
CREATE INDEX documents_system ON documents(name) WHERE name LIKE 'SYSTEM:%';

CREATE TABLE sync_state (
  collection  TEXT PRIMARY KEY,
  revision    INTEGER NOT NULL,
  synced_at   INTEGER NOT NULL
);
`

// documentColumns sind die Spalten von documents in der Reihenfolge, die
// scanDocument erwartet.
const documentColumns = `id, collection, name, content, meta, deleted, revision,
	created_at, created_by, updated_at, updated_by`

// Abfragen der Replica.
const (
	qDocUpsert = `INSERT OR REPLACE INTO documents (` + documentColumns + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	qDocsDeleteOf  = `DELETE FROM documents WHERE collection = ?`
	qDocsDeleteAll = `DELETE FROM documents`
	// qDocLive liest das lebende Dokument eines Namens. Ohne eindeutigen
	// Index könnten es kurzzeitig zwei sein; es gilt das jüngste.
	qDocLive = `SELECT ` + documentColumns + ` FROM documents
		WHERE collection = ? AND name = ? AND deleted = 0
		ORDER BY revision DESC, id DESC LIMIT 1`
	// qDocsLive liest die lebenden Dokumente einer Collection ohne
	// SYSTEM:-Zeilen, nach Name; ein Verzeichnis grenzt der Bereich
	// name >= 'a/' AND name < 'a0' ein ('0' folgt in Bytes auf '/').
	qDocsLive = `SELECT ` + documentColumns + ` FROM documents
		WHERE collection = ? AND deleted = 0 AND substr(name, 1, 7) <> 'SYSTEM:'
		AND name >= ? AND (? = '' OR name < ?)
		ORDER BY name, revision DESC, id DESC`
	qCollectionsAll = `SELECT collection FROM sync_state
		UNION SELECT DISTINCT collection FROM documents ORDER BY 1`

	qStateAll    = `SELECT collection, revision, synced_at FROM sync_state ORDER BY collection`
	qStateGet    = `SELECT COUNT(*) FROM sync_state WHERE collection = ?`
	qStateUpsert = `INSERT INTO sync_state (collection, revision, synced_at) VALUES (?, ?, ?)
		ON CONFLICT(collection) DO UPDATE SET revision = excluded.revision, synced_at = excluded.synced_at`
	qStateDeleteOf  = `DELETE FROM sync_state WHERE collection = ?`
	qStateDeleteAll = `DELETE FROM sync_state`
)

// Replica ist eine geöffnete Replica.
type Replica struct {
	db    *sql.DB
	hubID string
}

// State ist der Stand des Abgleichs einer Collection: bis zu welcher
// Revision die Replica alles hat und wann zuletzt eine Seite ankam (ms seit
// Epoche).
type State struct {
	Collection string
	Revision   int64
	SyncedAt   int64
}

// Document ist ein Dokument der Replica, so wie der Hub es geliefert hat.
type Document = contract.Row

// Open öffnet eine vorhandene Replica und prüft Rolle und Schemafassung.
// Fehlt die Datei, liefert Open sqlitedb.ErrNotFound und legt nichts an.
func Open(ctx context.Context, path string) (*Replica, error) {
	db, err := sqlitedb.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	info, err := sqlitedb.CheckInfo(ctx, db, Role, SchemaVersion)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Replica %s: %w", path, err)
	}
	id := info[KeyHubID]
	if id == "" {
		_ = db.Close()
		return nil, fmt.Errorf("Replica %s: db_info ohne hub_id", path)
	}
	return &Replica{db: db, hubID: id}, nil
}

// Create legt eine neue Replica für den Hub hubID an, samt Verzeichnis
// (0700). Existiert die Datei schon, bricht Create ab. Scheitert das Anlegen,
// bleibt keine Datei zurück.
func Create(ctx context.Context, path, hubID string) (*Replica, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("Verzeichnis der Replicas: %w", err)
	}
	db, err := sqlitedb.Create(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := sqlitedb.CreateSchema(ctx, db, schema, Role, SchemaVersion, map[string]string{KeyHubID: hubID}); err != nil {
		_ = db.Close()
		_ = sqlitedb.Remove(path)
		return nil, err
	}
	return &Replica{db: db, hubID: hubID}, nil
}

// Remove entfernt die Replica-Datei samt -wal und -shm; eine fehlende Datei
// ist kein Fehler.
func Remove(path string) error { return sqlitedb.Remove(path) }

// Close schließt die Replica.
func (r *Replica) Close() error { return r.db.Close() }

// HubID ist die hub_id aus db_info — die maßgebliche.
func (r *Replica) HubID() string { return r.hubID }

// inTx führt fn in einer Transaktion aus.
func (r *Replica) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Lesen: ohne Transaktion — sie wäre IMMEDIATE und nähme die Schreibsperre.

// States liest den Stand aller Collections, nach Name.
func (r *Replica) States(ctx context.Context) ([]State, error) {
	rows, err := r.db.QueryContext(ctx, qStateAll)
	if err != nil {
		return nil, fmt.Errorf("sync_state lesen: %w", err)
	}
	defer rows.Close()
	out := []State{}
	for rows.Next() {
		var s State
		if err := rows.Scan(&s.Collection, &s.Revision, &s.SyncedAt); err != nil {
			return nil, fmt.Errorf("sync_state lesen: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Collections liest alle Collections, von denen die Replica etwas hat: einen
// Stand oder Zeilen.
func (r *Replica) Collections(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, qCollectionsAll)
	if err != nil {
		return nil, fmt.Errorf("Collections der Replica lesen: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Replica) requireCollection(ctx context.Context, collection string) error {
	var n int
	if err := r.db.QueryRowContext(ctx, qStateGet, collection).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("Collection %s %w", collection, ErrNotFound)
	}
	return nil
}

func scanDocument(sc interface{ Scan(...any) error }) (Document, error) {
	var d Document
	var content, meta sql.NullString
	var deleted int64
	if err := sc.Scan(&d.ID, &d.Collection, &d.Name, &content, &meta, &deleted, &d.Revision,
		&d.CreatedAt, &d.CreatedBy, &d.UpdatedAt, &d.UpdatedBy); err != nil {
		return Document{}, err
	}
	if content.Valid {
		d.Content = &content.String
	}
	if meta.Valid {
		d.Meta = &meta.String
	}
	d.Deleted = deleted != 0
	return d, nil
}

// Document liest das lebende Dokument eines Namens: nie eine Löschmarke,
// nie eine SYSTEM:-Zeile. Eine Collection ohne Stand in der Replica ist
// ErrNotFound, ebenso ein fehlendes Dokument.
func (r *Replica) Document(ctx context.Context, collection, name string) (Document, error) {
	if err := ident.CheckDocName(name); err != nil {
		return Document{}, err
	}
	if err := r.requireCollection(ctx, collection); err != nil {
		return Document{}, err
	}
	d, err := scanDocument(r.db.QueryRowContext(ctx, qDocLive, collection, name))
	if errors.Is(err, sql.ErrNoRows) {
		return Document{}, fmt.Errorf("Dokument %s in %s %w", name, collection, ErrNotFound)
	}
	if err != nil {
		return Document{}, fmt.Errorf("Dokument %s lesen: %w", name, err)
	}
	return d, nil
}

// Documents liest die lebenden Dokumente einer Collection unter einem
// Verzeichnis ("" für alle), nach Name: nie Löschmarken, nie SYSTEM:-Zeilen.
// Tragen zwei lebende Zeilen denselben Namen, gilt die jüngste. Eine
// Collection ohne Stand in der Replica ist ErrNotFound.
func (r *Replica) Documents(ctx context.Context, collection, dir string) ([]Document, error) {
	prefix, err := ident.DocDirPrefix(dir)
	if err != nil {
		return nil, err
	}
	if err := r.requireCollection(ctx, collection); err != nil {
		return nil, err
	}
	hi := ""
	if prefix != "" {
		hi = prefix[:len(prefix)-1] + "0"
	}
	rows, err := r.db.QueryContext(ctx, qDocsLive, collection, prefix, hi, hi)
	if err != nil {
		return nil, fmt.Errorf("Dokumente lesen: %w", err)
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("Dokumente lesen: %w", err)
		}
		if n := len(out); n > 0 && out[n-1].Name == d.Name {
			continue
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Schreiben.

// page ist eine Seite, wie der Abgleich sie anwendet.
type page struct {
	// rows sind die Zeilen der Seite.
	rows []contract.Row
	// advance sind die Collections, deren Stand auf max(bisher, until)
	// rückt, mit ihrem bisherigen Stand.
	advance map[string]int64
	until   int64
	// drop sind Collections, die ganz aus der Replica verschwinden.
	drop []string
	now  int64
}

// dropped ist das Ergebnis des Entfernens einer Collection: wie viele
// Zeilen gingen.
type dropped struct {
	rows int64
}

// apply wendet eine Seite in einer Transaktion an: Collections entfernen,
// Zeilen per id einfügen oder ersetzen, Stände fortschreiben — nie zurück.
func (r *Replica) apply(ctx context.Context, p page) (map[string]dropped, error) {
	out := map[string]dropped{}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		for _, c := range p.drop {
			d, err := dropCollection(ctx, tx, c)
			if err != nil {
				return err
			}
			out[c] = d
		}
		for _, row := range p.rows {
			deleted := 0
			if row.Deleted {
				deleted = 1
			}
			if _, err := tx.ExecContext(ctx, qDocUpsert, row.ID, row.Collection, row.Name, row.Content, row.Meta,
				deleted, row.Revision, row.CreatedAt, row.CreatedBy, row.UpdatedAt, row.UpdatedBy); err != nil {
				return fmt.Errorf("Dokument %s in %s schreiben: %w", row.Name, row.Collection, err)
			}
		}
		for c, since := range p.advance {
			if _, err := tx.ExecContext(ctx, qStateUpsert, c, max(since, p.until), p.now); err != nil {
				return fmt.Errorf("sync_state schreiben: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func dropCollection(ctx context.Context, tx *sql.Tx, collection string) (dropped, error) {
	res, err := tx.ExecContext(ctx, qDocsDeleteOf, collection)
	if err != nil {
		return dropped{}, fmt.Errorf("Collection %s entfernen: %w", collection, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return dropped{}, err
	}
	if _, err := tx.ExecContext(ctx, qStateDeleteOf, collection); err != nil {
		return dropped{}, fmt.Errorf("Collection %s entfernen: %w", collection, err)
	}
	return dropped{rows: n}, nil
}

// reset leert die Replica und schreibt die neue hub_id, in einer
// Transaktion.
func (r *Replica) reset(ctx context.Context, hubID string) error {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		for _, del := range []string{qDocsDeleteAll, qStateDeleteAll} {
			if _, err := tx.ExecContext(ctx, del); err != nil {
				return err
			}
		}
		return sqlitedb.SetInfo(ctx, tx, KeyHubID, hubID)
	})
	if err != nil {
		return fmt.Errorf("Replica leeren: %w", err)
	}
	r.hubID = hubID
	return nil
}
