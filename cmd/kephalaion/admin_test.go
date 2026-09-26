package main

import (
	"flag"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kephalaion/kephalaion/internal/ident"
)

func TestParseFlagsAfterPositional(t *testing.T) {
	cases := []struct {
		args []string
		desc string
		pos  []string
	}{
		{[]string{"laptop", "--description", "x"}, "x", []string{"laptop"}},
		{[]string{"--description", "x", "laptop"}, "x", []string{"laptop"}},
		{[]string{"a", "--description=x", "b"}, "x", []string{"a", "b"}},
		{[]string{"a", "--", "--description", "b"}, "", []string{"a", "--description", "b"}},
		{[]string{"a"}, "", []string{"a"}},
		{nil, "", nil},
	}
	for _, c := range cases {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		desc := fs.String("description", "", "")
		pos, _, ok := parseFlags(fs, c.args, "", 3, io.Discard)
		if !ok || *desc != c.desc || !reflect.DeepEqual(pos, c.pos) {
			t.Errorf("%v: ok=%v desc=%q pos=%v", c.args, ok, *desc, pos)
		}
	}
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if _, code, ok := parseFlags(fs, []string{"a", "b"}, "", 1, io.Discard); ok || code != 2 {
		t.Error("zu viele Argumente angenommen")
	}
	if _, code, ok := parseFlags(fs, []string{"a", "--gibtsnicht"}, "", 1, io.Discard); ok || code != 2 {
		t.Error("unbekannte Option nach dem Argument angenommen")
	}
}

// tokenFrom holt das Token aus der Ausgabe von hub node add/token.
func tokenFrom(t *testing.T, out string) string {
	t.Helper()
	for _, f := range strings.Fields(out) {
		if strings.HasPrefix(f, "keph_") && ident.CheckToken(f) == nil {
			return f
		}
	}
	t.Fatalf("kein Token in der Ausgabe:\n%s", out)
	return ""
}

func TestHubFlow(t *testing.T) {
	dir := isolate(t)
	cfg := setup(t, dir)
	c := "--config=" + cfg
	runT(t, "hub", "collection", "add", "team-x", c, "--description", "Team X").want(t, 0, "Collection team-x angelegt.")
	runT(t, "hub", "collection", "add", c, "privat").want(t, 0)
	runT(t, "hub", "collection", "add", "team-x", c).want(t, 1, "gibt es schon")
	runT(t, "hub", "collection", "add", "System", c).want(t, 1, "reserviert")
	runT(t, "hub", "collection", "set", "privat", c, "--description", "Eigenes").want(t, 0, "geändert")
	runT(t, "hub", "collection", "set", "privat", c).want(t, 1, "nichts zu ändern")
	runT(t, "hub", "collection", "list", c).want(t, 0, "team-x", "Team X", "privat", "Eigenes")

	r := runT(t, "hub", "node", "add", "laptop", c, "--description", "Notebook")
	r.want(t, 0, "Node laptop angelegt.", "wird nicht wieder angezeigt", "--token-stdin")
	tok := tokenFrom(t, r.out)
	runT(t, "hub", "node", "set", "laptop", "--description", "Arbeitsrechner", c).want(t, 0, "geändert")
	runT(t, "hub", "node", "grant", "laptop", "team-x", c).want(t, 0, "team-x erlaubt")
	runT(t, "hub", "node", "grant", "laptop", "fehlt", c).want(t, 1, "Collection fehlt gibt es nicht")
	r = runT(t, "hub", "node", "show", "laptop", c)
	r.want(t, 0, "Node laptop", "Arbeitsrechner", "Collections:  team-x", "aktiv", "von admin")
	if strings.Contains(r.out, tok) || strings.Contains(r.out, ident.HashToken(tok)) {
		t.Error("show zeigt Token oder Hash")
	}
	runT(t, "hub", "node", "lock", "laptop", c).want(t, 0, "gesperrt")
	runT(t, "hub", "node", "list", c).want(t, 0, "laptop", "gesperrt", "team-x")
	runT(t, "hub", "node", "unlock", "laptop", c).want(t, 0, "entsperrt")
	r = runT(t, "hub", "node", "token", "laptop", c)
	r.want(t, 0, "das alte gilt nicht mehr", "wird nicht wieder angezeigt", "node hub token <alias> --token-stdin")
	if tok2 := tokenFrom(t, r.out); tok2 == tok {
		t.Error("Token nicht neu")
	}
	runT(t, "hub", "collection", "rm", "team-x", c).want(t, 1, "laptop")
	runT(t, "hub", "node", "revoke", "laptop", "team-x", c).want(t, 0)
	runT(t, "hub", "node", "grant", "laptop", "team-x", c).want(t, 0)
	runT(t, "hub", "node", "rm", "laptop", c).want(t, 0, "entfernt")
	runT(t, "hub", "node", "show", "laptop", c).want(t, 1, "gibt es nicht")
	runT(t, "hub", "collection", "rm", "team-x", c).want(t, 0, "entfernt")
	runT(t, "hub", "node", "list", c).want(t, 0, "Keine Nodes.")
	runT(t, "hub", "node", "grant", "laptop", c).want(t, 2, "Es fehlt: <collection>")
}

func TestNodeFlow(t *testing.T) {
	dir := isolate(t)
	cfg := setup(t, dir)
	c := "--config=" + cfg
	tok, _ := ident.NewToken()

	runIn(t, tok+"\n", "node", "hub", "add", "lokal", "--node", "laptop", "--transport", "local", "--token-stdin", c).
		want(t, 0, "Hub lokal eingetragen (local, als Node laptop)")
	runIn(t, tok+"\n", "node", "hub", "add", "zweit", "--node", "laptop", "--transport", "local", "--token-stdin", c).
		want(t, 1, "Transport local")
	runIn(t, tok, "node", "hub", "add", "test", "--node", "laptop", c, "--transport", "http", "--address", "http://localhost:8080", "--token-stdin").
		want(t, 0, "Hub test eingetragen (http http://localhost:8080, als Node laptop)")
	runIn(t, tok, "node", "hub", "add", "fern", "--node", "laptop", "--transport", "http", "--address", "http://hub.example.org", "--token-stdin", c).
		want(t, 1, "localhost")
	runIn(t, tok, "node", "hub", "add", "ohne", "--node", "laptop", "--transport", "https", "--address", "https://h", c).
		want(t, 1, "--token-stdin")
	for _, a := range []string{"--token", "-token", "--token=" + tok} {
		runT(t, "node", "hub", "add", "arg", "--node", "laptop", "--transport", "local", a, tok, c).
			want(t, 2, "nie als Argument", "--token-stdin")
	}
	runT(t, "node", "hub", "token", "lokal", "--token", tok, c).want(t, 2, "nie als Argument")
	runIn(t, "keph_falsch\n", "node", "hub", "add", "kaputt", "--node", "laptop", "--transport", "https", "--address", "https://h", "--token-stdin", c).
		want(t, 1, "ungültiges Token")
	runIn(t, "", "node", "hub", "add", "leer", "--node", "laptop", "--transport", "https", "--address", "https://h", "--token-stdin", c).
		want(t, 1, "kein Token")

	runT(t, "node", "hub", "set", "test", "--address", "http://127.0.0.1:9090", c).
		want(t, 0, "Hub test geändert (http http://127.0.0.1:9090, als Node laptop)")
	runT(t, "node", "hub", "set", "test", "--transport", "local", c).want(t, 1, "local")
	runT(t, "node", "hub", "set", "test", c).want(t, 1, "nichts zu ändern")
	runIn(t, tok, "node", "hub", "add", "ohne-node", "--transport", "https", "--address", "https://h", "--token-stdin", c).
		want(t, 1, "es fehlt --node")
	runIn(t, tok, "node", "hub", "add", "falsch", "--node", "Laptop", "--transport", "https", "--address", "https://h", "--token-stdin", c).
		want(t, 1, "Node \"Laptop\"")
	runT(t, "node", "hub", "set", "test", "--node", "rechner-2", c).
		want(t, 0, "Hub test geändert (http http://127.0.0.1:9090, als Node rechner-2)")
	runT(t, "node", "hub", "set", "test", "--node", "", c).want(t, 1, "--node")

	runT(t, "node", "collection", "add", "test:team-x", c).want(t, 0, "test:team-x gewünscht")
	runT(t, "node", "collection", "add", "lokal:privat", c).want(t, 0)
	runT(t, "node", "collection", "add", "fehlt:privat", c).want(t, 1, "Hub fehlt gibt es nicht")
	runT(t, "node", "collection", "add", "test:System", c).want(t, 1, "reserviert")
	runT(t, "node", "collection", "add", "ohne-doppelpunkt", c).want(t, 1, "<hub>:<collection>")
	runT(t, "node", "collection", "list", c).want(t, 0, "lokal:privat", "test:team-x")

	r := runT(t, "node", "hub", "list", c)
	r.want(t, 0, "NODE", "lokal", "laptop", "local", "test", "rechner-2", "http://127.0.0.1:9090", "noch kein Kontakt", ident.MaskToken(tok))
	if strings.Contains(r.out, tok) {
		t.Error("list zeigt das Token")
	}
	r = runT(t, "node", "hub", "show", "test", c)
	r.want(t, 0, "Hub test", "Node-Name:    rechner-2", "Collections:  team-x", "noch kein Kontakt")
	if strings.Contains(r.out, tok) {
		t.Error("show zeigt das Token")
	}
	tok2, _ := ident.NewToken()
	runIn(t, tok2+"\n", "node", "hub", "token", "test", "--token-stdin", c).want(t, 0, "Token ersetzt", ident.MaskToken(tok2))

	runT(t, "node", "collection", "rm", "test:team-x", c).want(t, 0, "nicht mehr gewünscht")
	runT(t, "node", "hub", "rm", "lokal", c).want(t, 0, "entfernt")
	runT(t, "node", "collection", "list", c).want(t, 0, "Keine Collections.")
	runT(t, "node", "hub", "list", c).want(t, 0, "test")
}

// local verlangt einen Hub in derselben config; der Hub wird dafür nicht
// geöffnet.
func TestNodeLocalWithoutHub(t *testing.T) {
	dir := isolate(t)
	cfg := filepath.Join(dir, "config.yaml")
	runT(t, "node", "init", "--db", "sqlite://"+filepath.Join(dir, "node.db"), "--config", cfg).want(t, 0)
	tok, _ := ident.NewToken()
	runIn(t, tok, "node", "hub", "add", "lokal", "--node", "laptop", "--transport", "local", "--token-stdin", "--config", cfg).
		want(t, 1, "verlangt einen Hub in derselben config")
	runT(t, "hub", "collection", "list", "--config", cfg).want(t, 1, "kephalaion hub init")
}

func TestAdminUsage(t *testing.T) {
	isolate(t)
	runT(t, "hub", "node").want(t, 2, "kephalaion hub node add")
	runT(t, "hub", "node", "--help").want(t, 0, "grant")
	runT(t, "hub", "collection", "help").want(t, 0, "kephalaion hub collection add")
	runT(t, "node", "hub", "--help").want(t, 0, "--token-stdin")
	runT(t, "node", "collection", "gibtsnicht").want(t, 2, "Unbekanntes Kommando: node collection gibtsnicht")
	runT(t, "hub", "hub").want(t, 2, "Unbekanntes Kommando: hub hub")
	runT(t, "hub", "node", "add").want(t, 2, "Es fehlt: <name>")
	runT(t, "hub", "node", "add", "a", "b").want(t, 2, "Unerwartetes Argument: b")
	runT(t, "help").want(t, 0, "collection, node")
}
