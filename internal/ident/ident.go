// Package ident hält, was Hub und Node beide über Namen und Token wissen: die
// Namensregel für Collections, Nodes und Hub-Aliase, die Adresse
// <hub>:<collection>, das Token-Format keph_…, seinen Hash und die gekürzte
// Anzeige. Es kennt weder Hub noch Node und steht beiden offen, später auch
// den Accounts.
package ident

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// namePattern ist die Namensregel: klein, beginnt mit Buchstabe oder Ziffer,
// höchstens 63 Zeichen, kein Doppelpunkt.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// reservedPrefix darf kein Name tragen, in keiner Schreibweise: Er ist für
// SYSTEM:-Zeilen reserviert.
const reservedPrefix = "system"

// CheckName prüft einen Namen gegen die Namensregel. what nennt die Art im
// Fehler, etwa „Collection“ oder „Node“.
func CheckName(what, name string) error {
	if name == "" {
		return fmt.Errorf("%s: Name fehlt", what)
	}
	if strings.HasPrefix(strings.ToLower(name), reservedPrefix) {
		return fmt.Errorf("%s %q: Namen mit dem Präfix %q sind reserviert", what, name, reservedPrefix)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%s %q: ungültiger Name; erlaubt sind a–z, 0–9, '.', '_' und '-', "+
			"am Anfang a–z oder 0–9, höchstens 63 Zeichen", what, name)
	}
	return nil
}

// ParseAddress zerlegt eine Adresse <hub>:<collection> und prüft beide Teile
// gegen die Namensregel.
func ParseAddress(addr string) (hub, collection string, err error) {
	hub, collection, ok := strings.Cut(addr, ":")
	if !ok {
		return "", "", fmt.Errorf("Adresse %q: erwartet <hub>:<collection>", addr)
	}
	if err := CheckName("Hub", hub); err != nil {
		return "", "", fmt.Errorf("Adresse %q: %w", addr, err)
	}
	if err := CheckName("Collection", collection); err != nil {
		return "", "", fmt.Errorf("Adresse %q: %w", addr, err)
	}
	return hub, collection, nil
}

// Address liefert die Adresse <hub>:<collection>.
func Address(hub, collection string) string { return hub + ":" + collection }

// TokenPrefix steht vor jedem Token; Scanner wie gitleaks erkennen daran ein
// eingechecktes Token.
const TokenPrefix = "keph_"

// tokenBytes ist die Länge des Geheimnisses: 256 Bit.
const tokenBytes = 32

var tokenEncoding = base64.RawURLEncoding

// NewToken erzeugt ein Token: keph_ und 32 Zufallsbytes, base64url ohne
// Padding.
func NewToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("Token erzeugen: %w", err)
	}
	return TokenPrefix + tokenEncoding.EncodeToString(b), nil
}

// CheckToken prüft das Format eines Tokens. Der Fehler nennt das Token nicht.
func CheckToken(token string) error {
	secret, ok := strings.CutPrefix(token, TokenPrefix)
	if !ok {
		return fmt.Errorf("ungültiges Token: beginnt nicht mit %s", TokenPrefix)
	}
	b, err := tokenEncoding.Strict().DecodeString(secret)
	if err != nil || len(b) != tokenBytes {
		return fmt.Errorf("ungültiges Token: nach %s werden %d Bytes base64url ohne Padding erwartet", TokenPrefix, tokenBytes)
	}
	return nil
}

// HashToken liefert sha256(token) als Hex, so wie es gespeichert wird.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MaskToken liefert die gekürzte Anzeige eines Tokens: keph_… und die letzten
// vier Zeichen. Ein leeres Token bleibt leer.
func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= len(TokenPrefix)+4 {
		return TokenPrefix + "…"
	}
	return TokenPrefix + "…" + token[len(token)-4:]
}
