package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPathOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfig, "")
	t.Setenv("XDG_CONFIG_HOME", "")

	check := func(flagValue, want string) {
		t.Helper()
		got, err := Path(flagValue)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Path(%q) = %q, erwartet %q", flagValue, got, want)
		}
	}

	check("", filepath.Join(home, ".config", "kephalaion", "config.yaml"))

	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	check("", filepath.Join(xdg, "kephalaion", "config.yaml"))

	// Ein relativer XDG_CONFIG_HOME gilt nach der Spezifikation nicht.
	t.Setenv("XDG_CONFIG_HOME", "relativ")
	check("", filepath.Join(home, ".config", "kephalaion", "config.yaml"))
	t.Setenv("XDG_CONFIG_HOME", xdg)

	env := filepath.Join(home, "env.yaml")
	t.Setenv(EnvConfig, env)
	check("", env)

	flagPath := filepath.Join(home, "flag.yaml")
	check(flagPath, flagPath)
}

func TestDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	got, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", "kephalaion"); got != want {
		t.Errorf("DataDir() = %q, erwartet %q", got, want)
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	got, _ = DataDir()
	if want := filepath.Join(home, "data", "kephalaion"); got != want {
		t.Errorf("DataDir() = %q, erwartet %q", got, want)
	}
}

func TestLoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg, exists, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if exists || !cfg.Empty() {
		t.Fatalf("fehlende Datei: exists=%v, cfg=%+v", exists, cfg)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")
	want := Config{
		Hub:  &Section{DB: "sqlite:///data/hub.db"},
		Node: &Section{DB: "sqlite:///data/node.db"},
	}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !exists || got.Hub == nil || got.Node == nil || *got.Hub != *want.Hub || *got.Node != *want.Node {
		t.Fatalf("Hin und zurück: %+v", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporäre Datei liegen geblieben: %v", entries)
	}
}

func TestAddRoleKeepsOther(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := AddRole(path, Node, "sqlite:///data/node.db"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddRole(path, Hub, "sqlite:///data/hub.db"); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Node == nil || cfg.Node.DB != "sqlite:///data/node.db" {
		t.Fatalf("Node verändert: %+v (vorher %s)", cfg.Node, before)
	}
	if cfg.Hub == nil || cfg.Hub.DB != "sqlite:///data/hub.db" {
		t.Fatalf("Hub fehlt: %+v", cfg.Hub)
	}
	if err := AddRole(path, Hub, "sqlite:///anders/hub.db"); err == nil {
		t.Fatal("zweites Eintragen derselben Rolle hätte scheitern sollen")
	}
	cfg, _, _ = Load(path)
	if cfg.Hub.DB != "sqlite:///data/hub.db" {
		t.Fatalf("Hub überschrieben: %+v", cfg.Hub)
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{
		"hub:\n  db: sqlite:///a\n  extra: 1\n",
		"unbekannt: {}\n",
		"hub: {}\n",
		"hub: [\n",
	} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) hätte scheitern sollen", in)
		}
	}
	cfg, err := Parse(nil)
	if err != nil || !cfg.Empty() {
		t.Errorf("leere Datei: %+v, %v", cfg, err)
	}
}

func TestParseDB(t *testing.T) {
	db, err := ParseDB("sqlite:///home/x/hub.db")
	if err != nil {
		t.Fatal(err)
	}
	if db.Kind != SQLite || db.Path != "/home/x/hub.db" || db.String() != "sqlite:///home/x/hub.db" {
		t.Fatalf("ParseDB: %+v", db)
	}

	if _, err := ParseDB("postgres://keph@db/kephalaion"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("postgres://: %v, erwartet ErrUnsupported", err)
	}
	if _, err := ParseDB("postgresql://keph@db/kephalaion"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("postgresql://: %v, erwartet ErrUnsupported", err)
	}
	for _, bad := range []string{
		"",
		"sqlite://",
		"sqlite://relativ/hub.db",
		"sqlite:///a/hub.db?mode=rwc",
		"sqlite:///a/",
		"/home/x/hub.db",
		"mysql://x",
	} {
		_, err := ParseDB(bad)
		if err == nil {
			t.Errorf("ParseDB(%q) hätte scheitern sollen", bad)
		} else if errors.Is(err, ErrUnsupported) {
			t.Errorf("ParseDB(%q): ungültig, nicht nur nicht unterstützt: %v", bad, err)
		}
	}
}
