package mcpnode

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/kephalaion/kephalaion/internal/contract"
	"github.com/kephalaion/kephalaion/internal/ident"
	"github.com/kephalaion/kephalaion/internal/node/replica"
	"github.com/kephalaion/kephalaion/internal/node/store"
	"github.com/kephalaion/kephalaion/internal/sqlitedb"
)

// dummyHash wird verglichen, wenn es zum Account keine Zeile gibt: So kostet
// ein unbekannter Account dieselbe Arbeit wie ein falsches Token.
var dummyHash = ident.HashToken("keph_unbekannter-account")

// Die Werte von Login.State, wie whoami sie als login zeigt.
const (
	// LoginOK: gültig angemeldet.
	LoginOK = "ok"
	// LoginInvalid: Zugangsdaten geschickt, aber ungültig — unbekannter
	// Account, falsches Token, gesperrt, nur ein Header des Paars, oder der
	// Node hat noch keine Replica dieses Hubs. Die Gründe unterscheidet der
	// Node nicht.
	LoginInvalid = "invalid"
	// LoginMissing: nichts geschickt.
	LoginMissing = "missing"
)

// Login ist die Anmeldung an einem Hub-Eintrag.
type Login struct {
	// Hub ist der Alias, Node der Name dieses Nodes am Hub.
	Hub  string
	Node string
	// State ist LoginOK, LoginInvalid oder LoginMissing.
	State   string
	Account string
	// User ist, wem der Account gehört, aus der Account-Zeile; nur bei
	// LoginOK.
	User string
	// Rights sind die Rechte je Collection, nach Collection; nur bei
	// LoginOK.
	Rights []Right
}

// Right ist das Recht eines Accounts in einer Collection.
type Right struct {
	Collection string
	contract.Rights
}

// Logins ist die Anmeldung einer Anfrage über alle Hubs: je Hub-Eintrag des
// Nodes eine, nach Alias, und die Aliase aus Headern, zu denen der Node
// keinen Eintrag hat.
type Logins struct {
	Hubs    []Login
	Unknown []string
}

// Valid liefert die gültigen Anmeldungen.
func (l Logins) Valid() []Login {
	var out []Login
	for _, h := range l.Hubs {
		if h.State == LoginOK {
			out = append(out, h)
		}
	}
	return out
}

// Authenticate prüft die Header einer Anfrage gegen alle Hub-Einträge — die
// eine Anmeldung, auf der whoami und jedes Werkzeug aufsetzen, das Inhalte
// liefert. Je Eintrag mit Header-Paar prüft check es gegen die Replica, ohne
// Cache; ein Eintrag ohne Header ist LoginMissing. Header zu Aliasen ohne
// Eintrag stehen in Unknown. Fehler sind nur Fehler der Datenbank.
func (n *Node) Authenticate(ctx context.Context, header http.Header) (Logins, error) {
	hubs, err := n.nodes.Hubs(ctx)
	if err != nil {
		return Logins{}, err
	}
	list := HubHeaders(header)
	pairs := make(map[string]HubHeader, len(list))
	for _, p := range list {
		pairs[p.Alias] = p
	}
	out := Logins{Hubs: make([]Login, 0, len(hubs)), Unknown: []string{}}
	for _, h := range hubs {
		p, ok := pairs[h.Name]
		delete(pairs, h.Name)
		if !ok {
			out.Hubs = append(out.Hubs, Login{Hub: h.Name, Node: h.NodeName, State: LoginMissing})
			continue
		}
		login, err := n.check(ctx, h, p)
		if err != nil {
			return Logins{}, err
		}
		out.Hubs = append(out.Hubs, login)
	}
	for _, p := range list {
		if _, ok := pairs[p.Alias]; ok {
			out.Unknown = append(out.Unknown, p.Alias)
		}
	}
	return out, nil
}

// check prüft ein Header-Paar gegen die Replica seines Hubs: die lebenden
// Zeilen des Accounts über den Index, sha256 des Tokens, Vergleich in
// konstanter Zeit; ohne Zeile gegen einen Ersatz-Hash. Unbekannter Account,
// falsches Token, eine unvollständige Angabe und eine fehlende Replica
// ergeben dasselbe: LoginInvalid. Eine Zeile, die sich nicht lesen lässt —
// etwa ohne user, von einem Hub vor Task 006 —, zählt nicht. Der User ist der
// der ersten passenden Zeile; der Hub schreibt ihn in alle Zeilen eines
// Accounts unter einer Revision.
func (n *Node) check(ctx context.Context, h store.Hub, p HubHeader) (Login, error) {
	out := Login{Hub: h.Name, Node: h.NodeName, State: LoginInvalid}
	hash := ident.HashToken(p.Token)
	var rows []replica.Document
	if p.Complete && ident.CheckPrincipalName("Account", p.Account) == nil {
		var err error
		if rows, err = accountRows(ctx, n.nodes, h, p.Account); err != nil {
			return out, err
		}
	}
	if len(rows) == 0 {
		subtle.ConstantTimeCompare([]byte(hash), []byte(dummyHash))
		return out, nil
	}
	var rights []Right
	user := ""
	for _, row := range rows {
		c, ok := decodeRow(row)
		if !ok {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(hash), []byte(c.Hash)) == 1 {
			rights = append(rights, Right{Collection: row.Collection, Rights: c.Rights})
			if user == "" {
				user = c.User
			}
		}
	}
	if len(rights) == 0 {
		return out, nil
	}
	out.State, out.Account, out.User, out.Rights = LoginOK, p.Account, user, rights
	return out, nil
}

func decodeRow(row replica.Document) (contract.AccountContent, bool) {
	if row.Content == nil {
		return contract.AccountContent{}, false
	}
	c, err := contract.DecodeAccountContent(*row.Content)
	return c, err == nil
}

// accountRows liest die lebenden Zeilen eines Accounts aus der Replica eines
// Eintrags. Die Replica wird je Anfrage geöffnet: Der Abgleich kann sie
// verwerfen und neu anlegen, node hub rm entfernt sie; eine offen gehaltene
// Datei zeigte dann den alten Stand. Eine Replica, die zu einem früheren
// Eintrag gleichen Namens gehört, zählt nicht.
func accountRows(ctx context.Context, nodes store.Store, h store.Hub, account string) ([]replica.Document, error) {
	rep, err := openReplica(ctx, nodes, h)
	if err != nil || rep == nil {
		return nil, err
	}
	defer rep.Close()
	return rep.AccountRows(ctx, account)
}

// openReplica öffnet die Replica eines Eintrags; fehlt sie oder gehört sie
// zu einem anderen Eintrag, ist sie nil.
func openReplica(ctx context.Context, nodes store.Store, h store.Hub) (*replica.Replica, error) {
	rep, err := replica.Open(ctx, nodes.ReplicaPath(h.Name))
	if errors.Is(err, sqlitedb.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if rep.EntryID() != h.EntryID {
		_ = rep.Close()
		return nil, nil
	}
	return rep, nil
}

// AccountLogins liefert für die Kommandozeile (node whoami <account>), was
// Authenticate einem Client mit gültigen Zugangsdaten dieses Accounts an
// jedem Hub liefern würde — ohne Token: LoginOK, wo der Account lebende
// Zeilen hat, sonst LoginMissing. Wer die CLI aufruft, kann node.db ohnehin
// lesen; ob ein Token gilt, prüft node account check.
func AccountLogins(ctx context.Context, nodes store.Store, account string) (Logins, error) {
	hubs, err := nodes.Hubs(ctx)
	if err != nil {
		return Logins{}, err
	}
	out := Logins{Hubs: make([]Login, 0, len(hubs)), Unknown: []string{}}
	for _, h := range hubs {
		login := Login{Hub: h.Name, Node: h.NodeName, State: LoginMissing}
		rows, err := accountRows(ctx, nodes, h, account)
		if err != nil {
			return Logins{}, err
		}
		for _, row := range rows {
			c, ok := decodeRow(row)
			if !ok {
				continue
			}
			login.Rights = append(login.Rights, Right{Collection: row.Collection, Rights: c.Rights})
			if login.User == "" {
				login.User = c.User
			}
		}
		if len(login.Rights) > 0 {
			login.State, login.Account = LoginOK, account
		}
		out.Hubs = append(out.Hubs, login)
	}
	return out, nil
}

// Account ist ein Account, den der Node aus einer Replica kennt: lebende
// Zeilen SYSTEM:A:<account>.
type Account struct {
	Hub     string
	Account string
	User    string
	Rights  []Right
}

// KnownAccounts liefert die Accounts aller Replicas, nach Hub und Account —
// für node whoami ohne Argument.
func KnownAccounts(ctx context.Context, nodes store.Store) ([]Account, error) {
	hubs, err := nodes.Hubs(ctx)
	if err != nil {
		return nil, err
	}
	var out []Account
	for _, h := range hubs {
		rep, err := openReplica(ctx, nodes, h)
		if err != nil {
			return nil, err
		}
		if rep == nil {
			continue
		}
		rows, err := rep.AllAccountRows(ctx)
		_ = rep.Close()
		if err != nil {
			return nil, err
		}
		byName := map[string]int{}
		for _, row := range rows {
			name, ok := contract.AccountOfRow(row.Name)
			c, valid := decodeRow(row)
			if !ok || !valid {
				continue
			}
			i, seen := byName[name]
			if !seen {
				i = len(out)
				byName[name] = i
				out = append(out, Account{Hub: h.Name, Account: name, User: c.User})
			}
			out[i].Rights = append(out[i].Rights, Right{Collection: row.Collection, Rights: c.Rights})
		}
	}
	return out, nil
}
