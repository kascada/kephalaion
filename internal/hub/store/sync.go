package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kephalaion/kephalaion/internal/contract"
)

// syncRowsQuery setzt die Abfrage des Abgleichs für n Collections zusammen,
// mit Grenze, wenn limit gilt. Platzhalter: $1 die obere Revision, dann je
// Collection Name und Revision, zuletzt die Grenze.
func syncRowsQuery(n int, limit bool) string {
	var b strings.Builder
	b.WriteString(queries.SyncRowsHead)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(queries.SyncRowsOr)
		}
		fmt.Fprintf(&b, queries.SyncRowsClause, 2+2*i, 3+2*i)
	}
	b.WriteString(queries.SyncRowsOrder)
	if limit {
		fmt.Fprintf(&b, queries.SyncRowsLimit, 2+2*n)
	}
	return b.String()
}

func (s *sqliteStore) SyncRows(ctx context.Context, since []contract.Since, upTo int64, limit int) ([]contract.Row, error) {
	out := []contract.Row{}
	if len(since) == 0 {
		return out, nil
	}
	args := make([]any, 0, 2+2*len(since))
	args = append(args, upTo)
	for _, c := range since {
		args = append(args, c.Collection, c.Since)
	}
	if limit > 0 {
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q(syncRowsQuery(len(since), limit > 0)), args...)
	if err != nil {
		return nil, fmt.Errorf("Zeilen für den Abgleich lesen: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("Zeilen für den Abgleich lesen: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// scanRow liest eine Zeile in der Reihenfolge von documentColumns; anders
// als scanDocument bleibt NULL als nil erhalten.
func scanRow(sc interface{ Scan(...any) error }) (contract.Row, error) {
	var r contract.Row
	var content, meta sql.NullString
	var deleted int64
	if err := sc.Scan(&r.ID, &r.Collection, &r.Name, &content, &meta, &deleted, &r.Revision,
		&r.CreatedAt, &r.CreatedBy, &r.UpdatedAt, &r.UpdatedBy); err != nil {
		return contract.Row{}, err
	}
	if content.Valid {
		r.Content = &content.String
	}
	if meta.Valid {
		r.Meta = &meta.String
	}
	r.Deleted = deleted != 0
	return r, nil
}
