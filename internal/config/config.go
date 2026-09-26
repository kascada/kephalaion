// Package config liest und schreibt die config: die kleine Datei, die sagt,
// welche Rollen auf diesem Rechner eingerichtet sind und wo ihre Datenbank
// liegt. Alles andere steht in der Datenbank der Rolle.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// EnvConfig nennt die Umgebungsvariable, die den Ort der config überschreibt.
const EnvConfig = "KEPHALAION_CONFIG"

// Role ist der Name einer Rolle.
type Role string

// Die beiden Rollen.
const (
	Hub  Role = "hub"
	Node Role = "node"
)

// Roles sind alle Rollen in fester Reihenfolge.
var Roles = []Role{Hub, Node}

// Section ist der Abschnitt einer Rolle in der config.
type Section struct {
	// DB ist die db-Adresse, etwa sqlite:///home/…/hub.db.
	DB string `yaml:"db"`
	// Listen ist host:port, wo serve für diese Rolle lauscht. Leer heißt:
	// nicht eingetragen, es gilt DefaultListen; siehe Config.Listen.
	Listen string `yaml:"listen,omitempty"`
}

// Die Standardadressen, auf denen serve lauscht: nur dieser Rechner. Nach
// außen lauscht nur, wer es ausdrücklich einträgt.
const (
	DefaultListenNode = "127.0.0.1:7433"
	DefaultListenHub  = "127.0.0.1:7434"
)

// DefaultListen liefert die Standardadresse einer Rolle.
func DefaultListen(r Role) string {
	if r == Hub {
		return DefaultListenHub
	}
	return DefaultListenNode
}

// CheckListen prüft eine listen-Adresse: host:port mit Host und einem Port
// von 1 bis 65535. Ein leerer Host (":7434") gilt nicht — wer auf allen
// Schnittstellen lauschen will, schreibt es aus (0.0.0.0:7434).
func CheckListen(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return fmt.Errorf("listen %q: erwartet host:port, etwa %s", addr, DefaultListenNode)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("listen %q: Port %q ist keine Zahl von 1 bis 65535", addr, port)
	}
	return nil
}

// Config ist der Inhalt der config. Fehlt ein Abschnitt, fehlt die Rolle.
type Config struct {
	Hub  *Section `yaml:"hub,omitempty"`
	Node *Section `yaml:"node,omitempty"`
}

// Section liefert den Abschnitt einer Rolle oder nil.
func (c *Config) Section(r Role) *Section {
	switch r {
	case Hub:
		return c.Hub
	case Node:
		return c.Node
	}
	return nil
}

// SetSection setzt den Abschnitt einer Rolle.
func (c *Config) SetSection(r Role, s *Section) {
	switch r {
	case Hub:
		c.Hub = s
	case Node:
		c.Node = s
	}
}

// Listen liefert, wo serve für eine eingerichtete Rolle lauscht: den Wert aus
// der config oder, fehlt er dort, den Standard. Ist die Rolle nicht
// eingerichtet, ist das Ergebnis leer.
func (c *Config) Listen(r Role) string {
	s := c.Section(r)
	if s == nil {
		return ""
	}
	if s.Listen == "" {
		return DefaultListen(r)
	}
	return s.Listen
}

// Empty sagt, ob keine Rolle eingerichtet ist.
func (c *Config) Empty() bool {
	return c.Hub == nil && c.Node == nil
}

// SystemConfig ist der feste Ort der globalen config (system installation).
const SystemConfig = "/etc/kephalaion/config.yaml"

// SystemPath ist der Ort, an dem die Suche die globale config erwartet:
// SystemConfig. Tests lenken ihn in ein temporäres Verzeichnis.
var SystemPath = SystemConfig

// Die festen Angaben der globalen Installation (docs/installation.md).
const (
	// SystemUser ist der Systembenutzer, unter dem serve global läuft und als
	// der verwaltet wird.
	SystemUser = "kephalaion"
	// SystemDataDir ist das Verzeichnis der Datenbanken, 0700, gehört
	// SystemUser.
	SystemDataDir = "/var/lib/kephalaion"
	// SystemBinary ist der Ort des Binarys, root, 0755.
	SystemBinary = "/usr/local/bin/kephalaion"
)

// Source sagt, woher der Ort der config kommt.
type Source string

// Die Quellen, in der Reihenfolge der Suche.
const (
	// FromFlag: --config.
	FromFlag Source = "flag"
	// FromEnv: KEPHALAION_CONFIG.
	FromEnv Source = "env"
	// FromUser: die config des Users — weil es sie gibt, oder als Ort, an
	// dem init sie anlegt, wenn es weder sie noch die globale gibt.
	FromUser Source = "user"
	// FromSystem: die globale config, weil es die des Users nicht gibt.
	FromSystem Source = "system"
)

// Location ist das Ergebnis der Suche nach der config.
type Location struct {
	// Path ist die config, die gilt, als absoluter Pfad.
	Path   string
	Source Source
	// UserPath ist der Ort der config des Users; leer, wenn das
	// Heimatverzeichnis unbekannt ist.
	UserPath string
	// UserExists und SystemExists sagen, ob es die config des Users bzw. die
	// globale gibt — unabhängig davon, welche gilt.
	UserExists   bool
	SystemExists bool
}

// System sagt, ob die globale config gilt — über die Suche, --config oder
// KEPHALAION_CONFIG.
func (l Location) System() bool {
	return l.Path == filepath.Clean(SystemPath)
}

// Explicit sagt, ob der Ort ausdrücklich angegeben ist (--config oder
// KEPHALAION_CONFIG).
func (l Location) Explicit() bool {
	return l.Source == FromFlag || l.Source == FromEnv
}

// BothKinds sagt, ob es die config des Users und die globale nebeneinander
// gibt: zwei Arten der Installation auf einem Rechner.
func (l Location) BothKinds() bool {
	return l.UserExists && l.SystemExists
}

// Locate sucht die config: --config > KEPHALAION_CONFIG > die config des
// Users ($XDG_CONFIG_HOME/kephalaion/config.yaml bzw.
// ~/.config/kephalaion/config.yaml), wenn es sie gibt > die globale
// (/etc/kephalaion/config.yaml), wenn es sie gibt > der Ort des Users, an dem
// init sie anlegt. Ob es die beiden Dateien gibt, prüft es in jedem Fall.
func Locate(flagValue string) (Location, error) {
	var loc Location
	userPath, userErr := UserPath()
	if userErr == nil {
		loc.UserPath = userPath
		loc.UserExists = fileExists(userPath)
	}
	loc.SystemExists = fileExists(SystemPath)
	switch {
	case flagValue != "":
		p, err := filepath.Abs(flagValue)
		if err != nil {
			return Location{}, err
		}
		loc.Path, loc.Source = p, FromFlag
	case os.Getenv(EnvConfig) != "":
		p, err := filepath.Abs(os.Getenv(EnvConfig))
		if err != nil {
			return Location{}, err
		}
		loc.Path, loc.Source = p, FromEnv
	case loc.UserExists:
		loc.Path, loc.Source = userPath, FromUser
	case loc.SystemExists:
		loc.Path, loc.Source = filepath.Clean(SystemPath), FromSystem
	case userErr != nil:
		return Location{}, userErr
	default:
		loc.Path, loc.Source = userPath, FromUser
	}
	return loc, nil
}

// fileExists sagt, ob es path gibt. Lässt sich das nicht feststellen, etwa
// mangels Rechten, zählt die Datei als vorhanden: Dann meldet das Lesen den
// Fehler, statt dass still eine andere config gilt.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// Path ermittelt den Ort der config wie Locate.
func Path(flagValue string) (string, error) {
	loc, err := Locate(flagValue)
	if err != nil {
		return "", err
	}
	return loc.Path, nil
}

// UserPath ist der Ort der config des Users:
// $XDG_CONFIG_HOME/kephalaion/config.yaml bzw. ~/.config/kephalaion/config.yaml.
func UserPath() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, "kephalaion", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Heimatverzeichnis unbekannt: %w", err)
	}
	return filepath.Join(home, ".config", "kephalaion", "config.yaml"), nil
}

// DefaultUserPath ist der Standardort der config des Users ohne
// XDG_CONFIG_HOME: ~/.config/kephalaion/config.yaml. Dort findet auch ein
// Dienst sie, dessen Umgebung XDG_CONFIG_HOME nicht kennt.
func DefaultUserPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Heimatverzeichnis unbekannt: %w", err)
	}
	return filepath.Join(home, ".config", "kephalaion", "config.yaml"), nil
}

// DataDir ist das Standardverzeichnis der Datenbanken:
// $XDG_DATA_HOME/kephalaion bzw. ~/.local/share/kephalaion.
func DataDir() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, "kephalaion"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Heimatverzeichnis unbekannt: %w", err)
	}
	return filepath.Join(home, ".local", "share", "kephalaion"), nil
}

// Load liest die config. Fehlt die Datei, ist das kein Fehler: dann ist keine
// Rolle eingerichtet, und exists ist false.
func Load(path string) (cfg Config, exists bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("config %s nicht lesbar: %w", path, err)
	}
	cfg, err = Parse(data)
	if err != nil {
		return Config{}, true, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, true, nil
}

// Parse liest den Inhalt einer config. Unbekannte Schlüssel sind ein Fehler,
// damit beim Zurückschreiben nichts still verloren geht.
func Parse(data []byte) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("ungültiges YAML: %w", err)
	}
	for _, r := range Roles {
		s := cfg.Section(r)
		if s == nil {
			continue
		}
		if s.DB == "" {
			return Config{}, fmt.Errorf("Abschnitt %s: db fehlt", r)
		}
		if s.Listen != "" {
			if err := CheckListen(s.Listen); err != nil {
				return Config{}, fmt.Errorf("Abschnitt %s: %w", r, err)
			}
		}
	}
	return cfg, nil
}

// Marshal liefert die config als YAML.
func Marshal(cfg Config) ([]byte, error) {
	if cfg.Empty() {
		return []byte("{}\n"), nil
	}
	return EncodeYAML(cfg)
}

// EncodeYAML liefert v als YAML, eingerückt mit zwei Leerzeichen wie in der
// Dokumentation.
func EncodeYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Save schreibt die config atomar: erst eine Datei daneben, dann rename. Das
// Verzeichnis wird bei Bedarf mit 0700 angelegt.
func Save(path string, cfg Config) error {
	data, err := Marshal(cfg)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data, 0o644)
}

// AddRole trägt eine Rolle in die config ein. Steht die Rolle schon darin, ist
// das ein Fehler. Die andere Rolle bleibt unberührt.
func AddRole(path string, r Role, s Section) error {
	if s.DB == "" {
		return fmt.Errorf("Abschnitt %s: db fehlt", r)
	}
	if s.Listen != "" {
		if err := CheckListen(s.Listen); err != nil {
			return fmt.Errorf("Abschnitt %s: %w", r, err)
		}
	}
	cfg, _, err := Load(path)
	if err != nil {
		return err
	}
	if cfg.Section(r) != nil {
		return fmt.Errorf("Rolle %s steht schon in der config %s", r, path)
	}
	cfg.SetSection(r, &s)
	return Save(path, cfg)
}

// WriteFileAtomic schreibt data über eine temporäre Datei im selben
// Verzeichnis und rename an path. Das Verzeichnis wird bei Bedarf mit 0700
// angelegt. Scheitert etwas, bleibt path unverändert.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("Verzeichnis %s nicht anlegbar: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("%s nicht schreibbar: %w", path, err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("%s nicht schreibbar: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("%s nicht schreibbar: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%s nicht schreibbar: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("%s nicht schreibbar: %w", path, err)
	}
	ok = true
	return nil
}
