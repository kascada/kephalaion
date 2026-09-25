package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kascada/kephalaion/internal/config"
	hubstore "github.com/kascada/kephalaion/internal/hub/store"
	nodestore "github.com/kascada/kephalaion/internal/node/store"
	"github.com/kascada/kephalaion/internal/sqlitedb"
)

// roleTitle ist der Name einer Rolle am Satzanfang.
func roleTitle(r config.Role) string {
	switch r {
	case config.Hub:
		return "Hub"
	case config.Node:
		return "Node"
	}
	return string(r)
}

// roleStore ist, was Hub- und Node-Datenbank für die CLI gemeinsam haben.
type roleStore interface {
	Settings(ctx context.Context) (map[string]string, error)
	ReplaceSettings(ctx context.Context, settings map[string]string) error
	Close() error
}

// openRole öffnet die vorhandene Datenbank einer Rolle.
func openRole(ctx context.Context, r config.Role, addr config.DB) (roleStore, error) {
	switch r {
	case config.Hub:
		s, err := hubstore.Open(ctx, addr)
		if err != nil {
			return nil, err
		}
		return s, nil
	case config.Node:
		s, err := nodestore.Open(ctx, addr)
		if err != nil {
			return nil, err
		}
		return s, nil
	}
	return nil, fmt.Errorf("unbekannte Rolle %q", r)
}

// openSection zerlegt die db-Adresse aus der config und öffnet die Datenbank.
func openSection(ctx context.Context, r config.Role, s *config.Section) (roleStore, error) {
	addr, err := config.ParseDB(s.DB)
	if err != nil {
		return nil, err
	}
	return openRole(ctx, r, addr)
}

// createRole legt die Datenbank einer Rolle neu an und liefert ihre
// Schemafassung.
func createRole(ctx context.Context, r config.Role, addr config.DB) (int, error) {
	switch r {
	case config.Hub:
		s, err := hubstore.Create(ctx, addr)
		if err != nil {
			return 0, err
		}
		return hubstore.SchemaVersion, s.Close()
	case config.Node:
		s, err := nodestore.Create(ctx, addr)
		if err != nil {
			return 0, err
		}
		return nodestore.SchemaVersion, s.Close()
	}
	return 0, fmt.Errorf("unbekannte Rolle %q", r)
}

// removeDB entfernt eine frisch angelegte Datenbank wieder.
func removeDB(addr config.DB) error {
	if addr.Kind == config.SQLite {
		return sqlitedb.Remove(addr.Path)
	}
	return nil
}

// newFlagSet liefert ein FlagSet, das bei Fehlern die Hilfe ausgibt.
func newFlagSet(name, usage string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	return fs
}

// parseFlags wertet die Optionen aus, vor und nach Positionsargumenten:
// `hub node add laptop --description …` geht wie `hub node add --description …
// laptop`. Nach `--` ist alles Positionsargument. pos sind die
// Positionsargumente; ok ist false, wenn das Kommando mit code enden soll.
func parseFlags(fs *flag.FlagSet, args []string, usage string, maxArgs int, stderr io.Writer) (pos []string, code int, ok bool) {
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, 0, false
			}
			return nil, 2, false
		}
		next := fs.Args()
		if len(next) == 0 {
			break
		}
		// Hat Parse an `--` aufgehört, ist der Rest Positionsargument.
		if used := len(rest) - len(next); used > 0 && rest[used-1] == "--" {
			pos = append(pos, next...)
			break
		}
		pos = append(pos, next[0])
		rest = next[1:]
	}
	if len(pos) > maxArgs {
		fmt.Fprintf(stderr, "Unerwartetes Argument: %s\n\n", pos[maxArgs])
		fmt.Fprint(stderr, usage)
		return nil, 2, false
	}
	return pos, 0, true
}

const hubUsage = `Aufruf:
  kephalaion hub init [--db sqlite:///pfad/hub.db] [--config pfad]
  kephalaion hub collection add|list|set|rm …
  kephalaion hub node add|list|show|set|rm|lock|unlock|grant|revoke|token …

Kommandos:
  init         richtet den Hub ein: Datenbank, Schema, Abschnitt hub: in der config
  collection   legt die Collections des Hubs an, ändert und entfernt sie
  node         legt Nodes an, erlaubt ihnen Collections, sperrt sie, erneuert
               ihr Token

Hilfe: kephalaion hub collection --help, kephalaion hub node --help
`

const nodeUsage = `Aufruf:
  kephalaion node init [--db sqlite:///pfad/node.db] [--config pfad]
  kephalaion node hub add|list|show|set|rm|token …
  kephalaion node collection add|list|rm …

Kommandos:
  init         richtet den Node ein: Datenbank, Schema, Abschnitt node: in der config
  hub          trägt die Hubs dieses Nodes ein: Transport, Adresse, Token
  collection   die Collections, die der Node von seinen Hubs haben will

Hilfe: kephalaion node hub --help, kephalaion node collection --help
`

// runRole verteilt die Kommandos unter hub bzw. node.
func runRole(r config.Role, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	usage := hubUsage
	if r == config.Node {
		usage = nodeUsage
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch {
	case args[0] == "help" || args[0] == "-h" || args[0] == "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case args[0] == "init":
		return runInit(r, args[1:], stdout, stderr)
	case r == config.Hub && args[0] == "collection":
		return runHubCollection(args[1:], stdout, stderr)
	case r == config.Hub && args[0] == "node":
		return runHubNode(args[1:], stdout, stderr)
	case r == config.Node && args[0] == "hub":
		return runNodeHub(args[1:], stdin, stdout, stderr)
	case r == config.Node && args[0] == "collection":
		return runNodeCollection(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unbekanntes Kommando: %s %s\n\n", r, args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func initUsage(r config.Role) string {
	return fmt.Sprintf(`Aufruf:
  kephalaion %[1]s init [--db sqlite:///pfad/%[1]s.db] [--config pfad]

Richtet die Rolle %[1]s ein: legt die Datenbank samt Schema an und trägt den
Abschnitt %[1]s: in die config ein, die bei Bedarf entsteht. Keine Rückfragen.
Steht die Rolle schon in der config oder gibt es die Datenbankdatei schon,
bricht init ab, statt zu überschreiben.

Optionen:
  --db adresse    Datenbank, sqlite:///<absoluter Pfad>; ohne Angabe
                  $XDG_DATA_HOME/kephalaion/%[1]s.db bzw.
                  ~/.local/share/kephalaion/%[1]s.db
                  (postgres://… ist noch nicht unterstützt)
  --config pfad   Ort der config; sonst $KEPHALAION_CONFIG,
                  $XDG_CONFIG_HOME/kephalaion/config.yaml bzw.
                  ~/.config/kephalaion/config.yaml
`, r)
}

func runInit(r config.Role, args []string, stdout, stderr io.Writer) int {
	usage := initUsage(r)
	fs := newFlagSet(string(r)+" init", usage, stderr)
	dbFlag := fs.String("db", "", "")
	cfgFlag := fs.String("config", "", "")
	if _, code, ok := parseFlags(fs, args, usage, 0, stderr); !ok {
		return code
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s init: "+format+"\n", append([]any{r}, a...)...)
		return 1
	}

	cfgPath, err := config.Path(*cfgFlag)
	if err != nil {
		return fail("%v", err)
	}
	cfg, _, err := config.Load(cfgPath)
	if err != nil {
		return fail("%v", err)
	}
	if s := cfg.Section(r); s != nil {
		return fail("die Rolle %s ist schon eingerichtet (config %s, db %s); init überschreibt nicht", r, cfgPath, s.DB)
	}

	var addr config.DB
	if *dbFlag == "" {
		dir, err := config.DataDir()
		if err != nil {
			return fail("%v", err)
		}
		addr = config.SQLiteDB(filepath.Join(dir, string(r)+".db"))
	} else if addr, err = config.ParseDB(*dbFlag); err != nil {
		return fail("%v", err)
	}
	if err := os.MkdirAll(filepath.Dir(addr.Path), 0o700); err != nil {
		return fail("Verzeichnis nicht anlegbar: %v", err)
	}

	ctx := context.Background()
	version, err := createRole(ctx, r, addr)
	if errors.Is(err, sqlitedb.ErrExists) {
		return fail("die Datenbankdatei %s existiert schon; init überschreibt nicht", addr.Path)
	}
	if err != nil {
		return fail("%v", err)
	}
	// Erst die Datenbank, dann die config. Scheitert die config, darf keine
	// halbe Einrichtung zurückbleiben, die ein zweites init blockiert.
	if err := config.AddRole(cfgPath, r, addr.String()); err != nil {
		if rmErr := removeDB(addr); rmErr != nil {
			return fail("%v; die angelegte Datenbank ließ sich nicht entfernen: %v", err, rmErr)
		}
		return fail("%v; die angelegte Datenbank ist wieder entfernt", err)
	}

	fmt.Fprintf(stdout, "%s eingerichtet.\n", roleTitle(r))
	fmt.Fprintf(stdout, "  Datenbank: %s (Schemafassung %d)\n", addr.Path, version)
	fmt.Fprintf(stdout, "  config:    %s (Abschnitt %s:)\n", cfgPath, r)
	return 0
}

const statusUsage = `Aufruf:
  kephalaion status [--config pfad]

Zeigt, welche Rollen auf diesem Rechner eingerichtet sind, wo ihre Datenbank
liegt und ihre Kennzahlen. Öffnet die Datenbanken nur, legt nichts an. Der
Exit-Code ist nur dann ungleich 0, wenn die Datenbank einer eingerichteten
Rolle fehlt oder nicht passt.

Optionen:
  --config pfad   Ort der config (siehe kephalaion hub init --help)
`

func runStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("status", statusUsage, stderr)
	cfgFlag := fs.String("config", "", "")
	if _, code, ok := parseFlags(fs, args, statusUsage, 0, stderr); !ok {
		return code
	}
	cfgPath, err := config.Path(*cfgFlag)
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return 1
	}
	cfg, exists, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return 1
	}
	printConfigLine(stdout, cfgPath, exists)

	ctx := context.Background()
	failed := false
	for _, r := range config.Roles {
		fmt.Fprintln(stdout)
		sec := cfg.Section(r)
		if sec == nil {
			fmt.Fprintf(stdout, "%s: nicht eingerichtet\n", r)
			continue
		}
		fmt.Fprintf(stdout, "%s: eingerichtet\n", r)
		fmt.Fprintf(stdout, "  db:            %s\n", sec.DB)
		if err := printRoleStatus(ctx, stdout, r, sec); err != nil {
			fmt.Fprintf(stdout, "  Fehler:        %v\n", err)
			failed = true
		}
	}
	if cfg.Empty() {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Keine Rolle eingerichtet. Einrichten mit:")
		fmt.Fprintln(stdout, "  kephalaion hub init    für den Hub")
		fmt.Fprintln(stdout, "  kephalaion node init   für den Node")
	}
	if failed {
		return 1
	}
	return 0
}

func printConfigLine(w io.Writer, path string, exists bool) {
	state := "vorhanden"
	if !exists {
		state = "fehlt"
	}
	fmt.Fprintf(w, "config: %s (%s)\n", path, state)
}

// printRoleStatus öffnet die Datenbank einer Rolle und gibt ihre Kennzahlen
// aus.
func printRoleStatus(ctx context.Context, w io.Writer, r config.Role, sec *config.Section) error {
	s, err := openSection(ctx, r, sec)
	if err != nil {
		return err
	}
	defer s.Close()
	switch st := s.(type) {
	case hubstore.Store:
		return printHubStatus(ctx, w, st)
	case nodestore.Store:
		return printNodeStatus(ctx, w, st)
	}
	return nil
}

func printHubStatus(ctx context.Context, w io.Writer, st hubstore.Store) error {
	info, err := st.Info(ctx)
	if err != nil {
		return err
	}
	stats, err := st.Stats(ctx)
	if err != nil {
		return err
	}
	colls, err := st.Collections(ctx)
	if err != nil {
		return err
	}
	nodes, err := st.Nodes(ctx)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(colls))
	for _, c := range colls {
		names = append(names, c.Name)
	}
	fmt.Fprintf(w, "  Schemafassung: %d\n", info.SchemaVersion)
	fmt.Fprintf(w, "  hub_id:        %s\n", info.HubID)
	fmt.Fprintf(w, "  Revision:      %d\n", info.Revision)
	fmt.Fprintf(w, "  Dokumente:     %d\n", stats.Documents)
	fmt.Fprintf(w, "  Collections:   %s\n", joinOrNone(names))
	if len(nodes) == 0 {
		fmt.Fprintf(w, "  Nodes:         keine\n")
		return nil
	}
	fmt.Fprintf(w, "  Nodes:\n")
	for _, n := range nodes {
		fmt.Fprintf(w, "    %s: %s, erlaubt: %s\n", n.Name, lockState(n.Locked), joinOrNone(n.Collections))
	}
	return nil
}

func printNodeStatus(ctx context.Context, w io.Writer, st nodestore.Store) error {
	info, err := st.Info(ctx)
	if err != nil {
		return err
	}
	hubs, err := st.Hubs(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "  Schemafassung: %d\n", info.SchemaVersion)
	if len(hubs) == 0 {
		fmt.Fprintf(w, "  Hubs:          keine\n")
		return nil
	}
	fmt.Fprintf(w, "  Hubs:\n")
	for _, h := range hubs {
		fmt.Fprintf(w, "    %s: %s, hub_id: %s\n", h.Name, describeTransport(h), hubIDOrNone(h.HubID))
		fmt.Fprintf(w, "      Collections: %s\n", joinOrNone(h.Collections))
	}
	return nil
}
