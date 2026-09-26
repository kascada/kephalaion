package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func addHub(t *testing.T, s Store, name string) Hub {
	t.Helper()
	ctx := context.Background()
	if err := s.AddHub(ctx, Hub{Name: name, NodeName: "laptop", Transport: TransportHTTPS,
		Address: "https://hub.example.org", Token: token(t)}, false); err != nil {
		t.Fatal(err)
	}
	h, err := s.Hub(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// Erfolg und Fehler landen in hub_sync; ein Fehler lässt den letzten Erfolg
// stehen, ein Erfolg leert den Fehler.
func TestRecordSync(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	a := addHub(t, s, "a")
	b := addHub(t, s, "b")
	if st, err := s.SyncStatus(ctx); err != nil || len(st) != 0 {
		t.Fatalf("leer: %+v, %v", st, err)
	}
	if err := s.RecordSync(ctx, "a", a.EntryID, SyncRecord{At: 100}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSync(ctx, "a", a.EntryID, SyncRecord{At: 200, Err: "Hub weg", ErrKind: "unreachable"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSync(ctx, "b", b.EntryID, SyncRecord{At: 300, Err: "nicht angemeldet", ErrKind: "unauthenticated"}); err != nil {
		t.Fatal(err)
	}
	want := map[string]SyncStatus{
		"a": {OKAt: 100, Err: "Hub weg", ErrKind: "unreachable", ErrAt: 200},
		"b": {Err: "nicht angemeldet", ErrKind: "unauthenticated", ErrAt: 300},
	}
	if st, err := s.SyncStatus(ctx); err != nil || !reflect.DeepEqual(st, want) {
		t.Errorf("nach Fehlern: %+v, %v", st, err)
	}
	if err := s.RecordSync(ctx, "a", a.EntryID, SyncRecord{At: 400}); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.SyncStatus(ctx); !reflect.DeepEqual(st["a"], SyncStatus{OKAt: 400}) {
		t.Errorf("Fehler nach Erfolg nicht leer: %+v", st["a"])
	}

	// Unbekannter Alias oder alte entry_id: nichts geschrieben.
	if err := s.RecordSync(ctx, "fehlt", a.EntryID, SyncRecord{At: 1}); !errors.Is(err, ErrEntryGone) {
		t.Errorf("unbekannt: %v", err)
	}
	if err := s.RecordSync(ctx, "b", a.EntryID, SyncRecord{At: 1}); !errors.Is(err, ErrEntryGone) {
		t.Errorf("fremde entry_id: %v", err)
	}

	// node hub rm räumt mit ab; ein neuer Eintrag gleichen Namens beginnt leer,
	// und die alte entry_id schreibt nicht hinein.
	if err := s.RemoveHub(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	b2 := addHub(t, s, "b")
	if err := s.RecordSync(ctx, "b", b.EntryID, SyncRecord{At: 500}); !errors.Is(err, ErrEntryGone) {
		t.Errorf("alte entry_id nach rm und add: %v", err)
	}
	if st, _ := s.SyncStatus(ctx); len(st) != 1 {
		t.Errorf("nach rm: %+v", st)
	}
	if err := s.RecordSync(ctx, "b", b2.EntryID, SyncRecord{At: 600}); err != nil {
		t.Fatal(err)
	}

	// Der Stand steht nicht in den Tabellen des Exports; der Import räumt ihn
	// ab und behält die entry_id der Aliase, die bleiben.
	tables, err := s.Tables(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := range tables.Hubs {
		tables.Hubs[i].EntryID = ""
	}
	tables.Hubs = tables.Hubs[:1] // nur a
	tables.Wanted = nil
	if err := s.Import(ctx, map[string]string{}, &tables, false); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.SyncStatus(ctx); len(st) != 0 {
		t.Errorf("nach Import: %+v", st)
	}
	if h, _ := s.Hub(ctx, "a"); h.EntryID != a.EntryID {
		t.Errorf("entry_id nach Import %q, vorher %q", h.EntryID, a.EntryID)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	for _, bad := range []struct{ key, value string }{
		{"sync_intervall", "30s"}, {"sync_interval", "bald"}, {"sync_interval", "500ms"},
		{"sync_interval", "-5s"}, {"sync_interval", ""},
	} {
		if err := s.SetSetting(ctx, bad.key, bad.value); err == nil {
			t.Errorf("%s = %q ging durch", bad.key, bad.value)
		}
	}
	if _, err := s.UnsetSetting(ctx, "fremd"); !errors.Is(err, ErrUnknownSetting) {
		t.Errorf("unset unbekannt: %v", err)
	}
	if err := s.SetSetting(ctx, SettingSyncInterval, "2m"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, SettingSyncInterval, "5s"); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.Settings(ctx)
	if d, err := SyncInterval(settings); d != 5*time.Second || err != nil {
		t.Errorf("SyncInterval = %s, %v", d, err)
	}
	if removed, err := s.UnsetSetting(ctx, SettingSyncInterval); !removed || err != nil {
		t.Errorf("unset: %v, %v", removed, err)
	}
	if removed, err := s.UnsetSetting(ctx, SettingSyncInterval); removed || err != nil {
		t.Errorf("unset zweimal: %v, %v", removed, err)
	}
	settings, _ = s.Settings(ctx)
	if d, err := SyncInterval(settings); d != DefaultSyncInterval || err != nil {
		t.Errorf("Standard = %s, %v", d, err)
	}
	for v, want := range map[string]time.Duration{"0": 0, "0s": 0, "1s": time.Second, "1m30s": 90 * time.Second} {
		if d, err := ParseSyncInterval(v); d != want || err != nil {
			t.Errorf("ParseSyncInterval(%q) = %s, %v", v, d, err)
		}
	}
	// Ein ungültiger Wert, etwa aus einem Import: Fehler, und der Standard.
	if d, err := SyncInterval(map[string]string{SettingSyncInterval: "x"}); d != DefaultSyncInterval || err == nil {
		t.Errorf("ungültig: %s, %v", d, err)
	}
}
