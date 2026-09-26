package upgrade

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
)

var fixedNow = time.Date(2026, 9, 26, 10, 0, 0, 0, time.FixedZone("CEST", 2*3600))

// readOnly nimmt dem Verzeichnis des Binarys das Schreibrecht.
func readOnly(t *testing.T, exe string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("als root greifen Dateirechte nicht")
	}
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// upgrade --check nennt zusätzlich, ob sich das Binary selbst ersetzen kann,
// und den Weg — und lässt keine Probedatei zurück.
func TestCheckReportsWay(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	dir := filepath.Dir(exe)

	u, out := f.upgrader("v0.1.0", exe)
	if _, err := u.Run(context.Background(), Options{Check: true}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Neue Version verfügbar: v0.1.0 → v0.2.0\n", "Selbst ersetzen: ja (Schreibrecht in " + dir + ")\n",
		"Weg: kephalaion upgrade (ersetzt dieses Binary und startet einen laufenden Dienst pro User neu)\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("Ausgabe ohne %q:\n%s", want, out.String())
		}
	}
	assertUnchanged(t, exe)

	// dev build: nur mit ausdrücklicher Version.
	u, out = f.upgrader(buildinfo.DevVersion, exe)
	if _, err := u.Run(context.Background(), Options{Check: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Weg: dieser dev build wird nur mit ausdrücklicher Version ersetzt: "+
		"kephalaion upgrade --version v0.2.0\n") {
		t.Errorf("dev build:\n%s", out.String())
	}

	readOnly(t, exe)
	u, out = f.upgrader("v0.1.0", exe)
	if _, err := u.Run(context.Background(), Options{Check: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Selbst ersetzen: nein (kein Schreibrecht in "+dir+")\n") ||
		!strings.Contains(out.String(), "Weg: kein Schreibrecht in "+dir+" — ersetzen kann dieses Binary nur") {
		t.Errorf("ohne Schreibrecht:\n%s", out.String())
	}
	u, out = f.upgrader("v0.1.0", exe)
	u.System = true
	if _, err := u.Run(context.Background(), Options{Check: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Weg: globale Installation — das Upgrade macht der Verwalter: über Ansible "+
		"(Version anheben) oder sudo kephalaion upgrade && sudo systemctl restart kephalaion\n") {
		t.Errorf("global:\n%s", out.String())
	}
	assertUnchanged(t, exe)
}

func TestReport(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)
	u.Now = func() time.Time { return fixedNow }

	r := u.Report(context.Background())
	want := Report{State: StateOK, CheckedAt: "2026-09-26T08:00:00Z", Version: "v0.1.0", Latest: "v0.2.0",
		UpdateAvailable: true, SelfUpgrade: true, Method: MethodSelf, Command: "kephalaion upgrade",
		Hint: "kephalaion upgrade (ersetzt dieses Binary und startet einen laufenden Dienst pro User neu)"}
	if r != want {
		t.Fatalf("Report\n%+v\nerwartet\n%+v", r, want)
	}
	assertUnchanged(t, exe)
	if n := f.downloads.Load(); n != 0 {
		t.Errorf("%d Downloads für einen Report", n)
	}

	// Die JSON-Felder sind die Schnittstelle für k-playbook.
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"state":"ok"`, `"checked_at":"2026-09-26T08:00:00Z"`, `"version":"v0.1.0"`,
		`"dev_build":false`, `"latest":"v0.2.0"`, `"update_available":true`, `"self_upgrade":true`, `"method":"self"`,
		`"command":"kephalaion upgrade"`, `"hint":`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON ohne %s: %s", key, b)
		}
	}
	if strings.Contains(string(b), `"error"`) {
		t.Errorf("error bei ok: %s", b)
	}
	if !strings.Contains(r.Summary(), "Update: v0.2.0 verfügbar (installiert v0.1.0, geprüft 2026-09-26T08:00:00Z); Weg: kephalaion upgrade") {
		t.Errorf("Summary: %s", r.Summary())
	}

	// Aktuell.
	u, _ = f.upgrader("v0.2.0", exe)
	if r := u.Report(context.Background()); r.State != StateOK || r.UpdateAvailable ||
		!strings.Contains(r.Summary(), "Update: keins, v0.2.0 ist aktuell") {
		t.Errorf("aktuell: %+v %s", r, r.Summary())
	}
	// dev build: nie update_available, Weg mit Version.
	u, _ = f.upgrader(buildinfo.DevVersion, exe)
	if r := u.Report(context.Background()); r.State != StateOK || r.UpdateAvailable || !r.DevBuild ||
		r.Method != MethodExplicit || r.Command != "kephalaion upgrade --version v0.2.0" {
		t.Errorf("dev: %+v", r)
	}
	// global ohne Schreibrecht, dev build: der Verwalter mit Version.
	readOnly(t, exe)
	u, _ = f.upgrader(buildinfo.DevVersion, exe)
	u.System = true
	if r := u.Report(context.Background()); r.SelfUpgrade || r.Method != MethodAdmin ||
		r.Command != "sudo kephalaion upgrade --version v0.2.0 && sudo systemctl restart kephalaion" {
		t.Errorf("global dev: %+v", r)
	}
	u, _ = f.upgrader("v0.1.0", exe)
	if r := u.Report(context.Background()); r.SelfUpgrade || r.Method != MethodManual || r.Command != "" {
		t.Errorf("ohne Schreibrecht: %+v", r)
	}
}

// Scheitert die Frage an GitHub, ist der Report failed mit Grund, der Weg
// steht trotzdem darin; Local fragt gar nicht.
func TestReportFailed(t *testing.T) {
	f := newFakeGitHub(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		return true
	}
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)
	u.Now = func() time.Time { return fixedNow }
	r := u.Report(context.Background())
	if r.State != StateFailed || !strings.Contains(r.Error, "Anfrage-Limit") || r.Latest != "" || r.UpdateAvailable ||
		r.CheckedAt != "2026-09-26T08:00:00Z" || r.Method != MethodSelf {
		t.Fatalf("Report %+v", r)
	}
	if !strings.HasPrefix(r.Summary(), "Update: Prüfung gescheitert (2026-09-26T08:00:00Z): Das Anfrage-Limit") {
		t.Errorf("Summary: %s", r.Summary())
	}

	before := f.requests.Load()
	l := u.Local()
	if l.State != StateUnchecked || l.Error != "noch nicht geprüft" || l.CheckedAt != "" || l.Method != MethodSelf ||
		f.requests.Load() != before {
		t.Errorf("Local %+v", l)
	}
	if l.Summary() != "Update: noch nicht geprüft" {
		t.Errorf("Summary: %s", l.Summary())
	}
}
