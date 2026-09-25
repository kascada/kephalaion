// Package upgrade ersetzt das laufende Binary durch das Binary eines Releases
// von GitHub: Version ermitteln, Asset und SHA256SUMS laden, Prüfsumme prüfen,
// atomar austauschen. Jeder Fehler lässt das alte Binary unberührt.
package upgrade

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kascada/kephalaion/internal/buildinfo"
)

// TempPattern ist das Namensmuster der temporären Datei neben dem Binary, an
// dem sich Reste eines abgebrochenen Laufs erkennen lassen.
const TempPattern = ".kephalaion-upgrade-*"

// Options sind die Angaben auf der Kommandozeile.
type Options struct {
	// Check meldet nur, ob es eine neuere Version gibt.
	Check bool
	// Version nennt die Zielversion ausdrücklich; nur so wird zurückgestuft
	// und nur so ein dev build ersetzt.
	Version string
}

// Upgrader trägt, woher Releases kommen und welches Binary ersetzt wird. Die
// Felder sind für Tests überschreibbar; New setzt die echten Werte.
type Upgrader struct {
	// APIBase ist die Basis-URL der GitHub-API, ohne Schrägstrich am Ende.
	APIBase string
	// Repo ist owner/name.
	Repo   string
	Client *http.Client
	// Current beschreibt das laufende Binary.
	Current buildinfo.Info
	// Executable liefert den Pfad des zu ersetzenden Binarys.
	Executable func() (string, error)
	Out        io.Writer
}

// New liefert einen Upgrader für dieses Binary und github.com/kascada/kephalaion.
func New(out io.Writer) *Upgrader {
	return &Upgrader{
		APIBase:    "https://api.github.com",
		Repo:       "kascada/kephalaion",
		Client:     &http.Client{Timeout: 10 * time.Minute},
		Current:    buildinfo.Get(),
		Executable: os.Executable,
		Out:        out,
	}
}

var errAborted = errors.New("abgebrochen")

// unchanged hängt an jede Fehlermeldung, dass nichts passiert ist.
func unchanged(err error) error {
	return fmt.Errorf("%w\nDas installierte Binary bleibt unverändert.", err)
}

// Run führt `kephalaion upgrade` aus.
func (u *Upgrader) Run(ctx context.Context, opts Options) error {
	if err := u.run(ctx, opts); err != nil {
		return unchanged(err)
	}
	return nil
}

func (u *Upgrader) run(ctx context.Context, opts Options) error {
	current, currentErr := ParseVersion(u.Current.Version)
	isDev := u.Current.IsDev() || currentErr != nil

	var explicit Version
	if opts.Version != "" {
		v, err := ParseVersion(opts.Version)
		if err != nil {
			return err
		}
		explicit = v
	}

	if isDev && opts.Version == "" && !opts.Check {
		return fmt.Errorf("dieses Binary ist ein dev build (%s) und wird nur mit ausdrücklicher --version ersetzt, z. B.:\n  kephalaion upgrade --version vX.Y.Z", u.Current.Version)
	}

	rel, err := u.fetchRelease(ctx, opts.Version)
	if errors.Is(err, errNotFound) {
		if opts.Version != "" {
			return fmt.Errorf("ein Release %s gibt es nicht", opts.Version)
		}
		return errors.New("es gibt noch kein veröffentlichtes Release")
	}
	if err != nil {
		return err
	}
	target, err := ParseVersion(rel.TagName)
	if err != nil {
		return fmt.Errorf("das Release trägt keinen gültigen Versions-Tag: %w", err)
	}
	if opts.Version != "" && target.Compare(explicit) != 0 {
		return fmt.Errorf("verlangt war %s, GitHub liefert %s", explicit, target)
	}
	// releases/latest liefert nie eine Vorabversion; darauf verlassen wir
	// uns nicht, denn latest ist hier als „ohne Suffix“ festgelegt.
	if opts.Version == "" && (target.IsPrerelease() || rel.Prerelease) {
		return fmt.Errorf("GitHub nennt %s als neuestes Release, das ist aber eine Vorabversion", target)
	}

	switch {
	case isDev:
		if opts.Check && opts.Version == "" {
			fmt.Fprintf(u.Out, "Installiert: %s (dev build). Neuestes Release: %s.\n", u.Current.Version, target)
			fmt.Fprintf(u.Out, "Ersetzen nur ausdrücklich: kephalaion upgrade --version %s\n", target)
			return nil
		}
	case target.Compare(current) == 0:
		fmt.Fprintf(u.Out, "%s ist bereits installiert.\n", current)
		return nil
	case target.Compare(current) < 0 && opts.Version == "":
		fmt.Fprintf(u.Out, "Installiert ist %s, neuer als das neueste Release %s. Nichts zu tun.\n", current, target)
		fmt.Fprintf(u.Out, "Zurückstufen nur ausdrücklich: kephalaion upgrade --version %s\n", target)
		return nil
	}

	from := u.Current.Version
	if opts.Check {
		verb := "Neue Version verfügbar"
		if !isDev && target.Compare(current) < 0 {
			verb = "Zurückstufen möglich"
		}
		fmt.Fprintf(u.Out, "%s: %s → %s\n", verb, from, target)
		return nil
	}

	name := AssetName(u.Current.OS, u.Current.Arch)
	bin, ok := rel.findAsset(name)
	if !ok {
		return fmt.Errorf("das Release %s hat kein Binary für %s (%s fehlt)", target, u.Current.Platform(), name)
	}
	sumsAsset, ok := rel.findAsset(SumsAsset)
	if !ok {
		return fmt.Errorf("das Release %s hat kein %s", target, SumsAsset)
	}

	want, err := u.expectedSum(ctx, sumsAsset, name)
	if err != nil {
		return err
	}

	exe, err := u.executablePath()
	if err != nil {
		return err
	}
	if err := u.replace(ctx, exe, bin, want); err != nil {
		return err
	}

	if !isDev && target.Compare(current) < 0 {
		fmt.Fprintf(u.Out, "Zurückgestuft: %s → %s (%s)\n", from, target, exe)
	} else {
		fmt.Fprintf(u.Out, "Aktualisiert: %s → %s (%s)\n", from, target, exe)
	}
	return nil
}

// expectedSum lädt SHA256SUMS und liefert die Summe für name.
func (u *Upgrader) expectedSum(ctx context.Context, sums asset, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sums.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := u.do(req)
	if err != nil {
		return "", fmt.Errorf("%s nicht ladbar: %w", SumsAsset, err)
	}
	defer resp.Body.Close()
	sum, err := findSum(io.LimitReader(resp.Body, 64<<10), name)
	if err != nil {
		return "", err
	}
	return sum, nil
}

// findSum liest eine Datei im Format von sha256sum und liefert die Summe für
// name. Ein * vor dem Namen (Binärmodus) wird angenommen.
func findSum(r io.Reader, name string) (string, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum := strings.ToLower(fields[0])
		if b, err := hex.DecodeString(sum); err != nil || len(b) != sha256.Size {
			return "", fmt.Errorf("%s enthält für %s keine gültige SHA-256-Summe", SumsAsset, name)
		}
		return sum, nil
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("%s nicht lesbar: %w", SumsAsset, err)
	}
	return "", fmt.Errorf("%s enthält keine Summe für %s", SumsAsset, name)
}

// executablePath liefert den Pfad des laufenden Binarys mit aufgelösten
// Links: ersetzt wird die Datei, nicht der Link, der auf sie zeigt.
func (u *Upgrader) executablePath() (string, error) {
	exe, err := u.Executable()
	if err != nil {
		return "", fmt.Errorf("Pfad des laufenden Binarys nicht ermittelbar: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("Pfad des laufenden Binarys nicht auflösbar (%s): %w", exe, err)
	}
	return resolved, nil
}

// replace lädt das Binary in eine temporäre Datei neben exe, prüft die Summe
// und benennt sie über exe um. rename ist atomar, weil beide Dateien im selben
// Verzeichnis und damit auf demselben Dateisystem liegen. Scheitert irgendein
// Schritt oder wird ctx abgebrochen (SIGINT/SIGTERM), wird die temporäre Datei
// entfernt und exe bleibt, wie es war.
func (u *Upgrader) replace(ctx context.Context, exe string, bin asset, wantSum string) (err error) {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, TempPattern)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("kein Schreibrecht im Verzeichnis %s — das Binary dort lässt sich nicht ersetzen", dir)
		}
		return fmt.Errorf("temporäre Datei in %s nicht anlegbar: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bin.URL, nil)
	if err != nil {
		return err
	}
	resp, err := u.do(req)
	if err != nil {
		return fmt.Errorf("%s nicht ladbar: %w", bin.Name, err)
	}
	defer resp.Body.Close()

	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		if ctx.Err() != nil {
			return errAborted
		}
		return fmt.Errorf("Download von %s abgebrochen: %w", bin.Name, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantSum {
		return fmt.Errorf("Prüfsumme von %s stimmt nicht: erwartet %s, erhalten %s", bin.Name, wantSum, got)
	}
	if err = tmp.Chmod(0o755); err != nil {
		return fmt.Errorf("Rechte der temporären Datei nicht setzbar: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("temporäre Datei nicht schreibbar: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("temporäre Datei nicht schreibbar: %w", err)
	}
	// Letzte Gelegenheit für einen Abbruch; nach rename ist das neue Binary da.
	if ctx.Err() != nil {
		return errAborted
	}
	if err = os.Rename(tmpName, exe); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("kein Schreibrecht, um %s zu ersetzen", exe)
		}
		return fmt.Errorf("%s nicht ersetzbar: %w", exe, err)
	}
	return nil
}
