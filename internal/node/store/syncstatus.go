package store

import (
	"context"
	"database/sql"
	"fmt"
)

// SyncRecord ist das Ergebnis eines Abgleichs, wie RecordSync es festhält.
// Ist Err leer, gelang er: Der letzte Erfolg wird At, ein früherer Fehler
// verschwindet. Sonst bleibt der letzte Erfolg, und der Fehler wird mit At
// und seiner Art (ErrKind) gemerkt.
type SyncRecord struct {
	// At ist der Zeitpunkt in ms seit Epoche.
	At      int64
	Err     string
	ErrKind string
}

// SyncStatus ist der Stand des Abgleichs eines Hub-Eintrags (hub_sync).
// Zeiten in ms seit Epoche, 0 heißt: nie.
type SyncStatus struct {
	OKAt int64
	// Err ist der letzte Fehler, leer, wenn der letzte Versuch gelang.
	Err     string
	ErrKind string
	ErrAt   int64
}

// Abfragen auf hub_sync. Geschrieben wird nur, solange es den Eintrag mit
// dieser entry_id gibt: Das INSERT liest ihn aus hubs; fehlt er, entsteht
// keine Zeile.
const (
	qSyncOK = `INSERT INTO hub_sync (hub, entry_id, ok_at, error, error_kind, error_at)
		SELECT name, entry_id, ?, NULL, NULL, NULL FROM hubs WHERE name = ? AND entry_id = ?
		ON CONFLICT(hub) DO UPDATE SET entry_id = excluded.entry_id, ok_at = excluded.ok_at,
			error = NULL, error_kind = NULL, error_at = NULL`
	qSyncErr = `INSERT INTO hub_sync (hub, entry_id, ok_at, error, error_kind, error_at)
		SELECT name, entry_id, NULL, ?, ?, ? FROM hubs WHERE name = ? AND entry_id = ?
		ON CONFLICT(hub) DO UPDATE SET entry_id = excluded.entry_id, error = excluded.error,
			error_kind = excluded.error_kind, error_at = excluded.error_at`
	qSyncAll = `SELECT hub, COALESCE(ok_at, 0), COALESCE(error, ''), COALESCE(error_kind, ''),
		COALESCE(error_at, 0) FROM hub_sync ORDER BY hub`
	qSyncOfDel = `DELETE FROM hub_sync WHERE hub = ?`
	qSyncClear = `DELETE FROM hub_sync`
)

func (s *sqliteStore) RecordSync(ctx context.Context, name, entryID string, rec SyncRecord) error {
	var res sql.Result
	var err error
	if rec.Err == "" {
		res, err = s.db.ExecContext(ctx, qSyncOK, rec.At, name, entryID)
	} else {
		res, err = s.db.ExecContext(ctx, qSyncErr, rec.Err, rec.ErrKind, rec.At, name, entryID)
	}
	if err != nil {
		return fmt.Errorf("Stand des Abgleichs schreiben: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("Hub-Eintrag %s %w", name, ErrEntryGone)
	}
	return nil
}

func (s *sqliteStore) SyncStatus(ctx context.Context) (map[string]SyncStatus, error) {
	rows, err := s.db.QueryContext(ctx, qSyncAll)
	if err != nil {
		return nil, fmt.Errorf("Stand des Abgleichs lesen: %w", err)
	}
	defer rows.Close()
	out := map[string]SyncStatus{}
	for rows.Next() {
		var name string
		var st SyncStatus
		if err := rows.Scan(&name, &st.OKAt, &st.Err, &st.ErrKind, &st.ErrAt); err != nil {
			return nil, fmt.Errorf("Stand des Abgleichs lesen: %w", err)
		}
		out[name] = st
	}
	return out, rows.Err()
}
