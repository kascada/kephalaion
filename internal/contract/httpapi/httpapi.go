// Package httpapi ist der Vertrag (internal/contract) über HTTP mit JSON: ein
// Handler, der eine beliebige Umsetzung von contract.Hub bedient, und ein
// Client, der contract.Hub über HTTP umsetzt. Beide folgen docs/vertrag.md,
// „HTTP“: POST auf /v<Fassung>/<Vorgang>, der Node meldet sich in Headern an,
// Fehler als HTTP-Status plus {"code", "message"}, Antworten gzip, wenn
// erbeten.
//
// Das Paket ist neutral: Es kennt weder Hub noch Node. Welche Umsetzung ein
// Node bekommt und was der Hub hinter den Handler stellt, entscheidet
// cmd/kephalaion.
package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kephalaion/kephalaion/internal/contract"
)

// HeaderNode trägt den Namen des Nodes; sein Token steht in Authorization:
// Bearer <token>.
const HeaderNode = "X-Keph-Node"

// Die Vorgänge, je ein Pfad /v<Fassung>/<Vorgang>.
const (
	OpWhoami = "whoami"
	OpRotate = "rotate"
	OpSync   = "sync"
)

// MaxBodyBytes begrenzt den Body einer Anfrage: 1 MiB. Antworten sind nicht
// begrenzt — eine Seite des Abgleichs kann eine große Revision ganz tragen.
const MaxBodyBytes = 1 << 20

// Zeitlimits. Der Server liest Kopf und Body in kurzer Zeit, schreibt eine
// Antwort aber so lange, wie eine große Seite braucht.
const (
	ReadHeaderTimeout = 10 * time.Second
	ReadTimeout       = 60 * time.Second
	WriteTimeout      = 10 * time.Minute
	IdleTimeout       = 2 * time.Minute
	// ShortTimeout gilt am Client für whoami und rotate, SyncTimeout für
	// eine Seite des Abgleichs.
	ShortTimeout = 30 * time.Second
	SyncTimeout  = 10 * time.Minute
)

// CodeInternal steht in der Antwort auf einen Fehler, der kein Fehler des
// Vertrags ist (Datenbank). Er ist kein contract.Code: Der Node behandelt ihn
// wie einen Fehler des Transports.
const CodeInternal = "internal"

// Status liefert den HTTP-Status zu einem Fehlercode des Vertrags.
func Status(c contract.Code) int {
	switch c {
	case contract.CodeUnauthenticated:
		return http.StatusUnauthorized
	case contract.CodeAccountUnauthenticated:
		return http.StatusForbidden
	case contract.CodeUnsupportedVersion:
		return http.StatusNotFound
	case contract.CodeNoSharedCollection:
		return http.StatusConflict
	case contract.CodeInvalid:
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// Path liefert den Pfad eines Vorgangs in einer Fassung: /v1/sync.
func Path(version int, op string) string {
	return "/v" + strconv.Itoa(version) + "/" + op
}

// parsePath zerlegt /v<n>/<op>; n ist eine ganze Zahl, auch eine, die keine
// Fassung ist (0, -1) — die beantwortet der Handler mit
// unsupported_version. ok ist false, wenn der Pfad nicht so aussieht.
func parsePath(p string) (version int, op string, ok bool) {
	rest, found := strings.CutPrefix(p, "/v")
	if !found {
		return 0, "", false
	}
	num, op, found := strings.Cut(rest, "/")
	if !found || op == "" || strings.Contains(op, "/") {
		return 0, "", false
	}
	digits := strings.TrimPrefix(num, "-")
	if digits == "" || len(digits) > 9 {
		return 0, "", false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, "", false
		}
	}
	version, err := strconv.Atoi(num)
	if err != nil {
		return 0, "", false
	}
	return version, op, true
}
