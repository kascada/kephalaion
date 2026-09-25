package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/kascada/kephalaion/internal/config"
)

const configUsage = `Aufruf:
  kephalaion config show   [--config pfad]
  kephalaion config export [--config pfad] [--output datei]
  kephalaion config import [--config pfad] <datei>

Kommandos:
  show     zeigt Ort und Inhalt der config und die settings je Rolle
  export   schreibt config und settings je Rolle als YAML — keine Inhalte
  import   schreibt die settings eines Exports in die eingerichteten Rollen
`

// runConfig verteilt die Kommandos unter config.
func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, configUsage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, configUsage)
		return 0
	case "show":
		return runConfigShow(args[1:], stdout, stderr)
	case "export":
		return runConfigExport(args[1:], stdout, stderr)
	case "import":
		return runConfigImport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unbekanntes Kommando: config %s\n\n", args[0])
		fmt.Fprint(stderr, configUsage)
		return 2
	}
}

// readSettings öffnet die Datenbank einer Rolle und liest ihre settings.
func readSettings(ctx context.Context, r config.Role, sec *config.Section) (map[string]string, error) {
	s, err := openSection(ctx, r, sec)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	return s.Settings(ctx)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

const configShowUsage = `Aufruf:
  kephalaion config show [--config pfad]

Zeigt Ort und Inhalt der config, dazu die settings aus der Datenbank jeder
eingerichteten Rolle. Fehlt die Datenbank einer Rolle oder passt sie nicht,
wird die config trotzdem gezeigt und der Fehler dieser Rolle gemeldet; der
Exit-Code ist dann ungleich 0.

Optionen:
  --config pfad   Ort der config (siehe kephalaion hub init --help)
`

func runConfigShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config show", configShowUsage, stderr)
	cfgFlag := fs.String("config", "", "")
	if code, ok := parseFlags(fs, args, configShowUsage, 0, stderr); !ok {
		return code
	}
	cfgPath, err := config.Path(*cfgFlag)
	if err != nil {
		fmt.Fprintf(stderr, "config show: %v\n", err)
		return 1
	}
	cfg, exists, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "config show: %v\n", err)
		return 1
	}
	printConfigLine(stdout, cfgPath, exists)
	if exists {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			fmt.Fprintf(stderr, "config show: %v\n", err)
			return 1
		}
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			fmt.Fprintf(stdout, "  %s\n", line)
		}
	}

	ctx := context.Background()
	failed := false
	for _, r := range config.Roles {
		sec := cfg.Section(r)
		if sec == nil {
			continue
		}
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "settings %s:\n", r)
		settings, err := readSettings(ctx, r, sec)
		if err != nil {
			fmt.Fprintf(stdout, "  Fehler: %v\n", err)
			failed = true
			continue
		}
		if len(settings) == 0 {
			fmt.Fprintln(stdout, "  (keine)")
		}
		for _, k := range sortedKeys(settings) {
			fmt.Fprintf(stdout, "  %s = %s\n", k, settings[k])
		}
	}
	if cfg.Empty() {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Keine Rolle eingerichtet.")
	}
	if failed {
		return 1
	}
	return 0
}

// exportFormat ist die Fassung des Exportformats, die dieses Binary schreibt
// und liest.
const exportFormat = 1

// exportFile ist der Inhalt einer Exportdatei: die config und die settings
// je Rolle, keine Inhalte.
type exportFile struct {
	Format   int                               `yaml:"format"`
	Config   config.Config                     `yaml:"config"`
	Settings map[config.Role]map[string]string `yaml:"settings"`
}

// roles liefert die Rollen eines Exports: die aus der config und die mit
// settings.
func (e *exportFile) roles() ([]config.Role, error) {
	var out []config.Role
	for _, r := range config.Roles {
		_, hasSettings := e.Settings[r]
		if e.Config.Section(r) != nil || hasSettings {
			out = append(out, r)
		}
	}
	for r := range e.Settings {
		if r != config.Hub && r != config.Node {
			return nil, fmt.Errorf("unbekannte Rolle %q", r)
		}
	}
	return out, nil
}

const configExportUsage = `Aufruf:
  kephalaion config export [--config pfad] [--output datei]

Schreibt die config und die settings jeder eingerichteten Rolle als YAML,
samt Fassung des Formats — keine Inhalte. Ohne --output auf die
Standardausgabe. Die Datei entsteht mit den Rechten 0600, weil settings
Geheimnisse enthalten können.

Optionen:
  --config pfad     Ort der config (siehe kephalaion hub init --help)
  --output datei    in diese Datei schreiben (ersetzt sie atomar)
`

func runConfigExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config export", configExportUsage, stderr)
	cfgFlag := fs.String("config", "", "")
	output := fs.String("output", "", "")
	if code, ok := parseFlags(fs, args, configExportUsage, 0, stderr); !ok {
		return code
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "config export: %v\n", err)
		return 1
	}
	cfgPath, err := config.Path(*cfgFlag)
	if err != nil {
		return fail(err)
	}
	cfg, _, err := config.Load(cfgPath)
	if err != nil {
		return fail(err)
	}
	exp := exportFile{Format: exportFormat, Config: cfg, Settings: map[config.Role]map[string]string{}}
	ctx := context.Background()
	for _, r := range config.Roles {
		sec := cfg.Section(r)
		if sec == nil {
			continue
		}
		settings, err := readSettings(ctx, r, sec)
		if err != nil {
			return fail(fmt.Errorf("Rolle %s: %w", r, err))
		}
		exp.Settings[r] = settings
	}
	data, err := config.EncodeYAML(&exp)
	if err != nil {
		return fail(err)
	}
	if *output == "" {
		_, _ = stdout.Write(data)
		return 0
	}
	if err := config.WriteFileAtomic(*output, data, 0o600); err != nil {
		return fail(err)
	}
	fmt.Fprintf(stdout, "Exportiert nach %s\n", *output)
	return 0
}

const configImportUsage = `Aufruf:
  kephalaion config import [--config pfad] <datei>

Schreibt die settings eines Exports in die bereits eingerichteten Rollen und
ersetzt sie dort; die config selbst bleibt unverändert. Vorab wird alles
geprüft: Fassung des Formats, jede Rolle des Exports eingerichtet, Datenbank
vorhanden und passend. Erst dann wird geschrieben, je Rolle in einer
Transaktion.

Optionen:
  --config pfad   Ort der config (siehe kephalaion hub init --help)
`

// parseExport liest eine Exportdatei. Die Fassung wird zuerst geprüft, damit
// ein Export einer anderen Fassung klar abgelehnt wird, statt an unbekannten
// Feldern zu scheitern.
func parseExport(data []byte) (exportFile, error) {
	var head struct {
		Format int `yaml:"format"`
	}
	if err := yaml.Unmarshal(data, &head); err != nil {
		return exportFile{}, fmt.Errorf("kein gültiges YAML: %w", err)
	}
	if head.Format != exportFormat {
		if head.Format == 0 {
			return exportFile{}, errors.New("keine Fassung des Formats angegeben (format:) — kein Export von kephalaion?")
		}
		return exportFile{}, fmt.Errorf("unbekannte Fassung des Formats %d; dieses Binary kennt %d", head.Format, exportFormat)
	}
	var exp exportFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&exp); err != nil {
		return exportFile{}, fmt.Errorf("Export nicht lesbar: %w", err)
	}
	return exp, nil
}

func runConfigImport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config import", configImportUsage, stderr)
	cfgFlag := fs.String("config", "", "")
	if code, ok := parseFlags(fs, args, configImportUsage, 1, stderr); !ok {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprint(stderr, "Es fehlt die Exportdatei.\n\n")
		fmt.Fprint(stderr, configImportUsage)
		return 2
	}
	file := fs.Arg(0)
	fail := func(err error) int {
		fmt.Fprintf(stderr, "config import: %v\n", err)
		return 1
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return fail(err)
	}
	exp, err := parseExport(data)
	if err != nil {
		return fail(fmt.Errorf("%s: %w", file, err))
	}
	roles, err := exp.roles()
	if err != nil {
		return fail(fmt.Errorf("%s: %w", file, err))
	}

	cfgPath, err := config.Path(*cfgFlag)
	if err != nil {
		return fail(err)
	}
	cfg, _, err := config.Load(cfgPath)
	if err != nil {
		return fail(err)
	}

	// Vorab prüfen: jede Rolle eingerichtet …
	var missing []string
	for _, r := range roles {
		if cfg.Section(r) != nil {
			continue
		}
		hint := fmt.Sprintf("kephalaion %s init", r)
		if s := exp.Config.Section(r); s != nil {
			hint += " --db " + s.DB
		}
		missing = append(missing, fmt.Sprintf("die Rolle %s ist nicht eingerichtet; zuerst: %s", r, hint))
	}
	if len(missing) > 0 {
		return fail(fmt.Errorf("%s\nNichts geschrieben", strings.Join(missing, "\n")))
	}

	// … und jede Datenbank öffnet und passt.
	ctx := context.Background()
	stores := map[config.Role]roleStore{}
	defer func() {
		for _, s := range stores {
			_ = s.Close()
		}
	}()
	for _, r := range roles {
		s, err := openSection(ctx, r, cfg.Section(r))
		if err != nil {
			return fail(fmt.Errorf("Rolle %s: %w\nNichts geschrieben", r, err))
		}
		stores[r] = s
	}

	// Erst jetzt schreiben, je Rolle in einer Transaktion.
	for _, r := range roles {
		settings := exp.Settings[r]
		if settings == nil {
			settings = map[string]string{}
		}
		if err := stores[r].ReplaceSettings(ctx, settings); err != nil {
			return fail(fmt.Errorf("Rolle %s: %w", r, err))
		}
		fmt.Fprintf(stdout, "%s: settings ersetzt (%d Einträge)\n", r, len(settings))
	}
	return 0
}
