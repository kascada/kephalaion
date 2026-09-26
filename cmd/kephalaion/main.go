// Command kephalaion ist das eine Binary des Projekts. Unterkommandos werden
// über das erste Argument gewählt.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
	"github.com/kephalaion/kephalaion/internal/config"
	"github.com/kephalaion/kephalaion/internal/upgrade"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = `kephalaion — geteilte Wissensdatenbank mehrerer Nutzer und Projekte

Aufruf:
  kephalaion <kommando> [optionen]

Kommandos:
  help        zeigt diese Übersicht
  version     zeigt Version, Commit, Go-Version und Plattform
  upgrade     aktualisiert dieses Binary auf das neueste Release
  hub         richtet den Hub ein, pflegt seine Collections, Nodes und
              Accounts und spielt Dokumente ein (init, collection, node,
              account, doc, import)
  node        richtet den Node ein, pflegt seine Hubs und gewünschten
              Collections, tauscht Tokens von Accounts, gleicht ab und zeigt
              Dokumente der Replica (init, hub, collection, account, sync, doc)
  serve       der Dienst: lauscht je eingerichteter Rolle auf ihrem listen
              (Hub: Vertrag für Nodes, Node: MCP für Clients)
  status      zeigt, welche Rollen eingerichtet sind und wo ihre Datenbank liegt
  config      zeigt, sichert und stellt die Einstellungen wieder her
              (show, export, import)

Hilfe zu einem Kommando: kephalaion <kommando> --help

Siehe https://github.com/kephalaion/kephalaion
`

// run verteilt auf die Unterkommandos und liefert den Exit-Code. stdin
// liest nur, wer es ausdrücklich verlangt: --token-stdin, und hub doc put
// ohne --file.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "--version":
		printVersion(stdout)
		return 0
	case "upgrade":
		return runUpgrade(args[1:], stdout, stderr)
	case "hub":
		return runRole(config.Hub, args[1:], stdin, stdout, stderr)
	case "node":
		return runRole(config.Node, args[1:], stdin, stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unbekanntes Kommando: %s\n\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func printVersion(w io.Writer) {
	info := buildinfo.Get()
	commit := info.Commit
	if commit == "" {
		commit = "unbekannt"
	}
	fmt.Fprintf(w, "kephalaion %s\n", info.Version)
	fmt.Fprintf(w, "  Commit:    %s\n", commit)
	fmt.Fprintf(w, "  Go:        %s\n", info.GoVersion)
	fmt.Fprintf(w, "  Plattform: %s\n", info.Platform())
}

const upgradeUsage = `Aufruf:
  kephalaion upgrade [--check] [--version vX.Y.Z]

Ersetzt dieses Binary durch das neueste Release — nach Prüfung gegen
SHA256SUMS und atomar. Stuft nie von selbst zurück.

Optionen:
  --check            nur melden, ob es eine neuere Version gibt
  --version vX.Y.Z   genau diese Version installieren, auch eine ältere oder
                     eine Vorabversion; nötig, um einen dev build zu ersetzen
`

func runUpgrade(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, upgradeUsage) }
	var opts upgrade.Options
	fs.BoolVar(&opts.Check, "check", false, "")
	fs.StringVar(&opts.Version, "version", "", "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "Unerwartetes Argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, upgradeUsage)
		return 2
	}

	// Ein Abbruch mit Strg-C oder SIGTERM beendet den Download, und upgrade
	// räumt die temporäre Datei weg, bevor das Programm endet.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := upgrade.New(stdout).Run(ctx, opts); err != nil {
		fmt.Fprintf(stderr, "upgrade: %v\n", err)
		return 1
	}
	return 0
}
