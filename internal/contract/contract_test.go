package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

// TestJSON prüft, dass Anfrage und Antwort als JSON laufen: Fassung und
// Anmeldung stehen nicht im Body, NULL bleibt null und kommt als nil zurück.
func TestJSON(t *testing.T) {
	req := SyncRequest{
		Version:     Version,
		Auth:        NodeAuth{Node: "laptop", Token: "keph_geheim"},
		Collections: []Since{{Collection: "a", Since: 3}},
		PageSize:    DefaultPageSize,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); strings.Contains(s, "keph_geheim") || strings.Contains(s, "laptop") ||
		s != `{"collections":[{"collection":"a","since":3}],"page_size":500}` {
		t.Errorf("Anfrage als JSON: %s", s)
	}

	resp := SyncResponse{
		HubID: "01HUB", Version: Version,
		Collections: []CollectionStatus{{Collection: "a", Allowed: true}},
		Allowed:     []string{"a"},
		Rows: []Row{
			{ID: "1", Collection: "a", Name: "x.md", Content: ptr(""), Meta: ptr(`{"k":1}`), Revision: 2},
			{ID: "2", Collection: "a", Name: "y.md", Deleted: true, Revision: 3},
		},
		HubRevision: 3, Until: 3,
	}
	b, err = json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"content":null,"meta":null,"deleted":true`) {
		t.Errorf("Löschmarke als JSON: %s", b)
	}
	var back SyncResponse
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	r0, r1 := back.Rows[0], back.Rows[1]
	if r0.Content == nil || *r0.Content != "" || r0.Meta == nil || *r0.Meta != `{"k":1}` {
		t.Errorf("leerer Inhalt und meta: %+v", r0)
	}
	if r1.Content != nil || r1.Meta != nil || !r1.Deleted {
		t.Errorf("NULL: %+v", r1)
	}
	if back.HubID != "01HUB" || back.Version != Version || back.Until != 3 || back.HubRevision != 3 {
		t.Errorf("Antwort: %+v", back)
	}
}

func TestErrorIs(t *testing.T) {
	err := fmt.Errorf("hub x: %w", Invalid("Seitengröße 0"))
	if !errors.Is(err, ErrInvalid) || errors.Is(err, ErrUnauthenticated) {
		t.Errorf("errors.Is: %v", err)
	}
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeInvalid {
		t.Errorf("errors.As: %#v", err)
	}
	b, err := json.Marshal(ErrUnsupportedVersion)
	if err != nil || string(b) != `{"code":"unsupported_version","message":"Fassung nicht unterstützt"}` {
		t.Errorf("Fehler als JSON: %s, %v", b, err)
	}
}
