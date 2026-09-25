package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kascada/kephalaion/internal/config"
	hubstore "github.com/kascada/kephalaion/internal/hub/store"
	"github.com/kascada/kephalaion/internal/ident"
	nodestore "github.com/kascada/kephalaion/internal/node/store"
)

// dispatch verteilt die Kommandos einer Gruppe wie `hub node` auf ihre
// Blätter.
func dispatch(group, usage string, args []string, stdout, stderr io.Writer, cmds map[string]func(args []string) int) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fn, ok := cmds[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "Unbekanntes Kommando: %s %s\n\n", group, args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
	return fn(args[1:])
}

// command ist ein Blatt-Kommando wie `hub node add`: Optionen samt --config,
// Pflicht-Positionsargumente, Fehlerausgabe.
type command struct {
	name   string
	usage  string
	args   []string
	stdout io.Writer
	stderr io.Writer
	fs     *flag.FlagSet
	cfg    *string
}

// newCommand legt ein Blatt-Kommando an. args nennt die Positionsargumente,
// die es verlangt, für die Meldung, wenn eines fehlt.
func newCommand(name, usage string, stdout, stderr io.Writer, args ...string) *command {
	c := &command{name: name, usage: usage, args: args, stdout: stdout, stderr: stderr}
	c.fs = newFlagSet(name, usage, stderr)
	c.cfg = c.fs.String("config", "", "")
	return c
}

// parse wertet Optionen und Positionsargumente aus.
func (c *command) parse(args []string) (pos []string, code int, ok bool) {
	pos, code, ok = parseFlags(c.fs, args, c.usage, len(c.args), c.stderr)
	if !ok {
		return nil, code, false
	}
	if len(pos) < len(c.args) {
		fmt.Fprintf(c.stderr, "Es fehlt: %s\n\n", c.args[len(pos)])
		fmt.Fprint(c.stderr, c.usage)
		return nil, 2, false
	}
	return pos, 0, true
}

// isSet sagt, ob eine Option angegeben wurde, auch mit leerem Wert.
func (c *command) isSet(name string) bool {
	set := false
	c.fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func (c *command) fail(err error) int {
	fmt.Fprintf(c.stderr, "%s: %v\n", c.name, err)
	return 1
}

func (c *command) loadConfig() (config.Config, error) {
	path, err := config.Path(*c.cfg)
	if err != nil {
		return config.Config{}, err
	}
	cfg, _, err := config.Load(path)
	return cfg, err
}

// openHub öffnet die Datenbank des Hubs aus der config.
func (c *command) openHub(ctx context.Context) (hubstore.Store, error) {
	cfg, err := c.loadConfig()
	if err != nil {
		return nil, err
	}
	sec := cfg.Section(config.Hub)
	if sec == nil {
		return nil, errors.New("der Hub ist nicht eingerichtet; zuerst: kephalaion hub init")
	}
	addr, err := config.ParseDB(sec.DB)
	if err != nil {
		return nil, err
	}
	return hubstore.Open(ctx, addr)
}

// openNode öffnet die Datenbank des Nodes aus der config. hubInConfig sagt,
// ob in derselben config ein Hub eingerichtet ist — die Voraussetzung für
// Transport local. Der Hub selbst wird dafür nicht geöffnet.
func (c *command) openNode(ctx context.Context) (s nodestore.Store, hubInConfig bool, err error) {
	cfg, err := c.loadConfig()
	if err != nil {
		return nil, false, err
	}
	sec := cfg.Section(config.Node)
	if sec == nil {
		return nil, false, errors.New("der Node ist nicht eingerichtet; zuerst: kephalaion node init")
	}
	addr, err := config.ParseDB(sec.DB)
	if err != nil {
		return nil, false, err
	}
	s, err = nodestore.Open(ctx, addr)
	if err != nil {
		return nil, false, err
	}
	return s, cfg.Section(config.Hub) != nil, nil
}

// readToken liest ein Token als eine Zeile von stdin und prüft sein Format.
// Ein Token wird nie als Argument übergeben: Es stünde sonst im
// Shell-Verlauf und in der Prozessliste.
func readToken(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("Token von stdin: %w", err)
	}
	token := strings.TrimSpace(line)
	if token == "" {
		return "", errors.New("kein Token auf stdin (eine Zeile keph_…)")
	}
	if err := ident.CheckToken(token); err != nil {
		return "", err
	}
	return token, nil
}

// orDash liefert „–“ für einen leeren Text.
func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

// joinOrNone liefert die Liste mit Komma oder „keine“.
func joinOrNone(list []string) string {
	if len(list) == 0 {
		return "keine"
	}
	return strings.Join(list, ", ")
}
