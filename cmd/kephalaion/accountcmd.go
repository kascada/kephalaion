package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/kascada/kephalaion/internal/contract"
	hubstore "github.com/kascada/kephalaion/internal/hub/store"
)

const hubAccountUsage = `Aufruf:
  kephalaion hub account add    <name> [--description text]
  kephalaion hub account list
  kephalaion hub account show   <name>
  kephalaion hub account set    <name> --description text
  kephalaion hub account rm     <name>
  kephalaion hub account lock   <name>
  kephalaion hub account unlock <name>
  kephalaion hub account grant  <name> <collection> [--write] [--supersede]
  kephalaion hub account revoke <name> <collection>
  kephalaion hub account token  <name>

Kommandos:
  add      legt einen Account ohne Collections an und zeigt sein
           Einrichtungstoken — genau einmal
  list     zeigt alle Accounts
  show     zeigt einen Account samt Rechten je Collection
  set      ändert die Beschreibung
  rm       entfernt einen Account; seine Zeilen werden Löschmarken, der Name
           ist danach wieder frei
  lock     sperrt einen Account: seine Zeilen werden Löschmarken, die Rechte
           merkt sich der Hub; unlock legt sie wieder an
  grant    setzt die Rechte in einer Collection vollständig: read immer, dazu
           --write (Eigenes anlegen, ändern, löschen) und --supersede
           (Fremdes ändern, ablösen, löschen); ohne --write wird write
           entzogen, ohne --supersede ebenso supersede
  revoke   nimmt dem Account die Collection
  token    erzeugt ein neues Einrichtungstoken und zeigt es einmal; das alte
           gilt nicht mehr

Das Einrichtungstoken ist das erste Token des Accounts. Sein erster Vorgang
tauscht es gegen ein eigenes: rotate über einen Node, der eine seiner
Collections abgleichen darf (kephalaion node account rotate); danach ist es
wertlos. Gespeichert wird nur der Hash. Account-Namen folgen den
Regeln für Collections, admin ist reserviert, und sie sind gemeinsam mit den
Node-Namen eindeutig. Jede Änderung an den Rechten ist ein Schreibvorgang mit
Revision und gleicht sich zu den Nodes ab; sie steht im Protokoll (actions) als
admin.

Optionen:
  --description text   Kurzbeschreibung
  --write              Recht write (bei grant)
  --supersede          Recht supersede (bei grant)
  --config pfad        Ort der config (siehe kephalaion hub init --help)
`

func runHubAccount(args []string, stdout, stderr io.Writer) int {
	u := hubAccountUsage
	simple := func(name, done string, fn func(ctx context.Context, s hubstore.Store, account string) error) func([]string) int {
		return func(a []string) int {
			c := newCommand("hub account "+name, u, stdout, stderr, "<name>")
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, pos []string) error {
				if err := fn(ctx, s, pos[0]); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Account %s %s.\n", pos[0], done)
				return nil
			})
		}
	}
	return dispatch("hub account", u, args, stdout, stderr, map[string]func([]string) int{
		"add": func(a []string) int {
			c := newCommand("hub account add", u, stdout, stderr, "<name>")
			desc := c.fs.String("description", "", "")
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, pos []string) error {
				token, err := s.AddAccount(ctx, pos[0], *desc)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Account %s angelegt, noch ohne Collections (kephalaion hub account grant).\n", pos[0])
				printAccountToken(stdout, token, pos[0])
				return nil
			})
		},
		"list": func(a []string) int {
			c := newCommand("hub account list", u, stdout, stderr)
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, _ []string) error {
				accounts, err := s.Accounts(ctx)
				if err != nil {
					return err
				}
				if len(accounts) == 0 {
					fmt.Fprintln(stdout, "Keine Accounts.")
					return nil
				}
				tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "NAME\tSTATUS\tRECHTE\tBESCHREIBUNG")
				for _, acc := range accounts {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", acc.Name, lockState(acc.Locked), rightsSummary(acc.Rights), orDash(acc.Description))
				}
				return tw.Flush()
			})
		},
		"show": func(a []string) int {
			c := newCommand("hub account show", u, stdout, stderr, "<name>")
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, pos []string) error {
				acc, err := s.Account(ctx, pos[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Account %s\n", acc.Name)
				fmt.Fprintf(stdout, "  Beschreibung: %s\n", orDash(acc.Description))
				fmt.Fprintf(stdout, "  Status:       %s\n", lockState(acc.Locked))
				label := "Rechte:"
				if acc.Locked {
					label = "Rechte (gemerkt, gelten erst nach unlock):"
				}
				if len(acc.Rights) == 0 {
					fmt.Fprintf(stdout, "  %s keine\n", label)
				} else {
					fmt.Fprintf(stdout, "  %s\n", label)
					for _, r := range acc.Rights {
						fmt.Fprintf(stdout, "    %s: %s\n", r.Collection, r.Rights)
					}
				}
				fmt.Fprintf(stdout, "  Token:        nur als Hash gespeichert\n")
				fmt.Fprintf(stdout, "  angelegt:     %s von %s\n", formatMillis(acc.CreatedAt), acc.CreatedBy)
				return nil
			})
		},
		"set": func(a []string) int {
			c := newCommand("hub account set", u, stdout, stderr, "<name>")
			desc := c.fs.String("description", "", "")
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, pos []string) error {
				if !c.isSet("description") {
					return fmt.Errorf("nichts zu ändern; erwartet --description")
				}
				if err := s.SetAccountDescription(ctx, pos[0], *desc); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Account %s geändert.\n", pos[0])
				return nil
			})
		},
		"rm": simple("rm", "entfernt; seine Zeilen sind Löschmarken", func(ctx context.Context, s hubstore.Store, n string) error {
			return s.RemoveAccount(ctx, n)
		}),
		"lock": simple("lock", "gesperrt; die Rechte sind gemerkt", func(ctx context.Context, s hubstore.Store, n string) error {
			return s.SetAccountLocked(ctx, n, true)
		}),
		"unlock": simple("unlock", "entsperrt", func(ctx context.Context, s hubstore.Store, n string) error {
			return s.SetAccountLocked(ctx, n, false)
		}),
		"grant": func(a []string) int {
			c := newCommand("hub account grant", u, stdout, stderr, "<name>", "<collection>")
			write := c.fs.Bool("write", false, "")
			supersede := c.fs.Bool("supersede", false, "")
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, pos []string) error {
				rights := contract.Rights{Write: *write, Supersede: *supersede}
				changed, err := s.GrantAccount(ctx, pos[0], pos[1], rights)
				if err != nil {
					return err
				}
				if !changed {
					fmt.Fprintf(stdout, "Account %s: %s unverändert (%s).\n", pos[0], pos[1], rights)
					return nil
				}
				fmt.Fprintf(stdout, "Account %s: %s erlaubt (%s).\n", pos[0], pos[1], rights)
				return nil
			})
		},
		"revoke": func(a []string) int {
			c := newCommand("hub account revoke", u, stdout, stderr, "<name>", "<collection>")
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, pos []string) error {
				if err := s.RevokeAccount(ctx, pos[0], pos[1]); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Account %s: %s nicht mehr erlaubt.\n", pos[0], pos[1])
				return nil
			})
		},
		"token": func(a []string) int {
			c := newCommand("hub account token", u, stdout, stderr, "<name>")
			return c.hubDo(a, func(ctx context.Context, s hubstore.Store, pos []string) error {
				token, err := s.NewAccountToken(ctx, pos[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Account %s: neues Einrichtungstoken, das alte gilt nicht mehr.\n", pos[0])
				printAccountToken(stdout, token, pos[0])
				return nil
			})
		},
	})
}

// printAccountToken zeigt ein Einrichtungstoken — das einzige Mal — und wie
// der Account es gegen sein eigenes tauscht.
func printAccountToken(w io.Writer, token, account string) {
	fmt.Fprintln(w, "Einrichtungstoken (wird nicht wieder angezeigt, gespeichert ist nur der Hash):")
	fmt.Fprintf(w, "  %s\n", token)
	fmt.Fprintln(w, "Als Erstes am Node gegen ein eigenes Token tauschen, danach ist es wertlos —")
	fmt.Fprintln(w, "über eine Datei oder stdin, nie als Argument:")
	fmt.Fprintf(w, "  kephalaion node account rotate <hub> %s --token-file <pfad>\n", account)
}

// rightsSummary fasst die Rechte eines Accounts in einer Zeile zusammen:
// „privat (read), team-x (read, write)“.
func rightsSummary(rights []hubstore.AccountRight) string {
	if len(rights) == 0 {
		return "keine"
	}
	parts := make([]string, 0, len(rights))
	for _, r := range rights {
		parts = append(parts, fmt.Sprintf("%s (%s)", r.Collection, r.Rights))
	}
	return strings.Join(parts, ", ")
}
