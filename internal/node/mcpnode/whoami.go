package mcpnode

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kephalaion/kephalaion/internal/ident"
	"github.com/kephalaion/kephalaion/internal/node/replica"
	"github.com/kephalaion/kephalaion/internal/node/store"
)

// WhoamiOutput ist die Antwort des Werkzeugs whoami (docs/konzept.md,
// „whoami — festgelegt am 2026-09-26“). Nie darin: Token, Hash, Adresse und
// Transport eines Hubs, hub_id.
type WhoamiOutput struct {
	// Version ist die Version des Nodes.
	Version string `json:"version"`
	// Hubs nennt alle Hub-Einträge des Nodes, nach Alias.
	Hubs []HubInfo `json:"hubs"`
	// UnknownHubs sind die Aliase aus Headern, zu denen der Node keinen
	// Eintrag hat — nur der Alias; eine Hilfe bei falsch eingerichteten
	// Clients.
	UnknownHubs []string `json:"unknown_hubs"`
}

// HubInfo ist ein Hub-Eintrag, wie whoami ihn zeigt.
type HubInfo struct {
	Hub string `json:"hub"`
	// Login ist ok, invalid oder missing.
	Login string `json:"login"`
	// Node ist der Name dieses Nodes am Hub.
	Node string   `json:"node"`
	Sync SyncInfo `json:"sync"`
	// Account, User und Collections nur bei login ok.
	Account     string             `json:"account,omitempty"`
	User        string             `json:"user,omitempty"`
	Collections []CollectionRights `json:"collections,omitempty"`
}

// SyncInfo ist der Stand des Abgleichs eines Hub-Eintrags. Zeiten in RFC
// 3339, UTC.
type SyncInfo struct {
	// NeverSynced: Der Node hat noch keine Replica dieses Hubs; dann fehlt
	// revision, und eine Anmeldung ist invalid, auch mit richtigen
	// Zugangsdaten.
	NeverSynced bool `json:"never_synced,omitempty"`
	// LastSuccess ist der letzte gelungene Abgleich.
	LastSuccess string `json:"last_success,omitempty"`
	// Revision ist der Stand, bis zu dem alle Collections der Replica
	// abgeglichen sind.
	Revision *int64 `json:"revision,omitempty"`
	// LastError ist die Art des letzten Fehlers als kurzer Satz, ohne
	// Adresse; leer, wenn der letzte Versuch gelang.
	LastError   string `json:"last_error,omitempty"`
	LastErrorAt string `json:"last_error_at,omitempty"`
}

// CollectionRights ist eine Collection mit Adresse und Rechten.
type CollectionRights struct {
	Collection string   `json:"collection"`
	Address    string   `json:"address"`
	Rights     []string `json:"rights"`
}

// Whoami baut die Antwort von whoami aus den Anmeldungen, samt Textteil —
// eine Zeile je Hub. Dieselbe Funktion dient dem Werkzeug (über
// Authenticate) und der Kommandozeile (node whoami, über AccountLogins).
func Whoami(ctx context.Context, nodes store.Store, version string, logins Logins) (WhoamiOutput, string, error) {
	status, err := nodes.SyncStatus(ctx)
	if err != nil {
		return WhoamiOutput{}, "", err
	}
	hubs, err := nodes.Hubs(ctx)
	if err != nil {
		return WhoamiOutput{}, "", err
	}
	entries := make(map[string]store.Hub, len(hubs))
	for _, h := range hubs {
		entries[h.Name] = h
	}
	out := WhoamiOutput{Version: version, Hubs: make([]HubInfo, 0, len(logins.Hubs)), UnknownHubs: logins.Unknown}
	if out.UnknownHubs == nil {
		out.UnknownHubs = []string{}
	}
	lines := []string{"kephalaion " + version}
	for _, l := range logins.Hubs {
		info := HubInfo{Hub: l.Hub, Login: l.State, Node: l.Node}
		sync, err := syncInfo(ctx, nodes, entries[l.Hub], status[l.Hub])
		if err != nil {
			return WhoamiOutput{}, "", err
		}
		info.Sync = sync
		var who string
		switch l.State {
		case LoginOK:
			info.Account, info.User = l.Account, l.User
			parts := make([]string, 0, len(l.Rights))
			for _, r := range l.Rights {
				addr := ident.Address(l.Hub, r.Collection)
				info.Collections = append(info.Collections, CollectionRights{Collection: r.Collection, Address: addr,
					Rights: strings.Split(r.Rights.String(), ", ")})
				parts = append(parts, fmt.Sprintf("%s (%s)", addr, r.Rights))
			}
			who = fmt.Sprintf("angemeldet als %s (User %s): %s", l.Account, l.User, strings.Join(parts, ", "))
		case LoginInvalid:
			who = "Anmeldung ungültig"
		default:
			who = "keine Zugangsdaten"
		}
		lines = append(lines, fmt.Sprintf("%s (Node %s): %s; %s", l.Hub, l.Node, who, DescribeSync(sync)))
		out.Hubs = append(out.Hubs, info)
	}
	if len(logins.Hubs) == 0 {
		lines = append(lines, "Keine Hubs eingetragen.")
	}
	if len(out.UnknownHubs) > 0 {
		lines = append(lines, "Zugangsdaten für Hubs, die dieser Node nicht kennt: "+strings.Join(out.UnknownHubs, ", "))
	}
	return out, strings.Join(lines, "\n"), nil
}

// syncInfo liest den Stand des Abgleichs eines Eintrags: hub_sync und die
// Revision seiner Replica.
func syncInfo(ctx context.Context, nodes store.Store, h store.Hub, st store.SyncStatus) (SyncInfo, error) {
	var out SyncInfo
	if st.OKAt != 0 {
		out.LastSuccess = formatTime(st.OKAt)
	}
	if st.Err != "" {
		out.LastError, out.LastErrorAt = replica.ErrorKind(st.ErrKind).Text(), formatTime(st.ErrAt)
	}
	rep, err := openReplica(ctx, nodes, h)
	if err != nil {
		return SyncInfo{}, err
	}
	if rep == nil {
		out.NeverSynced = true
		return out, nil
	}
	defer rep.Close()
	rev, err := rep.Revision(ctx)
	if err != nil {
		return SyncInfo{}, err
	}
	out.Revision = &rev
	return out, nil
}

func formatTime(ms int64) string { return time.UnixMilli(ms).UTC().Format(time.RFC3339) }

// DescribeSync ist der Stand des Abgleichs als Text, wie im Textteil von
// whoami.
func DescribeSync(s SyncInfo) string {
	var text string
	switch {
	case s.NeverSynced:
		text = "noch nie abgeglichen"
	case s.LastSuccess != "":
		text = fmt.Sprintf("abgeglichen %s, Revision %d", s.LastSuccess, *s.Revision)
	default:
		text = fmt.Sprintf("Revision %d", *s.Revision)
	}
	if s.LastError != "" {
		text += fmt.Sprintf("; letzter Fehler %s: %s", s.LastErrorAt, s.LastError)
	}
	return text
}

func (n *Node) whoami(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, WhoamiOutput, error) {
	var header http.Header
	if req != nil && req.Extra != nil {
		header = req.Extra.Header
	}
	logins, err := n.Authenticate(ctx, header)
	if err != nil {
		return nil, WhoamiOutput{}, fmt.Errorf("Datenbank des Nodes nicht lesbar")
	}
	out, text, err := Whoami(ctx, n.nodes, n.version, logins)
	if err != nil {
		return nil, WhoamiOutput{}, fmt.Errorf("Datenbank des Nodes nicht lesbar")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}
