// Package sqlq ist die kleine Hilfe für Abfragen, die in SQLite und
// PostgreSQL gleichermaßen laufen sollen: Abfragetexte werden mit
// nummerierten Platzhaltern $1, $2, … geschrieben und je Dialekt übersetzt.
// Check prüft einen Text auf Konstrukte, die nur einer der beiden versteht.
package sqlq

import (
	"fmt"
	"reflect"
	"strings"
)

// Dialect ist die Sprache einer Datenbank.
type Dialect int

// Bekannte Dialekte.
const (
	SQLite Dialect = iota
	Postgres
)

// Bind übersetzt die Platzhalter eines Abfragetexts in den Dialekt: für
// PostgreSQL bleibt $n, für SQLite wird daraus ?n. Text in einfachen
// Anführungszeichen bleibt unberührt.
func Bind(d Dialect, query string) string {
	if d == Postgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query))
	inLiteral := false
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case c == '\'':
			inLiteral = !inLiteral
		case !inLiteral && c == '$' && i+1 < len(query) && isDigit(query[i+1]):
			c = '?'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// forbidden sind Konstrukte, die nicht in beiden Dialekten gelten.
var forbidden = []string{
	"INSERT OR",     // SQLite-eigen; PostgreSQL kennt ON CONFLICT
	"REPLACE INTO",  // SQLite/MySQL-eigen
	"PRAGMA",        // SQLite-eigen; nur beim Öffnen der Verbindung erlaubt
	"AUTOINCREMENT", // SQLite-eigen; Revisionen zählt der Code
	"SQLITE_",       // Systemtabellen von SQLite
	"LAST_INSERT_ROWID",
}

// Check meldet, ob ein Abfragetext Konstrukte enthält, die nur einer der
// beiden Dialekte versteht, oder rohe ?-Platzhalter an Bind vorbei.
func Check(query string) error {
	upper := strings.ToUpper(strings.Join(strings.Fields(query), " "))
	for _, f := range forbidden {
		if strings.Contains(upper, f) {
			return fmt.Errorf("enthält %s: %s", f, query)
		}
	}
	if strings.Contains(query, "?") {
		return fmt.Errorf("enthält einen rohen ?-Platzhalter, erwartet $n: %s", query)
	}
	return nil
}

// Texts liefert alle Felder vom Typ string einer Struktur. So prüft ein Test
// die zentral abgelegten Abfragetexte eines Pakets vollständig, ohne dass
// eine Liste von Hand gepflegt wird.
func Texts(v any) map[string]string {
	out := map[string]string{}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Type.Kind() == reflect.String {
			out[rt.Field(i).Name] = rv.Field(i).String()
		}
	}
	return out
}
