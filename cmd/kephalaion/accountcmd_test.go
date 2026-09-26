package main

import (
	"strings"
	"testing"

	"github.com/kephalaion/kephalaion/internal/ident"
)

func TestHubAccountFlow(t *testing.T) {
	dir := isolate(t)
	cfg := setup(t, dir)
	c := "--config=" + cfg
	runT(t, "hub", "collection", "add", "team-x", c).want(t, 0)
	runT(t, "hub", "collection", "add", "privat", c).want(t, 0)
	runT(t, "hub", "node", "add", "laptop", c).want(t, 0)

	r := runT(t, "hub", "account", "add", "bob", "--description", "Bob", c)
	r.want(t, 0, "Account bob angelegt", "wird nicht wieder angezeigt", "node account rotate <hub> bob")
	tok := tokenFrom(t, r.out)
	runT(t, "hub", "account", "add", "bob", c).want(t, 1, "gibt es schon")
	runT(t, "hub", "account", "add", "admin", c).want(t, 1, "reserviert")
	runT(t, "hub", "account", "add", "laptop", c).want(t, 1, "an einen Node vergeben")
	runT(t, "hub", "node", "add", "bob", c).want(t, 1, "an einen Account vergeben")
	runT(t, "hub", "node", "add", "admin", c).want(t, 1, "reserviert")

	runT(t, "hub", "account", "grant", "bob", "team-x", "--write", c).want(t, 0, "team-x erlaubt (read, write)")
	runT(t, "hub", "account", "grant", "bob", "team-x", "--write", c).want(t, 0, "unverändert")
	runT(t, "hub", "account", "grant", "bob", "privat", c).want(t, 0, "privat erlaubt (read)")
	runT(t, "hub", "account", "grant", "bob", "fehlt", c).want(t, 1, "Collection fehlt gibt es nicht")
	runT(t, "hub", "account", "grant", "bob", c).want(t, 2, "Es fehlt: <collection>")
	runT(t, "hub", "account", "list", c).want(t, 0, "bob", "aktiv", "privat (read), team-x (read, write)", "Bob")
	r = runT(t, "hub", "account", "show", "bob", c)
	r.want(t, 0, "Account bob", "Rechte:", "team-x: read, write", "nur als Hash", "von admin")
	if strings.Contains(r.out, tok) || strings.Contains(r.out, ident.HashToken(tok)) {
		t.Error("show zeigt Token oder Hash")
	}
	// grant ohne --write entzieht write.
	runT(t, "hub", "account", "grant", "bob", "team-x", "--supersede", c).want(t, 0, "(read, supersede)")

	runT(t, "hub", "account", "lock", "bob", c).want(t, 0, "gesperrt")
	runT(t, "hub", "account", "lock", "bob", c).want(t, 1, "schon gesperrt")
	runT(t, "hub", "account", "show", "bob", c).want(t, 0, "gesperrt", "gemerkt", "team-x: read, supersede")
	runT(t, "status", c).want(t, 0, "Accounts:      bob (gesperrt)")
	runT(t, "hub", "account", "unlock", "bob", c).want(t, 0, "entsperrt")
	runT(t, "hub", "account", "revoke", "bob", "privat", c).want(t, 0, "privat nicht mehr erlaubt")
	runT(t, "hub", "account", "revoke", "bob", "privat", c).want(t, 1, "keine Rechte")
	r = runT(t, "hub", "account", "token", "bob", c)
	r.want(t, 0, "neues Einrichtungstoken")
	if tokenFrom(t, r.out) == tok {
		t.Error("token liefert das alte Token")
	}
	runT(t, "hub", "account", "set", "bob", "--description", "Robert", c).want(t, 0, "geändert")
	runT(t, "hub", "account", "rm", "bob", c).want(t, 0, "entfernt")
	runT(t, "hub", "account", "list", c).want(t, 0, "Keine Accounts.")
	// Der Name ist wieder frei.
	runT(t, "hub", "node", "add", "bob", c).want(t, 0)
	runT(t, "hub", "account", "--help").want(t, 0, "--supersede")
}
