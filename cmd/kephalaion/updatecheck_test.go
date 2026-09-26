package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
	"github.com/kephalaion/kephalaion/internal/config"
	"github.com/kephalaion/kephalaion/internal/node/mcpnode"
	"github.com/kephalaion/kephalaion/internal/upgrade"
)

// serve fragt als Node einmal bei GitHub und beantwortet whoami danach aus dem
// Speicher, beliebig oft; der Weg ist der aus Sicht von serve — mit globaler
// config und ohne Schreibrecht der des Verwalters.
func TestServeUpdateInWhoami(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("als root greifen Dateirechte nicht")
	}
	dir := isolate(t)
	var requests atomic.Int32
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"tag_name":"v0.2.0","assets":[]}`)
	}))
	t.Cleanup(gh.Close)
	// Aufgelöst wie in upgrade: Auf macOS liegt das temporäre Verzeichnis
	// hinter einem Link.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(resolved, "usr-local-bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(binDir, "kephalaion")
	if err := os.WriteFile(exe, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(binDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(binDir, 0o755) })
	old := newUpgrader
	newUpgrader = func(out io.Writer) *upgrade.Upgrader {
		u := old(out)
		u.APIBase, u.Client = gh.URL, gh.Client()
		u.Current = buildinfo.Info{Version: "v0.1.0", OS: "linux", Arch: "amd64"}
		u.Executable = func() (string, error) { return exe, nil }
		return u
	}
	t.Cleanup(func() { newUpgrader = old })

	runT(t, "node", "init").want(t, 0)
	cfg, _, err := config.Load(filepath.Join(dir, "config", "kephalaion", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Node.Listen = "127.0.0.1:0"
	srv := startServeWith(t, cfg, true)
	endpoint := "http://" + srv.addrs[config.Node] + mcpnode.Path

	out := mcpWhoamiChecked(t, endpoint, nil)
	u := out.Update
	if u.State != upgrade.StateOK || u.Version != "v0.1.0" || u.Latest != "v0.2.0" || !u.UpdateAvailable ||
		u.SelfUpgrade || u.Method != upgrade.MethodAdmin ||
		u.Command != "sudo kephalaion upgrade && sudo systemctl restart kephalaion" || u.CheckedAt != testNow.Format("2006-01-02T15:04:05Z") {
		t.Errorf("update: %+v", u)
	}
	for i := 0; i < 5; i++ {
		mcpWhoami(t, endpoint, nil)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("%d Anfragen an GitHub, erwartet 1", n)
	}
	eventually(t, "Logzeile zum Update", func() bool {
		return contains(srv.log.String(), "Update: v0.2.0 verfügbar (installiert v0.1.0")
	})

	// node whoami fragt selbst, aus seiner Sicht: ohne globale config der
	// allgemeine Weg.
	runT(t, "node", "whoami").want(t, 0, "kephalaion dev\nUpdate: v0.2.0 verfügbar (installiert v0.1.0",
		"Weg: kein Schreibrecht in "+binDir)
	if n := requests.Load(); n != 2 {
		t.Errorf("%d Anfragen an GitHub nach node whoami, erwartet 2", n)
	}
}
