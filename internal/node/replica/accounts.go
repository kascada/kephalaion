package replica

import (
	"context"
	"fmt"
	"slices"

	"github.com/kascada/kephalaion/internal/contract"
	"github.com/kascada/kephalaion/internal/node/store"
)

// qAccountRows liest die lebenden Zeilen eines Accounts, nach Collection. Die
// Bedingung name LIKE 'SYSTEM:%' steht wörtlich wie im Teilindex
// documents_system, damit SQLite ihn benutzt; genau grenzt name = ? ein.
const qAccountRows = `SELECT ` + documentColumns + ` FROM documents
	WHERE name = ? AND name LIKE 'SYSTEM:%' AND deleted = 0
	ORDER BY collection, revision DESC, id DESC`

// AccountRows liest die lebenden Zeilen SYSTEM:A:<account> der Replica, je
// Collection die jüngste, nach Collection — über den Index documents_system,
// ohne Transaktion und ohne Cache: Die Datenbank ist die einzige Wahrheit.
func (r *Replica) AccountRows(ctx context.Context, account string) ([]Document, error) {
	rows, err := r.db.QueryContext(ctx, qAccountRows, contract.AccountRowName(account))
	if err != nil {
		return nil, fmt.Errorf("Account-Zeilen lesen: %w", err)
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("Account-Zeilen lesen: %w", err)
		}
		if n := len(out); n > 0 && out[n-1].Collection == d.Collection {
			continue
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// AdoptHubID übernimmt die hub_id, die ein Hub genannt hat (whoami): Weicht
// sie von der Replica ab, wird die Replica geleert (Regel aus docs/vertrag.md,
// „hub_id“) und reset sagt warum; danach folgt die Kopie in node.db. Fehlt
// die Replica, wird nur die Kopie geschrieben — anlegen tut sie der Abgleich.
func AdoptHubID(ctx context.Context, nodes store.Store, h store.Hub, hubID string) (reset string, err error) {
	rep, reason, err := openForSync(ctx, nodes.ReplicaPath(h.Name))
	if err != nil {
		return "", err
	}
	reset = reason
	if rep != nil {
		defer rep.Close()
		if rep.HubID() != hubID {
			why := fmt.Sprintf("hub_id gewechselt (%s → %s); Replica geleert, der nächste Abgleich beginnt von vorn",
				rep.HubID(), hubID)
			if err := rep.reset(ctx, hubID); err != nil {
				return "", err
			}
			reset = why
		}
	}
	if h.HubID != hubID {
		if err := nodes.SetHubID(ctx, h.Name, hubID); err != nil {
			return reset, err
		}
	}
	return reset, nil
}

// WriteAccountRows schreibt die Zeilen, die ein rotate geliefert hat, in die
// Replica eines Hub-Eintrags — nur die der gewünschten Collections. Fehlt die
// Replica, legt es sie an; nennt sie eine andere hub_id, wird sie zuerst
// geleert (wie beim Abgleich). Der Stand des Abgleichs (sync_state) bleibt:
// Der nächste Abgleich liefert die Zeilen noch einmal, per id ersetzt.
// written sind die Collections, deren Zeile geschrieben wurde.
func WriteAccountRows(ctx context.Context, nodes store.Store, h store.Hub, hubID string, rows []contract.Row) (written []string, reset string, err error) {
	var keep []contract.Row
	for _, row := range rows {
		if _, ok := contract.AccountOfRow(row.Name); !ok {
			return nil, "", fmt.Errorf("der Hub liefert %q, keine Account-Zeile", row.Name)
		}
		if slices.Contains(h.Collections, row.Collection) {
			keep = append(keep, row)
			written = append(written, row.Collection)
		}
	}
	if len(keep) == 0 {
		return nil, "", nil
	}
	path := nodes.ReplicaPath(h.Name)
	rep, reason, err := openForSync(ctx, path)
	if err != nil {
		return nil, "", err
	}
	reset = reason
	if rep == nil {
		if rep, err = Create(ctx, path, hubID); err != nil {
			return nil, reset, err
		}
	} else if rep.HubID() != hubID {
		reset = fmt.Sprintf("hub_id gewechselt (%s → %s); Replica geleert, der nächste Abgleich beginnt von vorn",
			rep.HubID(), hubID)
		if err := rep.reset(ctx, hubID); err != nil {
			_ = rep.Close()
			return nil, "", err
		}
	}
	defer rep.Close()
	if _, err := rep.apply(ctx, page{rows: keep, advance: map[string]int64{}}); err != nil {
		return nil, reset, err
	}
	if h.HubID != hubID {
		if err := nodes.SetHubID(ctx, h.Name, hubID); err != nil {
			return written, reset, err
		}
	}
	return written, reset, nil
}
