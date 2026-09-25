package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// release ist der Ausschnitt einer GitHub-Release-Antwort, den upgrade braucht.
type release struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// findAsset sucht ein Asset nach seinem Namen.
func (r release) findAsset(name string) (asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return asset{}, false
}

// AssetName ist der Name des Binarys einer Plattform im Release.
func AssetName(goos, goarch string) string {
	return "kephalaion-" + goos + "-" + goarch
}

// SumsAsset ist der Name der Prüfsummendatei im Release.
const SumsAsset = "SHA256SUMS"

// errNotFound meldet eine 404-Antwort; der Aufrufer weiß, was fehlt.
var errNotFound = errors.New("nicht gefunden")

// fetchRelease holt ein Release über die GitHub-API: das neueste ohne Suffix
// (tag == "") oder das zum Tag.
func (u *Upgrader) fetchRelease(ctx context.Context, tag string) (release, error) {
	path := "/repos/" + u.Repo + "/releases/latest"
	if tag != "" {
		path = "/repos/" + u.Repo + "/releases/tags/" + url.PathEscape(tag)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.APIBase+path, nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := u.do(req)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()

	var r release
	// Eine Release-Antwort ist wenige Kilobyte groß; mehr ist kein Release.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&r); err != nil {
		return release{}, fmt.Errorf("Antwort der GitHub-API nicht lesbar: %w", err)
	}
	return r, nil
}

// do führt eine Anfrage aus und übersetzt Netz- und HTTP-Fehler in Meldungen,
// die sagen, was los ist. Bei Erfolg (2xx) gehört der Body dem Aufrufer.
func (u *Upgrader) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "kephalaion/"+u.Current.Version)
	resp, err := u.Client.Do(req)
	if err != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, errAborted
		}
		return nil, netError(req.URL, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, errNotFound
	case isRateLimited(resp):
		return nil, rateLimitError(resp)
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("GitHub verweigert den Zugriff auf %s (403)", req.URL.Redacted())
	default:
		return nil, fmt.Errorf("unerwartete Antwort von %s: %s", req.URL.Host, resp.Status)
	}
}

// isRateLimited erkennt das Rate-Limit der GitHub-API: 429, oder 403 mit
// erschöpftem Kontingent bzw. Retry-After (sekundäres Limit).
func isRateLimited(resp *http.Response) bool {
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	return resp.StatusCode == http.StatusForbidden &&
		(resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "")
}

func rateLimitError(resp *http.Response) error {
	msg := "Das Anfrage-Limit der GitHub-API ist erreicht (ohne Anmeldung 60 Anfragen je Stunde)."
	if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil && reset > 0 {
		msg += " Wieder möglich ab " + time.Unix(reset, 0).Local().Format("15:04") + " Uhr."
	} else if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
		msg += fmt.Sprintf(" Wieder möglich in %d Sekunden.", secs)
	} else {
		msg += " Später erneut versuchen."
	}
	return errors.New(msg)
}

func netError(u *url.URL, err error) error {
	var dnsErr *net.DNSError
	var opErr *net.OpError
	if errors.As(err, &dnsErr) || errors.As(err, &opErr) {
		return fmt.Errorf("keine Verbindung zu %s — ist das Netz erreichbar? (%v)", u.Host, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("Zeitüberschreitung bei %s (%v)", u.Host, err)
	}
	return fmt.Errorf("Anfrage an %s fehlgeschlagen: %v", u.Host, err)
}
