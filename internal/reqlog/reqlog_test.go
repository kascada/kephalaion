package reqlog

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.now = func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) }
	h := l.Middleware("node", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Note(r.Context(), "account", "alice")
		Note(r.Context(), "account", "keph_geheim\nzeile")
		NoteError(r.Context(), errors.New("kaputt"))
		w.WriteHeader(http.StatusForbidden)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp?x=%0A", nil))
	line := buf.String()
	want := "2026-09-26T12:00:00Z node POST /mcp 403 0s account=alice account=(ungültig) error=\"kaputt\"\n"
	if line != want {
		t.Errorf("Zeile\n%q\nerwartet\n%q", line, want)
	}
	if strings.Contains(line, "geheim") {
		t.Error("Token im Log")
	}
}
