package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Kind ist die Art einer Datenbank.
type Kind string

// Bekannte Arten. PostgreSQL wird erkannt, aber noch nicht unterstützt.
const (
	SQLite Kind = "sqlite"
)

// ErrUnsupported meldet eine erkannte, aber noch nicht unterstützte db-Adresse.
var ErrUnsupported = errors.New("noch nicht unterstützt")

// DB ist eine zerlegte db-Adresse.
type DB struct {
	Kind Kind
	// Path ist bei SQLite der absolute Pfad der Datenbankdatei.
	Path string
}

// String liefert die db-Adresse, wie sie in der config steht.
func (d DB) String() string {
	return "sqlite://" + d.Path
}

// SQLiteDB liefert die db-Adresse einer SQLite-Datei.
func SQLiteDB(path string) DB {
	return DB{Kind: SQLite, Path: filepath.Clean(path)}
}

// ParseDB zerlegt eine db-Adresse. Erlaubt ist sqlite:// mit absolutem Pfad;
// postgres:// wird erkannt und abgelehnt.
func ParseDB(addr string) (DB, error) {
	switch {
	case strings.HasPrefix(addr, "sqlite://"):
		p := strings.TrimPrefix(addr, "sqlite://")
		if p == "" || !filepath.IsAbs(p) {
			return DB{}, fmt.Errorf("db-Adresse %q: sqlite:// braucht einen absoluten Pfad, etwa sqlite:///home/…/hub.db", addr)
		}
		if strings.ContainsAny(p, "?#") {
			return DB{}, fmt.Errorf("db-Adresse %q: Parameter sind nicht erlaubt", addr)
		}
		if strings.HasSuffix(p, "/") {
			return DB{}, fmt.Errorf("db-Adresse %q: Pfad nennt keine Datei", addr)
		}
		return SQLiteDB(p), nil
	case strings.HasPrefix(addr, "postgres://"), strings.HasPrefix(addr, "postgresql://"):
		return DB{}, fmt.Errorf("db-Adresse %q: PostgreSQL wird %w", addr, ErrUnsupported)
	case addr == "":
		return DB{}, errors.New("db-Adresse fehlt")
	default:
		return DB{}, fmt.Errorf("db-Adresse %q: unbekannte Art, erwartet sqlite:///<absoluter Pfad>", addr)
	}
}
