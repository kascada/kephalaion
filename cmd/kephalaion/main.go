// Command kephalaion ist das eine Binary des Projekts. Unterkommandos werden
// über das erste Argument gewählt.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/kascada/kephalaion/internal/buildinfo"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `kephalaion — geteilte Wissensdatenbank mehrerer Nutzer und Projekte

Aufruf:
  kephalaion <kommando> [optionen]

Kommandos:
  help       zeigt diese Übersicht
  version    zeigt Version, Commit, Go-Version und Plattform

Siehe https://github.com/kascada/kephalaion
`

// run verteilt auf die Unterkommandos und liefert den Exit-Code.
func run(args []string, stdout, stderr io.Writer) int {
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
