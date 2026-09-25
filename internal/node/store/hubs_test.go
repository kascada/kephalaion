package store

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/kascada/kephalaion/internal/ident"
)

func newStore(t *testing.T) Store {
	t.Helper()
	s, err := Create(context.Background(), newDB(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func token(t *testing.T) string {
	t.Helper()
	tok, err := ident.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func ptr(s string) *string { return &s }

func TestTransportRules(t *testing.T) {
	tok := token(t)
	cases := []struct {
		name  string
		h     Hub
		local bool
		ok    bool
	}{
		{"local mit Hub", Hub{Transport: "local"}, true, true},
		{"local ohne Hub", Hub{Transport: "local"}, false, false},
		{"local mit Adresse", Hub{Transport: "local", Address: "x"}, true, false},
		{"http localhost", Hub{Transport: "http", Address: "http://localhost:8080"}, false, true},
		{"http 127.0.0.1", Hub{Transport: "http", Address: "http://127.0.0.1:8080"}, false, true},
		{"http ::1", Hub{Transport: "http", Address: "http://[::1]:8080"}, false, true},
		{"http entfernt", Hub{Transport: "http", Address: "http://hub.example.org"}, false, false},
		{"http mit https-URL", Hub{Transport: "http", Address: "https://localhost"}, false, false},
		{"http ohne Adresse", Hub{Transport: "http"}, false, false},
		{"https", Hub{Transport: "https", Address: "https://hub.example.org"}, false, true},
		{"https mit http-URL", Hub{Transport: "https", Address: "http://hub.example.org"}, false, false},
		{"https ohne Host", Hub{Transport: "https", Address: "https://"}, false, false},
		{"ssh", Hub{Transport: "ssh", Address: "keph@hub:2222"}, false, true},
		{"ssh mit Schlüssel", Hub{Transport: "ssh", Address: "hub", SSHKey: "/k/id"}, false, true},
		{"ssh ohne Adresse", Hub{Transport: "ssh"}, false, false},
		{"Schlüssel bei https", Hub{Transport: "https", Address: "https://h", SSHKey: "/k"}, false, false},
		{"unbekannt", Hub{Transport: "ftp", Address: "ftp://h"}, false, false},
		{"ohne Transport", Hub{}, true, false},
	}
	for _, c := range cases {
		c.h.Name = "privat"
		c.h.Token = tok
		err := CheckHub(c.h, c.local)
		if (err == nil) != c.ok {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if err := CheckHub(Hub{Name: "privat", Transport: "local", Token: "keph_kurz"}, true); err == nil {
		t.Error("ungültiges Token angenommen")
	}
	if err := CheckHub(Hub{Name: "System", Transport: "local", Token: tok}, true); err == nil {
		t.Error("ungültiger Name angenommen")
	}
}

func TestHubs(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	tok := token(t)
	if err := s.AddHub(ctx, Hub{Name: "lokal", Transport: "local", Token: tok}, false); err == nil {
		t.Error("local ohne Hub in der config angenommen")
	}
	if err := s.AddHub(ctx, Hub{Name: "lokal", Transport: "local", Token: tok}, true); err != nil {
		t.Fatal(err)
	}
	if err := s.AddHub(ctx, Hub{Name: "zweit", Transport: "local", Token: tok}, true); err == nil ||
		!strings.Contains(err.Error(), "local") {
		t.Errorf("zweites local: %v", err)
	}
	if err := s.AddHub(ctx, Hub{Name: "lokal", Transport: "http", Address: "http://localhost:1", Token: tok}, true); !errors.Is(err, ErrExists) {
		t.Errorf("doppelt: %v", err)
	}
	if err := s.AddHub(ctx, Hub{Name: "test", Transport: "http", Address: "http://localhost:8080", Token: tok}, true); err != nil {
		t.Fatal(err)
	}
	hubs, err := s.Hubs(ctx)
	if err != nil || len(hubs) != 2 || hubs[0].Name != "lokal" || hubs[1].Address != "http://localhost:8080" ||
		hubs[1].Token != tok || hubs[1].HubID != "" {
		t.Errorf("Hubs = %+v, %v", hubs, err)
	}
	tok2 := token(t)
	if err := s.SetHubToken(ctx, "test", tok2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHubToken(ctx, "test", "keph_x"); err == nil {
		t.Error("ungültiges Token angenommen")
	}
	if err := s.SetHubToken(ctx, "fehlt", tok2); !errors.Is(err, ErrNotFound) {
		t.Errorf("token auf Fehlendes: %v", err)
	}
	if h, _ := s.Hub(ctx, "test"); h.Token != tok2 {
		t.Error("Token nicht ersetzt")
	}
	if _, err := s.Hub(ctx, "fehlt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("show auf Fehlendes: %v", err)
	}
}

func TestSetHub(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	tok := token(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.AddHub(ctx, Hub{Name: "lokal", Transport: "local", Token: tok}, true))
	must(s.AddHub(ctx, Hub{Name: "fern", Transport: "ssh", Address: "keph@hub", SSHKey: "/k/id", Token: tok}, true))
	must(s.AddCollection(ctx, "fern", "team-x"))
	// hub_id von Hand, wie nach einem Kontakt.
	db := s.(*sqliteStore).db
	const hubID = "01J8Z3N6Q4T3V5W7X9Y0A1B2C3"
	if _, err := db.ExecContext(ctx, `UPDATE hubs SET hub_id = ? WHERE name = 'fern'`, hubID); err != nil {
		t.Fatal(err)
	}

	// Ein zweites local wird abgewiesen, nichts ändert sich.
	if err := s.SetHub(ctx, "fern", HubUpdate{Transport: ptr("local")}, true); err == nil {
		t.Fatal("zweites local angenommen")
	}
	if h, _ := s.Hub(ctx, "fern"); h.Transport != "ssh" || h.Address != "keph@hub" {
		t.Errorf("nach Abweisung verändert: %+v", h)
	}
	// Adresse ändern, Rest bleibt.
	must(s.SetHub(ctx, "fern", HubUpdate{Address: ptr("keph@hub2:22")}, true))
	if h, _ := s.Hub(ctx, "fern"); h.Address != "keph@hub2:22" || h.SSHKey != "/k/id" || h.Transport != "ssh" {
		t.Errorf("nach Adresse: %+v", h)
	}
	// --ssh-key bei anderem Transport wird abgewiesen.
	if err := s.SetHub(ctx, "lokal", HubUpdate{SSHKey: ptr("/k")}, true); err == nil {
		t.Error("ssh-key bei local angenommen")
	}
	// Wechsel auf local verwirft Adresse und Schlüssel; hub_collections und
	// hub_id bleiben.
	must(s.RemoveHub(ctx, "lokal"))
	must(s.SetHub(ctx, "fern", HubUpdate{Transport: ptr("local")}, true))
	h, _ := s.Hub(ctx, "fern")
	if h.Transport != "local" || h.Address != "" || h.SSHKey != "" || h.HubID != hubID || h.Token != tok ||
		!reflect.DeepEqual(h.Collections, []string{"team-x"}) {
		t.Errorf("nach Wechsel auf local: %+v", h)
	}
	// Wechsel auf https braucht eine passende Adresse.
	if err := s.SetHub(ctx, "fern", HubUpdate{Transport: ptr("https")}, true); err == nil {
		t.Error("https ohne Adresse angenommen")
	}
	must(s.SetHub(ctx, "fern", HubUpdate{Transport: ptr("https"), Address: ptr("https://hub.example.org")}, true))
	// Wechsel auf local mit Adresse wird abgewiesen.
	if err := s.SetHub(ctx, "fern", HubUpdate{Transport: ptr("local"), Address: ptr("x")}, true); err == nil {
		t.Error("local mit Adresse angenommen")
	}
	if err := s.SetHub(ctx, "fehlt", HubUpdate{Address: ptr("x")}, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("set auf Fehlendes: %v", err)
	}
}

func TestCollections(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	tok := token(t)
	if err := s.AddCollection(ctx, "privat", "team-x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ohne Hub-Eintrag: %v", err)
	}
	if err := s.AddHub(ctx, Hub{Name: "privat", Transport: "https", Address: "https://h", Token: tok}, false); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][2]string{{"privat", "Team"}, {"privat", "system"}, {"privat", "a:b"}, {"Privat", "x"}, {"privat", ""}} {
		if err := s.AddCollection(ctx, bad[0], bad[1]); err == nil {
			t.Errorf("%v angenommen", bad)
		}
	}
	if err := s.AddCollection(ctx, "privat", "team-x"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddCollection(ctx, "privat", "team-x"); !errors.Is(err, ErrExists) {
		t.Errorf("doppelt: %v", err)
	}
	if err := s.AddCollection(ctx, "privat", "notizen"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Collections(ctx)
	want := []Wanted{{"privat", "notizen"}, {"privat", "team-x"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Collections = %v, %v", got, err)
	}
	if err := s.RemoveCollection(ctx, "privat", "notizen"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveCollection(ctx, "privat", "notizen"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rm doppelt: %v", err)
	}
	// rm des Hubs entfernt Abhängiges.
	if err := s.RemoveHub(ctx, "privat"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveHub(ctx, "privat"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rm doppelt: %v", err)
	}
	if got, _ := s.Collections(ctx); len(got) != 0 {
		t.Errorf("Reste: %v", got)
	}
}

func TestCheckTables(t *testing.T) {
	tok := token(t)
	ok := Tables{
		Hubs:   []Hub{{Name: "a", Transport: "local", Token: tok}, {Name: "b", Transport: "https", Address: "https://h", Token: tok}},
		Wanted: []Wanted{{"a", "x"}},
	}
	if err := CheckTables(ok, true); err != nil {
		t.Fatal(err)
	}
	if err := CheckTables(ok, false); err == nil {
		t.Error("local ohne Hub in der config angenommen")
	}
	bad := []Tables{
		{Hubs: []Hub{ok.Hubs[0], {Name: "c", Transport: "local", Token: tok}}},
		{Hubs: []Hub{ok.Hubs[1], ok.Hubs[1]}},
		{Hubs: ok.Hubs, Wanted: []Wanted{{"x", "y"}}},
		{Hubs: ok.Hubs, Wanted: []Wanted{{"a", "Y"}}},
		{Hubs: ok.Hubs, Wanted: []Wanted{{"a", "x"}, {"a", "x"}}},
		{Hubs: []Hub{{Name: "a", Transport: "https", Address: "https://h", Token: "keph_x"}}},
		{Hubs: []Hub{{Name: "a", Transport: "https", Address: "https://h", Token: tok, HubID: "kaputt"}}},
	}
	for i, b := range bad {
		if err := CheckTables(b, true); err == nil {
			t.Errorf("Fall %d angenommen", i)
		}
	}
}
