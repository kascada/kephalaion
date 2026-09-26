package replica

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/kephalaion/kephalaion/internal/contract"
	"github.com/kephalaion/kephalaion/internal/ident"
	"github.com/kephalaion/kephalaion/internal/node/store"
)

func accountRow(t *testing.T, collection, account, token string, rev int64, r contract.Rights) contract.Row {
	t.Helper()
	content, err := contract.EncodeAccountContent(contract.AccountContent{Hash: ident.HashToken(token), Rights: r})
	if err != nil {
		t.Fatal(err)
	}
	return contract.Row{ID: ulid.Make().String(), Collection: collection, Name: contract.AccountRowName(account),
		Content: &content, Revision: rev, CreatedAt: 1, CreatedBy: "admin", UpdatedAt: 1, UpdatedBy: "admin"}
}

func (e *env) hubEntry() store.Hub {
	e.t.Helper()
	h, err := e.nodes.Hub(context.Background(), "privat")
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

// rotate schreibt Zeilen in die Replica, nur die gewünschter Collections, und
// legt sie an, wenn es sie nicht gibt; AccountRows liest sie über den Index.
func TestWriteAccountRows(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, "a")
	rows := []contract.Row{
		accountRow(t, "a", "bob", "keph_neu", 7, contract.Rights{Write: true}),
		accountRow(t, "b", "bob", "keph_neu", 7, contract.Rights{}),
	}
	written, reset, err := WriteAccountRows(ctx, e.nodes, e.hubEntry(), e.hub.id, rows)
	if err != nil || reset != "" || len(written) != 1 || written[0] != "a" {
		t.Fatalf("WriteAccountRows = %v, %q, %v", written, reset, err)
	}
	if h := e.hubEntry(); h.HubID != e.hub.id {
		t.Errorf("Kopie der hub_id = %q", h.HubID)
	}
	r, err := Open(ctx, e.nodes.ReplicaPath("privat"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.AccountRows(ctx, "bob")
	if err != nil || len(got) != 1 || got[0].Collection != "a" {
		t.Errorf("AccountRows = %+v, %v", got, err)
	}
	if other, _ := r.AccountRows(ctx, "alice"); len(other) != 0 {
		t.Errorf("alice: %+v", other)
	}
	var plan strings.Builder
	qrows, err := r.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+qAccountRows, "SYSTEM:A:bob")
	if err != nil {
		t.Fatal(err)
	}
	for qrows.Next() {
		var id, parent, notused int
		var detail string
		if err := qrows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	qrows.Close()
	if !strings.Contains(plan.String(), "documents_system") {
		t.Errorf("AccountRows ohne Index documents_system:\n%s", plan.String())
	}
	_ = r.Close()

	// Nichts Gewünschtes: nichts geschrieben.
	written, _, err = WriteAccountRows(ctx, e.nodes, e.hubEntry(), e.hub.id, rows[1:])
	if err != nil || len(written) != 0 {
		t.Errorf("nur b: %v, %v", written, err)
	}
	// Andere hub_id: die Replica wird geleert, dann geschrieben.
	e.put(t)
	written, reset, err = WriteAccountRows(ctx, e.nodes, e.hubEntry(), ulid.Make().String(), rows)
	if err != nil || len(written) != 1 || !strings.Contains(reset, "hub_id gewechselt") {
		t.Errorf("andere hub_id: %v, %q, %v", written, reset, err)
	}
	if n := e.countRows(t); n != 1 {
		t.Errorf("%d Zeilen nach dem Wechsel, erwartet nur die Account-Zeile", n)
	}
	// Keine Account-Zeile: abgelehnt.
	bad := rows[0]
	bad.Name = "x.md"
	if _, _, err := WriteAccountRows(ctx, e.nodes, e.hubEntry(), e.hub.id, []contract.Row{bad}); err == nil {
		t.Error("Dokument als Account-Zeile angenommen")
	}
}

// put gleicht einmal ab, damit die Replica ein Dokument hat.
func (e *env) put(t *testing.T) {
	t.Helper()
	e.hub.put("a", "doc.md", "x")
	if res := e.run(); res.Err != nil {
		t.Fatal(res.Err)
	}
}

func (e *env) countRows(t *testing.T) int {
	t.Helper()
	r, err := Open(context.Background(), e.nodes.ReplicaPath("privat"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&n); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	return n
}

func TestAdoptHubID(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, "a")
	// Ohne Replica: nur die Kopie.
	reset, err := AdoptHubID(ctx, e.nodes, e.hubEntry(), e.hub.id)
	if err != nil || reset != "" || e.hubEntry().HubID != e.hub.id {
		t.Fatalf("erster Kontakt: %q, %v, %q", reset, err, e.hubEntry().HubID)
	}
	e.put(t)
	if reset, err := AdoptHubID(ctx, e.nodes, e.hubEntry(), e.hub.id); err != nil || reset != "" || e.countRows(t) != 1 {
		t.Errorf("gleiche hub_id: %q, %v", reset, err)
	}
	other := ulid.Make().String()
	reset, err = AdoptHubID(ctx, e.nodes, e.hubEntry(), other)
	if err != nil || !strings.Contains(reset, "hub_id gewechselt") || e.countRows(t) != 0 || e.hubEntry().HubID != other {
		t.Errorf("andere hub_id: %q, %v", reset, err)
	}
}
