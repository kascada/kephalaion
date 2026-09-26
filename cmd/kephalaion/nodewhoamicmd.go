package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
	"github.com/kephalaion/kephalaion/internal/ident"
	"github.com/kephalaion/kephalaion/internal/node/mcpnode"
	nodestore "github.com/kephalaion/kephalaion/internal/node/store"
)

const nodeWhoamiUsage = `Aufruf:
  kephalaion node whoami [--hub <alias>]
  kephalaion node whoami <account> [--hub <alias>] [--json]

Ohne Account: die Version, je Hub der Name dieses Nodes am Hub und der
Stand des Abgleichs, dazu die Accounts, die der Node aus seinen Replicas
kennt (lebende Zeilen SYSTEM:A:), mit User, Collections und Rechten. Ein
gesperrter Account fehlt, sobald der Abgleich die Sperre gebracht hat.

Mit Account: was das MCP-Werkzeug whoami einem Client antworten würde, der
für diesen Account an jedem Hub gültige Zugangsdaten schickt — login ok, wo
der Account in der Replica steht, sonst missing. Aus derselben Funktion wie
das Werkzeug.

Ohne Token: Wer die Kommandozeile aufruft, kann die Datenbanken des Nodes
ohnehin lesen. Ob ein Token gilt, prüft kephalaion node account check.

Optionen:
  --hub alias     nur diesen Hub-Eintrag
  --json          die Antwort als JSON, in der Struktur des Werkzeugs (nur
                  mit <account>)
  --config pfad   Ort der config (siehe kephalaion node init --help)
`

func runNodeWhoami(args []string, stdout, stderr io.Writer) int {
	c := newCommand("node whoami", nodeWhoamiUsage, stdout, stderr)
	c.optional = 1
	hub := c.fs.String("hub", "", "")
	asJSON := c.fs.Bool("json", false, "")
	return c.nodeDo(args, func(ctx context.Context, s nodestore.Store, _ bool, pos []string) error {
		if *hub != "" {
			if _, err := s.Hub(ctx, *hub); err != nil {
				return err
			}
		}
		version := buildinfo.Get().Version
		if len(pos) == 0 {
			if *asJSON {
				return errors.New("--json gibt es nur mit <account>: die Struktur ist die des Werkzeugs whoami")
			}
			return printNodeOverview(ctx, stdout, s, version, *hub)
		}
		account := pos[0]
		if err := ident.CheckPrincipalName("Account", account); err != nil {
			return err
		}
		logins, err := mcpnode.AccountLogins(ctx, s, account)
		if err != nil {
			return err
		}
		logins.Hubs = onlyHub(logins.Hubs, *hub)
		out, text, err := mcpnode.Whoami(ctx, s, version, logins)
		if err != nil {
			return err
		}
		if *asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}
		fmt.Fprintln(stdout, text)
		return nil
	})
}

// onlyHub grenzt die Anmeldungen auf einen Alias ein; leer lässt alle.
func onlyHub(logins []mcpnode.Login, alias string) []mcpnode.Login {
	if alias == "" {
		return logins
	}
	var out []mcpnode.Login
	for _, l := range logins {
		if l.Hub == alias {
			out = append(out, l)
		}
	}
	return out
}

// printNodeOverview zeigt Version, je Hub Node-Name und Stand des Abgleichs
// — aus Whoami, ohne Anmeldung — und die bekannten Accounts.
func printNodeOverview(ctx context.Context, w io.Writer, s nodestore.Store, version, alias string) error {
	hubs, err := s.Hubs(ctx)
	if err != nil {
		return err
	}
	var none mcpnode.Logins
	for _, h := range hubs {
		none.Hubs = append(none.Hubs, mcpnode.Login{Hub: h.Name, Node: h.NodeName, State: mcpnode.LoginMissing})
	}
	none.Hubs = onlyHub(none.Hubs, alias)
	out, _, err := mcpnode.Whoami(ctx, s, version, none)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "kephalaion %s\n", out.Version)
	if len(out.Hubs) == 0 {
		fmt.Fprintln(w, "Keine Hubs.")
		return nil
	}
	fmt.Fprintln(w, "Hubs:")
	for _, h := range out.Hubs {
		fmt.Fprintf(w, "  %s (Node %s): %s\n", h.Hub, h.Node, mcpnode.DescribeSync(h.Sync))
	}
	accounts, err := mcpnode.KnownAccounts(ctx, s)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	n := 0
	for _, a := range accounts {
		if alias != "" && a.Hub != alias {
			continue
		}
		if n == 0 {
			fmt.Fprintln(w, "Accounts:")
			fmt.Fprintln(tw, "  HUB\tACCOUNT\tUSER\tCOLLECTIONS")
		}
		n++
		parts := make([]string, 0, len(a.Rights))
		for _, r := range a.Rights {
			parts = append(parts, fmt.Sprintf("%s (%s)", ident.Address(a.Hub, r.Collection), r.Rights))
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", a.Hub, a.Account, a.User, strings.Join(parts, ", "))
	}
	if n == 0 {
		fmt.Fprintln(w, "Accounts: keine bekannt (erst nach rotate oder Abgleich)")
		return nil
	}
	return tw.Flush()
}
