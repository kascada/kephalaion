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
	"os"
	"path/filepath"

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

// Empty sagt, ob keine Rolle eingerichtet ist.
func (c *Config) Empty() bool {
	return c.Hub == nil && c.Node == nil
}

// Path ermittelt den Ort der config: flag > KEPHALAION_CONFIG >
// $XDG_CONFIG_HOME/kephalaion/config.yaml > ~/.config/kephalaion/config.yaml.
func Path(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	if v := os.Getenv(EnvConfig); v != "" {
		return filepath.Abs(v)
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, "kephalaion", "config.yaml"), nil
	}
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
		if s := cfg.Section(r); s != nil && s.DB == "" {
			return Config{}, fmt.Errorf("Abschnitt %s: db fehlt", r)
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
func AddRole(path string, r Role, db string) error {
	cfg, _, err := Load(path)
	if err != nil {
		return err
	}
	if cfg.Section(r) != nil {
		return fmt.Errorf("Rolle %s steht schon in der config %s", r, path)
	}
	cfg.SetSection(r, &Section{DB: db})
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
