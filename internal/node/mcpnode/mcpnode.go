// Package mcpnode ist der MCP-Eingang des Nodes für Clients: Streamable HTTP
// unter /mcp über das go-sdk, zustandslos. Clients melden sich je Hub mit
// einem Header-Paar an (X-Keph-Account-<alias>, X-Keph-Token-<alias>); der
// Node prüft es bei jeder Anfrage gegen die Account-Zeilen (SYSTEM:A:) der
// Replica dieses Hubs — ohne Cache, die Datenbank ist die einzige Wahrheit.
//
// Werkzeuge: whoami. Transport und initialize gehen ohne Anmeldung; spätere
// Werkzeuge verlangen eine gültige. Kein Token und kein Hash steht je in einer
// Antwort.
//
// Wie jedes Paket unter internal/node kennt es den Hub nicht.
package mcpnode

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/kephalaion/internal/contract"
	"github.com/kascada/kephalaion/internal/ident"
	"github.com/kascada/kephalaion/internal/node/replica"
	"github.com/kascada/kephalaion/internal/node/store"
	"github.com/kascada/kephalaion/internal/reqlog"
	"github.com/kascada/kephalaion/internal/sqlitedb"
)

// Path ist der Pfad des MCP-Eingangs.
const Path = "/mcp"

// Die Präfixe der Header je Hub, klein geschrieben. Der Rest des Namens ist
// der Alias des Hub-Eintrags; siehe HubHeaders.
const (
	accountHeaderPrefix = "x-keph-account-"
	tokenHeaderPrefix   = "x-keph-token-"
)

// dummyHash wird verglichen, wenn es zum Account keine Zeile gibt: So kostet
// ein unbekannter Account dieselbe Arbeit wie ein falsches Token.
var dummyHash = ident.HashToken("keph_unbekannter-account")

// Node ist der MCP-Eingang über der Datenbank des Nodes.
type Node struct {
	nodes store.Store
}

// NewHandler liefert den Handler des Nodes: /mcp mit Prüfung von Host und
// Origin, alles andere 404. version steht in der Antwort auf initialize.
func NewHandler(nodes store.Store, version string) http.Handler {
	n := &Node{nodes: nodes}
	srv := mcp.NewServer(&mcp.Implementation{Name: "kephalaion", Version: version}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name: "whoami",
		Description: "Zeigt je Hub, für den dieser Client Zugangsdaten mitschickt, ob die Anmeldung gilt, " +
			"und wenn ja den Account und seine Collections mit Rechten (read, write, supersede). " +
			"Ohne Argumente.",
	}, n.whoami)
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	mux := http.NewServeMux()
	mux.Handle(Path, guard(h))
	return mux
}

// guard lässt nur Anfragen durch, deren Host dieser Rechner mit dem eigenen
// Port ist und deren Origin fehlt oder lokal ist — sonst 403. So erreicht
// eine Webseite im Browser den Node nicht über DNS-Rebinding. Danach vermerkt
// es die Accounts der Header im Log (nur Namen).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := checkHost(r); err != nil {
			http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
			return
		}
		if err := checkOrigin(r.Header.Values("Origin")); err != nil {
			http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
			return
		}
		for _, p := range HubHeaders(r.Header) {
			reqlog.Note(r.Context(), "account", p.Account)
		}
		next.ServeHTTP(w, r)
	})
}

var localHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}

// checkHost verlangt als Host localhost, 127.0.0.1 oder [::1] mit dem Port,
// auf dem die Anfrage ankam.
func checkHost(r *http.Request) error {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || !localHosts[strings.ToLower(host)] {
		return fmt.Errorf("Host %q ist nicht dieser Rechner", r.Host)
	}
	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || local == nil {
		return errors.New("eigene Adresse unbekannt")
	}
	_, ownPort, err := net.SplitHostPort(local.String())
	if err != nil || ownPort != port {
		return fmt.Errorf("Host %q nennt nicht den Port dieses Nodes", r.Host)
	}
	return nil
}

// checkOrigin lässt eine fehlende Origin zu, sonst nur http://localhost… und
// http://127.0.0.1… (auch [::1]).
func checkOrigin(values []string) error {
	switch len(values) {
	case 0:
		return nil
	case 1:
	default:
		return errors.New("Origin mehrfach")
	}
	u, err := url.Parse(values[0])
	if err != nil || u.Scheme != "http" || !localHosts[strings.ToLower(u.Hostname())] || u.Path != "" || u.User != nil {
		return fmt.Errorf("Origin %q ist nicht dieser Rechner", values[0])
	}
	return nil
}

// HubHeader ist ein Header-Paar für einen Hub: Alias des Hub-Eintrags,
// Account und Token. Complete ist false, wenn einer der beiden Header fehlt
// oder mehrfach steht.
type HubHeader struct {
	Alias    string
	Account  string
	Token    string
	Complete bool
}

// HubHeaders liest die Header-Paare einer Anfrage, nach Alias sortiert.
//
// Zuordnung: Header-Namen sind in HTTP unabhängig von Groß- und
// Kleinschreibung, und Go schreibt sie kanonisch (X-Keph-Account-Team.x_y).
// Der Alias ist deshalb der Rest des Namens nach dem Präfix
// x-keph-account- bzw. x-keph-token-, klein geschrieben. Aliase folgen der
// Namensregel (klein, a–z, 0–9, '.', '_', '-'), das Kleinschreiben ist also
// eindeutig; ein '-' im Alias bleibt, der Präfix ist fest. Namen, die danach
// keinen gültigen Alias ergeben, übergeht der Node.
func HubHeaders(h http.Header) []HubHeader {
	accounts := map[string][]string{}
	tokens := map[string][]string{}
	for key, values := range h {
		lower := strings.ToLower(key)
		if alias, ok := strings.CutPrefix(lower, accountHeaderPrefix); ok {
			accounts[alias] = append(accounts[alias], values...)
		} else if alias, ok := strings.CutPrefix(lower, tokenHeaderPrefix); ok {
			tokens[alias] = append(tokens[alias], values...)
		}
	}
	seen := map[string]bool{}
	var out []HubHeader
	for _, m := range []map[string][]string{accounts, tokens} {
		for alias := range m {
			if seen[alias] || ident.CheckName("Hub", alias) != nil {
				continue
			}
			seen[alias] = true
			p := HubHeader{Alias: alias}
			a, t := accounts[alias], tokens[alias]
			if len(a) == 1 {
				p.Account = strings.TrimSpace(a[0])
			}
			if len(t) == 1 {
				p.Token = strings.TrimSpace(t[0])
			}
			p.Complete = len(a) == 1 && len(t) == 1 && p.Account != "" && p.Token != ""
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out
}

// Login ist das Ergebnis der Prüfung für einen Hub.
type Login struct {
	Hub           string
	Authenticated bool
	Account       string
	// Rights sind die Rechte je Collection, nach Collection; nur wenn
	// Authenticated gilt.
	Rights []Right
}

// Right ist das Recht eines Accounts in einer Collection.
type Right struct {
	Collection string
	contract.Rights
}

// Check prüft ein Header-Paar gegen die Replica seines Hubs: die lebenden
// Zeilen des Accounts über den Index, sha256 des Tokens, Vergleich in
// konstanter Zeit; ohne Zeile gegen einen Ersatz-Hash. Unbekannter Alias,
// unbekannter Account, falsches Token und eine unvollständige Angabe ergeben
// dasselbe: nicht angemeldet. Fehler sind nur Fehler der Datenbank.
func (n *Node) Check(ctx context.Context, p HubHeader) (Login, error) {
	out := Login{Hub: p.Alias}
	hash := ident.HashToken(p.Token)
	rows, err := n.accountRows(ctx, p)
	if err != nil {
		return out, err
	}
	if len(rows) == 0 {
		subtle.ConstantTimeCompare([]byte(hash), []byte(dummyHash))
		return out, nil
	}
	var rights []Right
	for _, row := range rows {
		if row.Content == nil {
			continue
		}
		c, err := contract.DecodeAccountContent(*row.Content)
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(hash), []byte(c.Hash)) == 1 {
			rights = append(rights, Right{Collection: row.Collection, Rights: c.Rights})
		}
	}
	if !p.Complete || len(rights) == 0 {
		return out, nil
	}
	out.Authenticated, out.Account, out.Rights = true, p.Account, rights
	return out, nil
}

// accountRows liest die Zeilen des Accounts aus der Replica des Hubs. Die
// Replica wird je Anfrage geöffnet: Der Abgleich kann sie verwerfen und neu
// anlegen, node hub rm entfernt sie; eine offen gehaltene Datei zeigte dann
// den alten Stand.
func (n *Node) accountRows(ctx context.Context, p HubHeader) ([]replica.Document, error) {
	if !p.Complete || ident.CheckPrincipalName("Account", p.Account) != nil {
		return nil, nil
	}
	if _, err := n.nodes.Hub(ctx, p.Alias); errors.Is(err, store.ErrNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	rep, err := replica.Open(ctx, n.nodes.ReplicaPath(p.Alias))
	if errors.Is(err, sqlitedb.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rep.Close()
	return rep.AccountRows(ctx, p.Account)
}

// WhoamiOutput ist die Antwort des Werkzeugs whoami.
type WhoamiOutput struct {
	// Hubs nennt je Hub mit Header-Paar das Ergebnis, nach Alias.
	Hubs []HubLogin `json:"hubs"`
}

// HubLogin ist die Anmeldung an einem Hub, wie whoami sie zeigt.
type HubLogin struct {
	Hub           string `json:"hub"`
	Authenticated bool   `json:"authenticated"`
	Account       string `json:"account,omitempty"`
	// Collections nur bei gültiger Anmeldung.
	Collections []CollectionRights `json:"collections,omitempty"`
}

// CollectionRights ist eine Collection mit Adresse und Rechten.
type CollectionRights struct {
	Collection string   `json:"collection"`
	Address    string   `json:"address"`
	Rights     []string `json:"rights"`
}

func (n *Node) whoami(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, WhoamiOutput, error) {
	out := WhoamiOutput{Hubs: []HubLogin{}}
	var header http.Header
	if req != nil && req.Extra != nil {
		header = req.Extra.Header
	}
	var lines []string
	for _, p := range HubHeaders(header) {
		login, err := n.Check(ctx, p)
		if err != nil {
			return nil, WhoamiOutput{}, fmt.Errorf("Hub %s: Replica nicht lesbar", p.Alias)
		}
		hl := HubLogin{Hub: login.Hub, Authenticated: login.Authenticated, Account: login.Account}
		if !login.Authenticated {
			lines = append(lines, fmt.Sprintf("%s: nicht angemeldet", login.Hub))
			out.Hubs = append(out.Hubs, hl)
			continue
		}
		var parts []string
		for _, r := range login.Rights {
			rights := strings.Split(r.Rights.String(), ", ")
			hl.Collections = append(hl.Collections, CollectionRights{Collection: r.Collection,
				Address: ident.Address(login.Hub, r.Collection), Rights: rights})
			parts = append(parts, fmt.Sprintf("%s (%s)", ident.Address(login.Hub, r.Collection), r.Rights))
		}
		lines = append(lines, fmt.Sprintf("%s: angemeldet als %s; %s", login.Hub, login.Account, strings.Join(parts, ", ")))
		out.Hubs = append(out.Hubs, hl)
	}
	text := "Keine Zugangsdaten: Der Client schickt für keinen Hub X-Keph-Account-<hub> und X-Keph-Token-<hub>."
	if len(lines) > 0 {
		text = strings.Join(lines, "\n")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}
