// Package sqlitedb ist der gemeinsame Unterbau der Datenbanken von Hub und
// Node. Er kennt keine der beiden Rollen: SQLite öffnen samt Einstellungen der
// Verbindung, db_info prüfen (Schemafassung, Rolle), settings lesen und
// schreiben.
//
// SQLite-eigen ist nur das Öffnen. Die Abfragen auf db_info und settings
// folgen der Regel für den Hub: Sie laufen auch in PostgreSQL (siehe
// internal/sqlq), denn der Hub benutzt sie mit.
package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/kascada/kephalaion/internal/sqlq"

	_ "modernc.org/sqlite" // Treiber "sqlite", reines Go
)

// ErrNotFound meldet eine fehlende Datenbankdatei.
var ErrNotFound = errors.New("Datenbankdatei fehlt")

// ErrExists meldet, dass eine anzulegende Datenbankdatei schon existiert.
var ErrExists = errors.New("Datenbankdatei existiert schon")

// BaseSchema ist das DDL der Tabellen, die jede Datenbank hat.
const BaseSchema = `
CREATE TABLE db_info (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

// Schlüssel in db_info.
const (
	KeySchemaVersion = "schema_version"
	KeyRole          = "role"
	KeyCreatedAt     = "created_at"
)

// busyTimeout ist die Wartezeit in ms, wenn ein anderer schreibt.
const busyTimeout = 5000

// Queries sind die zentral abgelegten Abfragetexte des Unterbaus, mit
// Platzhaltern $n. Ein Test prüft sie mit sqlq.Check.
var Queries = struct {
	InfoAll        string
	InfoGet        string
	InfoInsert     string
	InfoUpdate     string
	SettingsAll    string
	SettingsDelete string
	SettingsInsert string
}{
	InfoAll:        `SELECT key, value FROM db_info`,
	InfoGet:        `SELECT value FROM db_info WHERE key = $1`,
	InfoInsert:     `INSERT INTO db_info (key, value) VALUES ($1, $2)`,
	InfoUpdate:     `UPDATE db_info SET value = $2 WHERE key = $1`,
	SettingsAll:    `SELECT key, value FROM settings ORDER BY key`,
	SettingsDelete: `DELETE FROM settings`,
	SettingsInsert: `INSERT INTO settings (key, value) VALUES ($1, $2)`,
}

// q übersetzt einen Abfragetext für SQLite.
func q(text string) string { return sqlq.Bind(sqlq.SQLite, text) }

// dsn liefert die Adresse für den Treiber: URI-Form, damit mode=rw gilt und
// eine fehlende Datei nicht angelegt wird, dazu Fremdschlüssel und
// busy_timeout für jede Verbindung — beides ändert die Datei nicht.
// _txlock=immediate lässt jede Transaktion als BEGIN IMMEDIATE beginnen: Sie
// nimmt die Schreibsperre sofort, sodass ein späterer Wechsel vom Lesen zum
// Schreiben nicht an SQLITE_BUSY scheitert. Das gilt auch für Transaktionen,
// die nur lesen; reine Lesezugriffe laufen deshalb ohne Transaktion.
//
// withWAL setzt zusätzlich journal_mode(WAL). Das bleibt in der Datei stehen
// und gehört deshalb nur zu Create, nie zum Öffnen einer vorhandenen Datei.
func dsn(path string, withWAL bool) string {
	u := url.URL{Scheme: "file", Path: path}
	s := u.String() + "?mode=rw" +
		"&_txlock=immediate" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(" + strconv.Itoa(busyTimeout) + ")"
	if withWAL {
		s += "&_pragma=journal_mode(WAL)"
	}
	return s
}

// Open öffnet eine vorhandene Datenbankdatei. Fehlt sie, liefert Open
// ErrNotFound und legt nichts an. Open ändert die Datei nicht, auch nicht
// ihren journal_mode.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	return open(ctx, path, false)
}

func open(ctx context.Context, path string, withWAL bool) (*sql.DB, error) {
	fi, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if err != nil {
		return nil, fmt.Errorf("Datenbankdatei %s: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("Datenbankdatei %s ist keine Datei", path)
	}
	db, err := sql.Open("sqlite", dsn(path, withWAL))
	if err != nil {
		return nil, fmt.Errorf("Datenbank %s: %w", path, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Datenbank %s lässt sich nicht öffnen: %w", path, err)
	}
	return db, nil
}

// Create legt eine neue, leere Datenbankdatei mit 0600 an, stellt sie auf WAL
// und öffnet sie. Existiert die Datei schon, liefert Create ErrExists und ändert nichts.
func Create(ctx context.Context, path string) (*sql.DB, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("%w: %s", ErrExists, path)
	}
	if err != nil {
		return nil, fmt.Errorf("Datenbankdatei %s nicht anlegbar: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = Remove(path)
		return nil, fmt.Errorf("Datenbankdatei %s nicht anlegbar: %w", path, err)
	}
	db, err := open(ctx, path, true)
	if err != nil {
		_ = Remove(path)
		return nil, err
	}
	return db, nil
}

// Remove entfernt eine Datenbankdatei samt -wal, -shm und -journal. Fehlende
// Dateien sind kein Fehler.
func Remove(path string) error {
	var errs []error
	for _, p := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Querier ist, was *sql.DB und *sql.Tx gemeinsam haben.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// NowMillis liefert die aktuelle Zeit in ms seit Epoche, wie sie in der
// Datenbank steht.
func NowMillis() int64 { return time.Now().UnixMilli() }

// InitInfo schreibt die Zeilen von db_info einer frisch angelegten Datenbank:
// Schemafassung, Rolle, Anlagezeit und was extra dazukommt.
func InitInfo(ctx context.Context, tx Querier, role string, schemaVersion int, extra map[string]string) error {
	rows := map[string]string{
		KeySchemaVersion: strconv.Itoa(schemaVersion),
		KeyRole:          role,
		KeyCreatedAt:     strconv.FormatInt(NowMillis(), 10),
	}
	for k, v := range extra {
		rows[k] = v
	}
	for _, k := range sortedKeys(rows) {
		if _, err := tx.ExecContext(ctx, q(Queries.InfoInsert), k, rows[k]); err != nil {
			return fmt.Errorf("db_info schreiben: %w", err)
		}
	}
	return nil
}

// ReadInfo liest alle Zeilen von db_info.
func ReadInfo(ctx context.Context, db Querier) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, q(Queries.InfoAll))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// GetInfo liest eine Zeile von db_info.
func GetInfo(ctx context.Context, db Querier, key string) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, q(Queries.InfoGet), key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("db_info: %s fehlt", key)
	}
	return v, err
}

// SetInfo ändert eine vorhandene Zeile von db_info.
func SetInfo(ctx context.Context, tx Querier, key, value string) error {
	res, err := tx.ExecContext(ctx, q(Queries.InfoUpdate), key, value)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("db_info: %s fehlt", key)
	}
	return nil
}

// WrongRoleError meldet eine Datenbank der anderen Rolle.
type WrongRoleError struct {
	Want, Got string
}

func (e *WrongRoleError) Error() string {
	return fmt.Sprintf("die Datenbank gehört zur Rolle %s, nicht zu %s", e.Got, e.Want)
}

// SchemaVersionError meldet eine Schemafassung, die nicht zu diesem Binary
// passt. Migrationen gibt es noch nicht: Die Datenbank ist neu anzulegen.
type SchemaVersionError struct {
	Role      string
	Want, Got string
}

func (e *SchemaVersionError) Error() string {
	return fmt.Sprintf("Schemafassung %s der Datenbank passt nicht zu diesem Binary (erwartet %s). "+
		"Die Datenbank muss neu angelegt werden (kephalaion %s init); die Inhalte gehen dabei verloren. "+
		"Die Einstellungen (settings und config) lassen sich vorher mit dem bisherigen Binary über "+
		"`kephalaion config export` sichern und danach mit `kephalaion config import` zurückholen.",
		e.Got, e.Want, e.Role)
}

// CheckInfo prüft Rolle und Schemafassung einer geöffneten Datenbank und
// liefert db_info.
func CheckInfo(ctx context.Context, db Querier, role string, schemaVersion int) (map[string]string, error) {
	info, err := ReadInfo(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("keine Kephalaion-Datenbank (db_info nicht lesbar: %w)", err)
	}
	gotRole, ok := info[KeyRole]
	if !ok {
		return nil, errors.New("keine Kephalaion-Datenbank (db_info ohne Rolle)")
	}
	if gotRole != role {
		return nil, &WrongRoleError{Want: role, Got: gotRole}
	}
	want := strconv.Itoa(schemaVersion)
	if got := info[KeySchemaVersion]; got != want {
		return nil, &SchemaVersionError{Role: role, Want: want, Got: got}
	}
	return info, nil
}

// Settings liest alle settings.
func Settings(ctx context.Context, db Querier) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, q(Queries.SettingsAll))
	if err != nil {
		return nil, fmt.Errorf("settings lesen: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("settings lesen: %w", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settings lesen: %w", err)
	}
	return out, nil
}

// ReplaceSettings ersetzt alle settings in einer Transaktion.
func ReplaceSettings(ctx context.Context, db *sql.DB, settings map[string]string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("settings schreiben: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, q(Queries.SettingsDelete)); err != nil {
		return fmt.Errorf("settings schreiben: %w", err)
	}
	for _, k := range sortedKeys(settings) {
		if _, err := tx.ExecContext(ctx, q(Queries.SettingsInsert), k, settings[k]); err != nil {
			return fmt.Errorf("settings schreiben: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("settings schreiben: %w", err)
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CreateSchema legt in einer frisch angelegten Datenbank in einer Transaktion
// BaseSchema, das DDL der Rolle und die Zeilen von db_info an.
func CreateSchema(ctx context.Context, db *sql.DB, roleDDL, role string, schemaVersion int, extra map[string]string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Schema anlegen: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, BaseSchema+roleDDL); err != nil {
		return fmt.Errorf("Schema anlegen: %w", err)
	}
	if err := InitInfo(ctx, tx, role, schemaVersion, extra); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Schema anlegen: %w", err)
	}
	return nil
}
