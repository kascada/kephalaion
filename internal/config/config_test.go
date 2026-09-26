package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// systemAt lenkt die globale config in ein temporäres Verzeichnis und liefert
// ihren Ort; die Datei gibt es noch nicht.
func systemAt(t *testing.T) string {
	t.Helper()
	old := SystemPath
	SystemPath = filepath.Join(t.TempDir(), "etc", "kephalaion", "config.yaml")
	t.Cleanup(func() { SystemPath = old })
	return SystemPath
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPathOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfig, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	systemAt(t)

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

// Die Suche Stufe für Stufe, mit Quelle: --config > KEPHALAION_CONFIG > config
// des Users, wenn es sie gibt > globale, wenn es sie gibt > Ort des Users.
func TestLocate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfig, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	system := systemAt(t)
	user := filepath.Join(home, ".config", "kephalaion", "config.yaml")

	check := func(name, flagValue, wantPath string, wantSource Source, wantUser, wantSystem bool) Location {
		t.Helper()
		loc, err := Locate(flagValue)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if loc.Path != wantPath || loc.Source != wantSource || loc.UserPath != user ||
			loc.UserExists != wantUser || loc.SystemExists != wantSystem {
			t.Fatalf("%s: %+v, erwartet %s aus %s, User %v, global %v", name, loc, wantPath, wantSource, wantUser, wantSystem)
		}
		return loc
	}

	// Beide fehlen: der Ort des Users, dort legt init an.
	loc := check("beide fehlen", "", user, FromUser, false, false)
	if loc.System() || loc.Explicit() || loc.BothKinds() {
		t.Errorf("beide fehlen: %+v", loc)
	}

	// Nur die globale: sie gilt.
	touch(t, system)
	loc = check("nur global", "", system, FromSystem, false, true)
	if !loc.System() || loc.Explicit() || loc.BothKinds() {
		t.Errorf("nur global: System %v, Explicit %v, BothKinds %v", loc.System(), loc.Explicit(), loc.BothKinds())
	}

	// Beide: die des Users gewinnt, zwei Arten sind erkannt.
	touch(t, user)
	loc = check("beide", "", user, FromUser, true, true)
	if loc.System() || !loc.BothKinds() {
		t.Errorf("beide: System %v, BothKinds %v", loc.System(), loc.BothKinds())
	}

	// Nur die des Users.
	if err := os.Remove(system); err != nil {
		t.Fatal(err)
	}
	check("nur User", "", user, FromUser, true, false)

	// KEPHALAION_CONFIG vor der des Users, auch wenn sie auf die globale zeigt.
	touch(t, system)
	t.Setenv(EnvConfig, system)
	loc = check("Umgebung", "", system, FromEnv, true, true)
	if !loc.System() || !loc.Explicit() {
		t.Errorf("Umgebung: System %v, Explicit %v", loc.System(), loc.Explicit())
	}

	// --config vor allem.
	flagPath := filepath.Join(home, "anders.yaml")
	loc = check("Flag", flagPath, flagPath, FromFlag, true, true)
	if loc.System() || !loc.Explicit() {
		t.Errorf("Flag: System %v, Explicit %v", loc.System(), loc.Explicit())
	}
}

// Ohne Heimatverzeichnis findet die Suche die globale config weiter, und
// --config bzw. KEPHALAION_CONFIG gehen ohne es.
func TestLocateWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv(EnvConfig, "")
	system := systemAt(t)
	if _, err := Locate(""); err == nil {
		t.Fatal("ohne HOME und ohne globale config hätte Locate scheitern sollen")
	}
	touch(t, system)
	loc, err := Locate("")
	if err != nil || loc.Source != FromSystem || loc.UserPath != "" {
		t.Fatalf("globale ohne HOME: %+v, %v", loc, err)
	}
	t.Setenv(EnvConfig, system)
	if loc, err := Locate(""); err != nil || loc.Source != FromEnv {
		t.Fatalf("Umgebung ohne HOME: %+v, %v", loc, err)
	}
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
	if err := AddRole(path, Node, Section{DB: "sqlite:///data/node.db"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddRole(path, Hub, Section{DB: "sqlite:///data/hub.db", Listen: "0.0.0.0:7434"}); err != nil {
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
	if err := AddRole(path, Hub, Section{DB: "sqlite:///anders/hub.db"}); err == nil {
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
		"hub:\n  db: sqlite:///a\n  listen: 7434\n",
		"hub:\n  db: sqlite:///a\n  listen: :7434\n",
		"node:\n  db: sqlite:///a\n  listen: localhost:0\n",
		"node:\n  db: sqlite:///a\n  listen: localhost:http\n",
		"node:\n  db: sqlite:///a\n  listen: localhost:70000\n",
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

func TestListen(t *testing.T) {
	cfg, err := Parse([]byte("hub:\n  db: sqlite:///a/hub.db\nnode:\n  db: sqlite:///a/node.db\n  listen: '[::1]:8000'\n"))
	if err != nil {
		t.Fatal(err)
	}
	// Fehlt listen in einer bestehenden config, gilt der Standard.
	if got := cfg.Listen(Hub); got != DefaultListenHub {
		t.Errorf("Listen(hub) = %q, erwartet %q", got, DefaultListenHub)
	}
	if got := cfg.Listen(Node); got != "[::1]:8000" {
		t.Errorf("Listen(node) = %q", got)
	}
	if DefaultListen(Node) != "127.0.0.1:7433" || DefaultListen(Hub) != "127.0.0.1:7434" {
		t.Error("Standardadressen verändert")
	}
	if got := (&Config{}).Listen(Hub); got != "" {
		t.Errorf("nicht eingerichtete Rolle: %q", got)
	}
	for _, ok := range []string{"127.0.0.1:7433", "0.0.0.0:1", "localhost:65535", "[::1]:7434"} {
		if err := CheckListen(ok); err != nil {
			t.Errorf("CheckListen(%q): %v", ok, err)
		}
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := AddRole(path, Node, Section{DB: "sqlite:///a/node.db", Listen: "nix"}); err == nil {
		t.Fatal("ungültiges listen angenommen")
	}
	if exists(path) {
		t.Fatal("config trotz Fehler geschrieben")
	}
	if err := AddRole(path, Node, Section{DB: "sqlite:///a/node.db", Listen: DefaultListenNode}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "node:\n  db: sqlite:///a/node.db\n  listen: 127.0.0.1:7433\n"; string(data) != want {
		t.Fatalf("config:\n%s\nerwartet:\n%s", data, want)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
