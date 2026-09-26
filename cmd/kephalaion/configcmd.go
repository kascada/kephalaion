package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/kascada/kephalaion/internal/config"
	hubstore "github.com/kascada/kephalaion/internal/hub/store"
	nodestore "github.com/kascada/kephalaion/internal/node/store"
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
	if _, code, ok := parseFlags(fs, args, configShowUsage, 0, stderr); !ok {
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

const configExportUsage = `Aufruf:
  kephalaion config export [--config pfad] [--output datei]

Schreibt die config und je eingerichteter Rolle die settings und die lokalen
Tabellen als YAML, samt Fassung des Formats — keine Inhalte:
  hub:   collections, nodes (nur der Hash des Tokens), node_collections
  node:  hubs (samt dem eigenen Token beim Hub, im Klartext), hub_collections
Nicht dabei sind db_info — ein neu angelegter Hub bekommt eine neue hub_id —
und das Protokoll actions. Ohne --output auf die Standardausgabe. Die Datei
entsteht mit den Rechten 0600, weil sie Geheimnisse enthält.

Optionen:
  --config pfad     Ort der config (siehe kephalaion hub init --help)
  --output datei    in diese Datei schreiben (ersetzt sie atomar)
`

func runConfigExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config export", configExportUsage, stderr)
	cfgFlag := fs.String("config", "", "")
	output := fs.String("output", "", "")
	if _, code, ok := parseFlags(fs, args, configExportUsage, 0, stderr); !ok {
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
	exp, err := buildExport(context.Background(), cfg)
	if err != nil {
		return fail(err)
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

// buildExport liest settings und lokale Tabellen jeder eingerichteten Rolle.
// Jeder Teil jeder Rolle steht im Export, auch leer.
func buildExport(ctx context.Context, cfg config.Config) (exportFile, error) {
	exp := exportFile{
		Format:   exportFormat,
		Config:   cfg,
		Settings: map[config.Role]map[string]string{},
		Tables:   &exportTables{},
	}
	for _, r := range config.Roles {
		sec := cfg.Section(r)
		if sec == nil {
			continue
		}
		s, err := openSection(ctx, r, sec)
		if err != nil {
			return exportFile{}, fmt.Errorf("Rolle %s: %w", r, err)
		}
		err = func() error {
			defer s.Close()
			settings, err := s.Settings(ctx)
			if err != nil {
				return err
			}
			exp.Settings[r] = settings
			switch st := s.(type) {
			case hubstore.Store:
				t, err := st.Tables(ctx)
				if err != nil {
					return err
				}
				exp.Tables.Hub = hubTablesToYAML(t)
			case nodestore.Store:
				t, err := st.Tables(ctx)
				if err != nil {
					return err
				}
				exp.Tables.Node = nodeTablesToYAML(t)
			}
			return nil
		}()
		if err != nil {
			return exportFile{}, fmt.Errorf("Rolle %s: %w", r, err)
		}
	}
	return exp, nil
}

const configImportUsage = `Aufruf:
  kephalaion config import [--config pfad] <datei>

Schreibt einen Export in die bereits eingerichteten Rollen und ersetzt dort je
Rolle alles in einer Transaktion: die settings und, ab Format 2, die lokalen
Tabellen. Ein Export im Format 1 ersetzt nur die settings und lässt die
Tabellen unberührt. Die config selbst, db_info und das Protokoll bleiben; am
Hub kommt eine Zeile config.import ins Protokoll. Am Node gehen die Replicas der
Hub-Einträge mit, die der Import nicht mehr enthält; sie sind abgeleitet.

Vorab wird alles geprüft, mit denselben Regeln wie beim Anlegen über die
Kommandozeile: Fassung des Formats, jede Rolle des Exports eingerichtet und mit
allen ihren Teilen (ein fehlender Teil oder null bricht ab, nur ein leerer
leert), Namen, Token, Transporte, keine Collection mit Dokumenten fiele weg.
Geschrieben wird erst der Hub, dann der Node.

Optionen:
  --config pfad   Ort der config (siehe kephalaion hub init --help)
`

// nodeImport schreibt den Node-Teil eines Imports. Ein Test ersetzt es, um das
// Scheitern nach dem Hub herbeizuführen.
var nodeImport = func(ctx context.Context, s nodestore.Store, settings map[string]string, t *nodestore.Tables, hubInConfig bool) error {
	return s.Import(ctx, settings, t, hubInConfig)
}

func runConfigImport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config import", configImportUsage, stderr)
	cfgFlag := fs.String("config", "", "")
	pos, code, ok := parseFlags(fs, args, configImportUsage, 1, stderr)
	if !ok {
		return code
	}
	if len(pos) != 1 {
		fmt.Fprint(stderr, "Es fehlt die Exportdatei.\n\n")
		fmt.Fprint(stderr, configImportUsage)
		return 2
	}
	file := pos[0]
	fail := func(err error) int {
		fmt.Fprintf(stderr, "config import: %v\n", err)
		return 1
	}
	nothing := func(err error) int {
		return fail(fmt.Errorf("%w\nNichts geschrieben", err))
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return fail(err)
	}
	exp, err := parseExport(data)
	if err != nil {
		return nothing(fmt.Errorf("%s: %w", file, err))
	}
	roles, err := exp.roles()
	if err != nil {
		return nothing(fmt.Errorf("%s: %w", file, err))
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
		return nothing(errors.New(strings.Join(missing, "\n")))
	}

	// … die Tabellen nach denselben Regeln wie die CLI …
	hubInConfig := cfg.Section(config.Hub) != nil
	var hubTables *hubstore.Tables
	var nodeTables *nodestore.Tables
	if exp.Format >= 2 {
		if exp.Tables.Hub != nil {
			t := exp.Tables.Hub.toStore()
			if err := hubstore.CheckTables(t); err != nil {
				return nothing(fmt.Errorf("Rolle hub: %w", err))
			}
			hubTables = &t
		}
		if exp.Tables.Node != nil {
			t := exp.Tables.Node.toStore()
			if err := nodestore.CheckTables(t, hubInConfig); err != nil {
				return nothing(fmt.Errorf("Rolle node: %w", err))
			}
			nodeTables = &t
		}
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
			return nothing(fmt.Errorf("Rolle %s: %w", r, err))
		}
		stores[r] = s
	}

	// Erst jetzt schreiben: erst den Hub, dann den Node, je in einer
	// Transaktion. Was die Datenbank des Hubs braucht (Dokumente in
	// wegfallenden Collections, Account-Namen), prüft seine Transaktion vor dem
	// ersten Schreiben.
	var done []string
	for _, r := range roles {
		settings := exp.Settings[r]
		var err error
		var summary string
		switch st := stores[r].(type) {
		case hubstore.Store:
			err = st.Import(ctx, settings, hubTables)
			summary = fmt.Sprintf("settings ersetzt (%d Einträge)", len(settings))
			if hubTables != nil {
				summary += fmt.Sprintf(", %d Collections, %d Nodes, %d Rechte",
					len(hubTables.Collections), len(hubTables.Nodes), len(hubTables.Grants))
			}
			summary += "; config.import im Protokoll"
		case nodestore.Store:
			err = nodeImport(ctx, st, settings, nodeTables, hubInConfig)
			summary = fmt.Sprintf("settings ersetzt (%d Einträge)", len(settings))
			if nodeTables != nil {
				summary += fmt.Sprintf(", %d Hubs, %d Collections", len(nodeTables.Hubs), len(nodeTables.Wanted))
			}
		}
		if exp.Format < 2 {
			summary += "; Tabellen unberührt (Format 1)"
		}
		if err != nil {
			if len(done) == 0 {
				return nothing(fmt.Errorf("Rolle %s: %w", r, err))
			}
			lines := append([]string{fmt.Sprintf("Rolle %s: %v", r, err)}, done...)
			for _, rest := range roles[len(done):] {
				lines = append(lines, fmt.Sprintf("%s: nicht geschrieben", rest))
			}
			return fail(errors.New(strings.Join(lines, "\n")))
		}
		done = append(done, fmt.Sprintf("%s: ersetzt — %s", r, summary))
		fmt.Fprintf(stdout, "%s: %s\n", r, summary)
	}
	return 0
}
