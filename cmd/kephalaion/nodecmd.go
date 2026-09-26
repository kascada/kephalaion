package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/kascada/kephalaion/internal/ident"
	nodestore "github.com/kascada/kephalaion/internal/node/store"
)

const nodeHubUsage = `Aufruf:
  kephalaion node hub add   <alias> --node <name am hub> --transport local|http|https|ssh
                            [--address adresse] [--ssh-key pfad] --token-stdin
  kephalaion node hub list
  kephalaion node hub show  <alias>
  kephalaion node hub set   <alias> [--node …] [--transport …] [--address …] [--ssh-key …]
  kephalaion node hub token <alias> --token-stdin
  kephalaion node hub rm    <alias>

Kommandos:
  add     trägt einen Hub ein; den Alias vergibt der Node, den Node-Namen der
          Hub (kephalaion hub node add)
  list    zeigt alle Hubs
  show    zeigt einen Hub samt gewünschten Collections
  set     ändert Node-Namen, Transport, Adresse oder Schlüssel; der Rest bleibt
  token   ersetzt das Token dieses Nodes beim Hub
  rm      entfernt den Eintrag samt seinen gewünschten Collections und seiner
          Replica

Transporte:
  local   Hub im selben Prozess; verlangt einen Hub in derselben config, keine
          Adresse; höchstens ein Eintrag je Node
  http    nur auf diesem Rechner: http://localhost:<port> (auch 127.0.0.1,
          [::1]) — zum Testen des HTTP-Wegs
  https   https://<host>[:<port>]
  ssh     [user@]host[:port], dazu optional --ssh-key

Wechselt set den Transport, fällt weg, was nicht passt: die Adresse bei local,
der Schlüssel außer bei ssh.

Das Token liest --token-stdin als eine Zeile von der Standardeingabe; als
Argument wird es nie übergeben. Angezeigt wird es nur gekürzt.

Optionen:
  --node name        der Name, unter dem der Hub diesen Node kennt; mit ihm und
                     dem Token meldet sich der Node beim Hub an (Pflicht bei add)
  --transport art    local, http, https oder ssh
  --address adresse  Adresse des Hubs, je nach Transport
  --ssh-key pfad     SSH-Schlüssel, nur bei ssh
  --token-stdin      Token von der Standardeingabe lesen
  --config pfad      Ort der config (siehe kephalaion node init --help)
`

const nodeCollectionUsage = `Aufruf:
  kephalaion node collection add  <hub>:<collection>
  kephalaion node collection list
  kephalaion node collection rm   <hub>:<collection>

Kommandos:
  add    will die Collection von diesem Hub haben; geprüft wird nur, dass es
         den Hub-Eintrag gibt — ob der Hub sie erlaubt, zeigt der Abgleich
  list   zeigt alle gewünschten Collections
  rm     will sie nicht mehr haben

Optionen:
  --config pfad   Ort der config (siehe kephalaion node init --help)
`

func runNodeHub(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	u := nodeHubUsage
	return dispatch("node hub", u, args, stdout, stderr, map[string]func([]string) int{
		"add": func(a []string) int {
			c := newCommand("node hub add", u, stdout, stderr, "<alias>")
			nodeName := c.fs.String("node", "", "")
			transport := c.fs.String("transport", "", "")
			address := c.fs.String("address", "", "")
			sshKey := c.fs.String("ssh-key", "", "")
			tokenStdin := c.fs.Bool("token-stdin", false, "")
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, hubInConfig bool, pos []string) error {
				if *nodeName == "" {
					return errors.New("es fehlt --node: der Name, unter dem der Hub diesen Node kennt")
				}
				if *transport == "" {
					return errors.New("es fehlt --transport (local, http, https oder ssh)")
				}
				if !*tokenStdin {
					return errors.New("es fehlt --token-stdin; das Token wird nie als Argument übergeben")
				}
				h := nodestore.Hub{Name: pos[0], NodeName: *nodeName, Transport: *transport, Address: *address, SSHKey: *sshKey}
				// Erst alles andere prüfen, dann stdin lesen.
				if err := ident.CheckName("Hub", h.Name); err != nil {
					return err
				}
				token, err := readToken(stdin)
				if err != nil {
					return err
				}
				h.Token = token
				if err := s.AddHub(ctx, h, hubInConfig); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Hub %s eingetragen (%s, als Node %s).\n", h.Name, describeTransport(h), h.NodeName)
				return nil
			})
		},
		"list": func(a []string) int {
			c := newCommand("node hub list", u, stdout, stderr)
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, _ bool, _ []string) error {
				hubs, err := s.Hubs(ctx)
				if err != nil {
					return err
				}
				if len(hubs) == 0 {
					fmt.Fprintln(stdout, "Keine Hubs.")
					return nil
				}
				tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ALIAS\tNODE\tTRANSPORT\tADRESSE\tTOKEN\tHUB_ID\tCOLLECTIONS")
				for _, h := range hubs {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", h.Name, h.NodeName, h.Transport, orDash(h.Address),
						ident.MaskToken(h.Token), hubIDOrNone(h.HubID), joinOrNone(h.Collections))
				}
				return tw.Flush()
			})
		},
		"show": func(a []string) int {
			c := newCommand("node hub show", u, stdout, stderr, "<alias>")
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, _ bool, pos []string) error {
				h, err := s.Hub(ctx, pos[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Hub %s\n", h.Name)
				fmt.Fprintf(stdout, "  Node-Name:    %s\n", h.NodeName)
				fmt.Fprintf(stdout, "  Transport:    %s\n", h.Transport)
				fmt.Fprintf(stdout, "  Adresse:      %s\n", orDash(h.Address))
				if h.Transport == nodestore.TransportSSH {
					fmt.Fprintf(stdout, "  SSH-Schlüssel: %s\n", orDash(h.SSHKey))
				}
				fmt.Fprintf(stdout, "  Token:        %s\n", ident.MaskToken(h.Token))
				fmt.Fprintf(stdout, "  hub_id:       %s\n", hubIDOrNone(h.HubID))
				fmt.Fprintf(stdout, "  Collections:  %s\n", joinOrNone(h.Collections))
				return nil
			})
		},
		"set": func(a []string) int {
			c := newCommand("node hub set", u, stdout, stderr, "<alias>")
			nodeName := c.fs.String("node", "", "")
			transport := c.fs.String("transport", "", "")
			address := c.fs.String("address", "", "")
			sshKey := c.fs.String("ssh-key", "", "")
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, hubInConfig bool, pos []string) error {
				var upd nodestore.HubUpdate
				if c.isSet("node") {
					upd.NodeName = nodeName
				}
				if c.isSet("transport") {
					upd.Transport = transport
				}
				if c.isSet("address") {
					upd.Address = address
				}
				if c.isSet("ssh-key") {
					upd.SSHKey = sshKey
				}
				if upd == (nodestore.HubUpdate{}) {
					return errors.New("nichts zu ändern; erwartet --node, --transport, --address oder --ssh-key")
				}
				if err := s.SetHub(ctx, pos[0], upd, hubInConfig); err != nil {
					return err
				}
				h, err := s.Hub(ctx, pos[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Hub %s geändert (%s, als Node %s).\n", h.Name, describeTransport(h), h.NodeName)
				return nil
			})
		},
		"token": func(a []string) int {
			c := newCommand("node hub token", u, stdout, stderr, "<alias>")
			tokenStdin := c.fs.Bool("token-stdin", false, "")
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, _ bool, pos []string) error {
				if !*tokenStdin {
					return errors.New("es fehlt --token-stdin; das Token wird nie als Argument übergeben")
				}
				if _, err := s.Hub(ctx, pos[0]); err != nil {
					return err
				}
				token, err := readToken(stdin)
				if err != nil {
					return err
				}
				if err := s.SetHubToken(ctx, pos[0], token); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Hub %s: Token ersetzt (%s).\n", pos[0], ident.MaskToken(token))
				return nil
			})
		},
		"rm": func(a []string) int {
			c := newCommand("node hub rm", u, stdout, stderr, "<alias>")
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, _ bool, pos []string) error {
				if err := s.RemoveHub(ctx, pos[0]); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Hub %s entfernt.\n", pos[0])
				return nil
			})
		},
	})
}

func runNodeCollection(args []string, stdout, stderr io.Writer) int {
	u := nodeCollectionUsage
	return dispatch("node collection", u, args, stdout, stderr, map[string]func([]string) int{
		"add": func(a []string) int {
			c := newCommand("node collection add", u, stdout, stderr, "<hub>:<collection>")
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, _ bool, pos []string) error {
				hub, coll, err := ident.ParseAddress(pos[0])
				if err != nil {
					return err
				}
				if err := s.AddCollection(ctx, hub, coll); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Collection %s gewünscht.\n", pos[0])
				return nil
			})
		},
		"list": func(a []string) int {
			c := newCommand("node collection list", u, stdout, stderr)
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, _ bool, _ []string) error {
				wanted, err := s.Collections(ctx)
				if err != nil {
					return err
				}
				if len(wanted) == 0 {
					fmt.Fprintln(stdout, "Keine Collections.")
					return nil
				}
				for _, w := range wanted {
					fmt.Fprintln(stdout, ident.Address(w.Hub, w.Collection))
				}
				return nil
			})
		},
		"rm": func(a []string) int {
			c := newCommand("node collection rm", u, stdout, stderr, "<hub>:<collection>")
			return c.nodeDo(a, func(ctx context.Context, s nodestore.Store, _ bool, pos []string) error {
				hub, coll, err := ident.ParseAddress(pos[0])
				if err != nil {
					return err
				}
				if err := s.RemoveCollection(ctx, hub, coll); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "Collection %s nicht mehr gewünscht.\n", pos[0])
				return nil
			})
		},
	})
}

// nodeDo wertet die Argumente aus, öffnet den Node und führt fn aus.
func (c *command) nodeDo(args []string, fn func(ctx context.Context, s nodestore.Store, hubInConfig bool, pos []string) error) int {
	pos, code, ok := c.parse(args)
	if !ok {
		return code
	}
	ctx := context.Background()
	s, hubInConfig, err := c.openNode(ctx)
	if err != nil {
		return c.fail(err)
	}
	defer s.Close()
	if err := fn(ctx, s, hubInConfig, pos); err != nil {
		return c.fail(err)
	}
	return 0
}

func describeTransport(h nodestore.Hub) string {
	if h.Address == "" {
		return h.Transport
	}
	return h.Transport + " " + h.Address
}

func hubIDOrNone(id string) string {
	if id == "" {
		return "noch kein Kontakt"
	}
	return id
}
