package upgrade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kephalaion/kephalaion/internal/buildinfo"
)

const testOS, testArch = "linux", "amd64"

// fakeGitHub spielt API und Downloads von GitHub nach. releases ordnet einem
// Tag seine Assets zu; latest ist der Tag, den releases/latest liefert.
type fakeGitHub struct {
	t        *testing.T
	srv      *httptest.Server
	releases map[string]map[string][]byte
	latest   string
	requests atomic.Int32
	// override beantwortet eine Anfrage selbst, wenn es true liefert.
	override func(w http.ResponseWriter, r *http.Request) bool
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{t: t, releases: map[string]map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// addRelease legt ein Release mit Binary für die Testplattform und passender
// SHA256SUMS an.
func (f *fakeGitHub) addRelease(tag string, binary []byte) {
	name := AssetName(testOS, testArch)
	sum := sha256.Sum256(binary)
	f.releases[tag] = map[string][]byte{
		name:      binary,
		SumsAsset: []byte(fmt.Sprintf("%s  %s\n%s  kephalaion-darwin-arm64\n", hex.EncodeToString(sum[:]), name, strings.Repeat("0", 64))),
	}
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)
	if f.override != nil && f.override(w, r) {
		return
	}
	const prefix = "/repos/kephalaion/kephalaion/releases/"
	switch {
	case r.URL.Path == prefix+"latest":
		f.writeRelease(w, f.latest)
	case strings.HasPrefix(r.URL.Path, prefix+"tags/"):
		f.writeRelease(w, strings.TrimPrefix(r.URL.Path, prefix+"tags/"))
	case strings.HasPrefix(r.URL.Path, "/download/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/download/"), "/")
		data, ok := f.releases[parts[0]][parts[1]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeGitHub) writeRelease(w http.ResponseWriter, tag string) {
	assets, ok := f.releases[tag]
	if !ok {
		http.NotFound(w, nil)
		return
	}
	rel := release{TagName: tag, Prerelease: strings.Contains(tag, "-")}
	for name := range assets {
		rel.Assets = append(rel.Assets, asset{Name: name, URL: f.srv.URL + "/download/" + tag + "/" + name})
	}
	_ = json.NewEncoder(w).Encode(rel)
}

// installed legt ein „installiertes“ Binary in einem temporären Verzeichnis an.
func installed(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "kephalaion")
	if err := os.WriteFile(exe, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func (f *fakeGitHub) upgrader(version, exe string) (*Upgrader, *bytes.Buffer) {
	var out bytes.Buffer
	return &Upgrader{
		APIBase:    f.srv.URL,
		Repo:       "kephalaion/kephalaion",
		Client:     f.srv.Client(),
		Current:    buildinfo.Info{Version: version, OS: testOS, Arch: testArch},
		Executable: func() (string, error) { return exe, nil },
		Out:        &out,
	}, &out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// assertNoTemp prüft, dass keine temporäre Datei zurückgeblieben ist.
func assertNoTemp(t *testing.T, dir string) {
	t.Helper()
	left, err := filepath.Glob(filepath.Join(dir, TempPattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Fatalf("temporäre Dateien zurückgeblieben: %v", left)
	}
}

func assertUnchanged(t *testing.T, exe string) {
	t.Helper()
	if got := readFile(t, exe); got != "alt" {
		t.Fatalf("Binary verändert: %q", got)
	}
	assertNoTemp(t, filepath.Dir(exe))
}

func TestVersionCompare(t *testing.T) {
	ordered := []string{
		"v0.1.0-alpha", "v0.1.0-alpha.1", "v0.1.0-alpha.beta", "v0.1.0-beta",
		"v0.1.0-beta.2", "v0.1.0-beta.11", "v0.1.0-rc.1", "v0.1.0", "v0.1.1",
		"v0.2.0-rc1", "v0.2.0", "v0.10.0", "v1.0.0",
	}
	for i := range ordered {
		for j := range ordered {
			a, err := ParseVersion(ordered[i])
			if err != nil {
				t.Fatal(err)
			}
			b, err := ParseVersion(ordered[j])
			if err != nil {
				t.Fatal(err)
			}
			want := sign(i - j)
			if got := a.Compare(b); got != want {
				t.Errorf("%s vs %s: %d, erwartet %d", a, b, got, want)
			}
		}
	}
}

func TestParseVersionRejects(t *testing.T) {
	for _, s := range []string{"", "dev", "0.1.0", "v0.1", "v0.1.0.1", "v01.1.0", "v0.1.0-", "v0.1.0-a..b", "v0.1.0+meta", "v0.1.x"} {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("%q angenommen, erwartet Fehler", s)
		}
	}
	v, err := ParseVersion("v0.2.0-rc1")
	if err != nil || !v.IsPrerelease() {
		t.Fatalf("v0.2.0-rc1: %v, prerelease=%v", err, v.IsPrerelease())
	}
}

func TestAssetSelection(t *testing.T) {
	rel := release{Assets: []asset{
		{Name: "kephalaion-linux-arm64"}, {Name: "kephalaion-linux-amd64"},
		{Name: "kephalaion-darwin-arm64"}, {Name: SumsAsset}, {Name: "install.sh"},
	}}
	for _, p := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "arm64"}} {
		got, ok := rel.findAsset(AssetName(p[0], p[1]))
		if !ok || got.Name != "kephalaion-"+p[0]+"-"+p[1] {
			t.Errorf("%s/%s: %v %v", p[0], p[1], got, ok)
		}
	}
	if _, ok := rel.findAsset(AssetName("darwin", "amd64")); ok {
		t.Error("darwin/amd64 gefunden, obwohl es fehlt")
	}
}

func TestFindSum(t *testing.T) {
	a, b := strings.Repeat("a", 64), strings.Repeat("B", 64)
	sums := a + "  kephalaion-linux-amd64\n" + b + " *kephalaion-darwin-arm64\n"
	if got, err := findSum(strings.NewReader(sums), "kephalaion-linux-amd64"); err != nil || got != a {
		t.Errorf("linux-amd64: %q %v", got, err)
	}
	if got, err := findSum(strings.NewReader(sums), "kephalaion-darwin-arm64"); err != nil || got != strings.ToLower(b) {
		t.Errorf("darwin-arm64 (Binärmodus): %q %v", got, err)
	}
	if _, err := findSum(strings.NewReader(sums), "kephalaion-linux-arm64"); err == nil {
		t.Error("fehlende Summe nicht gemeldet")
	}
	if _, err := findSum(strings.NewReader("xyz  kephalaion-linux-amd64\n"), "kephalaion-linux-amd64"); err == nil {
		t.Error("ungültige Summe nicht gemeldet")
	}
}

func TestUpgradeReplacesAtomically(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.1.0", []byte("eins"))
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, out := f.upgrader("v0.1.0", exe)

	if err := u.Run(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != "zwei" {
		t.Fatalf("Inhalt %q, erwartet zwei", got)
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("Rechte %v, erwartet 0755", info.Mode().Perm())
	}
	assertNoTemp(t, filepath.Dir(exe))
	if !strings.Contains(out.String(), "Aktualisiert: v0.1.0 → v0.2.0") {
		t.Errorf("Ausgabe: %s", out.String())
	}
}

func TestUpgradeResolvesSymlink(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	link := filepath.Join(t.TempDir(), "kephalaion")
	if err := os.Symlink(exe, link); err != nil {
		t.Skip("Symlinks nicht verfügbar:", err)
	}
	u, _ := f.upgrader("v0.1.0", link)

	if err := u.Run(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != "zwei" {
		t.Fatalf("Ziel des Links: %q, erwartet zwei", got)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Link ersetzt statt Ziel: %v %v", fi, err)
	}
}

func TestUpgradeChecksumMismatch(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.releases["v0.2.0"][AssetName(testOS, testArch)] = []byte("manipuliert")
	f.latest = "v0.2.0"
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)

	err := u.Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "Prüfsumme") {
		t.Fatalf("Fehler %v, erwartet Prüfsumme", err)
	}
	if !strings.Contains(err.Error(), "bleibt unverändert") {
		t.Errorf("Meldung ohne Hinweis auf das alte Binary: %v", err)
	}
	assertUnchanged(t, exe)
}

func TestUpgradeSameVersion(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, out := f.upgrader("v0.2.0", exe)

	if err := u.Run(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, exe)
	if !strings.Contains(out.String(), "bereits installiert") {
		t.Errorf("Ausgabe: %s", out.String())
	}
}

func TestUpgradeNoAutomaticDowngrade(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, out := f.upgrader("v0.3.0-rc1", exe)

	if err := u.Run(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, exe)
	if !strings.Contains(out.String(), "neuer als das neueste Release") {
		t.Errorf("Ausgabe: %s", out.String())
	}
}

func TestUpgradeExplicitDowngrade(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.1.0", []byte("eins"))
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, out := f.upgrader("v0.2.0", exe)

	if err := u.Run(context.Background(), Options{Version: "v0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != "eins" {
		t.Fatalf("Inhalt %q, erwartet eins", got)
	}
	if !strings.Contains(out.String(), "Zurückgestuft") {
		t.Errorf("Ausgabe: %s", out.String())
	}
}

func TestUpgradeCheckOnly(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, out := f.upgrader("v0.1.0", exe)

	if err := u.Run(context.Background(), Options{Check: true}); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, exe)
	if !strings.Contains(out.String(), "Neue Version verfügbar: v0.1.0 → v0.2.0") {
		t.Errorf("Ausgabe: %s", out.String())
	}
}

func TestUpgradeDevBuildNeedsVersion(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, _ := f.upgrader(buildinfo.DevVersion, exe)

	err := u.Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "dev build") {
		t.Fatalf("Fehler %v, erwartet Hinweis auf dev build", err)
	}
	if n := f.requests.Load(); n != 0 {
		t.Errorf("%d Anfragen an GitHub, erwartet keine", n)
	}
	assertUnchanged(t, exe)

	if err := u.Run(context.Background(), Options{Version: "v0.2.0"}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != "zwei" {
		t.Fatalf("mit --version: Inhalt %q, erwartet zwei", got)
	}
}

func TestUpgradeDevBuildCheck(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, out := f.upgrader(buildinfo.DevVersion, exe)

	if err := u.Run(context.Background(), Options{Check: true}); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, exe)
	if !strings.Contains(out.String(), "--version v0.2.0") {
		t.Errorf("Ausgabe: %s", out.String())
	}
}

func TestUpgradePrerelease(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.3.0-rc1", []byte("rc"))
	f.latest = "v0.3.0-rc1" // GitHub täte das nicht; upgrade verlässt sich nicht darauf
	exe := installed(t)
	u, _ := f.upgrader("v0.2.0", exe)

	if err := u.Run(context.Background(), Options{}); err == nil || !strings.Contains(err.Error(), "Vorabversion") {
		t.Fatalf("Fehler %v, erwartet Vorabversion", err)
	}
	assertUnchanged(t, exe)

	if err := u.Run(context.Background(), Options{Version: "v0.3.0-rc1"}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, exe); got != "rc" {
		t.Fatalf("Inhalt %q, erwartet rc", got)
	}
}

func TestUpgradeUnknownVersion(t *testing.T) {
	f := newFakeGitHub(t)
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)

	err := u.Run(context.Background(), Options{Version: "v9.9.9"})
	if err == nil || !strings.Contains(err.Error(), "gibt es nicht") {
		t.Fatalf("Fehler %v", err)
	}
	if err := u.Run(context.Background(), Options{}); err == nil || !strings.Contains(err.Error(), "kein veröffentlichtes Release") {
		t.Fatalf("ohne Release: %v", err)
	}
	if err := u.Run(context.Background(), Options{Version: "0.1.0"}); err == nil || !strings.Contains(err.Error(), "keine gültige Version") {
		t.Fatalf("ungültige Version: %v", err)
	}
	assertUnchanged(t, exe)
}

func TestUpgradeMissingPlatformAsset(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	delete(f.releases["v0.2.0"], AssetName(testOS, testArch))
	f.latest = "v0.2.0"
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)

	err := u.Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "kein Binary für linux/amd64") {
		t.Fatalf("Fehler %v", err)
	}
	assertUnchanged(t, exe)
}

func TestUpgradeRateLimit(t *testing.T) {
	for _, c := range []struct {
		status  int
		headers map[string]string
	}{
		{http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "4102444800"}},
		{http.StatusTooManyRequests, map[string]string{"Retry-After": "60"}},
	} {
		f := newFakeGitHub(t)
		f.override = func(w http.ResponseWriter, r *http.Request) bool {
			for k, v := range c.headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(c.status)
			return true
		}
		exe := installed(t)
		u, _ := f.upgrader("v0.1.0", exe)

		err := u.Run(context.Background(), Options{})
		if err == nil || !strings.Contains(err.Error(), "Anfrage-Limit der GitHub-API") {
			t.Fatalf("%d: Fehler %v", c.status, err)
		}
		assertUnchanged(t, exe)
	}
}

func TestUpgradeForbiddenWithoutRateLimit(t *testing.T) {
	f := newFakeGitHub(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		w.WriteHeader(http.StatusForbidden)
		return true
	}
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)
	if err := u.Run(context.Background(), Options{}); err == nil || !strings.Contains(err.Error(), "verweigert") {
		t.Fatalf("Fehler %v", err)
	}
	assertUnchanged(t, exe)
}

func TestUpgradeNetworkDown(t *testing.T) {
	f := newFakeGitHub(t)
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)
	f.srv.Close() // niemand mehr am anderen Ende

	err := u.Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "keine Verbindung") {
		t.Fatalf("Fehler %v", err)
	}
	assertUnchanged(t, exe)
}

func TestUpgradeNoWritePermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("als root greifen Dateirechte nicht")
	}
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	exe := installed(t)
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	u, _ := f.upgrader("v0.1.0", exe)

	err := u.Run(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "kein Schreibrecht") {
		t.Fatalf("Fehler %v", err)
	}
	assertUnchanged(t, exe)
}

// Ein Abbruch mitten im Download (wie bei SIGINT/SIGTERM, das main in einen
// abgebrochenen Kontext übersetzt) räumt die temporäre Datei weg.
func TestUpgradeAbortCleansUp(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasSuffix(r.URL.Path, "/"+AssetName(testOS, testArch)) {
			return false
		}
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1000))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		return true
	}
	exe := installed(t)
	u, _ := f.upgrader("v0.1.0", exe)

	done := make(chan error, 1)
	go func() { done <- u.Run(ctx, Options{}) }()
	<-started
	// Die temporäre Datei existiert jetzt.
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), TempPattern)); len(left) != 1 {
		t.Fatalf("während des Downloads %d temporäre Dateien, erwartet 1", len(left))
	}
	cancel()
	err := <-done
	if err == nil || !strings.Contains(err.Error(), "abgebrochen") {
		t.Fatalf("Fehler %v, erwartet abgebrochen", err)
	}
	assertUnchanged(t, exe)
}
