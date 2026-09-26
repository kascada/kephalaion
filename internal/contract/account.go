package contract

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// AccountRowPrefix steht vor dem Namen der Zeile eines Accounts in
// documents: SYSTEM:A:<account>, je Account und Collection eine Zeile. Die
// Zeilen gleichen sich wie jede andere ab; der Node prüft seine Clients
// gegen sie.
const AccountRowPrefix = "SYSTEM:A:"

// AccountRowName liefert den Namen der Zeile eines Accounts.
func AccountRowName(account string) string { return AccountRowPrefix + account }

// AccountOfRow liefert den Account einer Zeile, oder ok false, wenn der Name
// keine Account-Zeile ist.
func AccountOfRow(name string) (account string, ok bool) {
	account, ok = strings.CutPrefix(name, AccountRowPrefix)
	return account, ok && account != ""
}

// Rights sind die Rechte eines Accounts in einer Collection über read hinaus;
// read ergibt sich aus der Zeile selbst. write und supersede sind unabhängig:
// write betrifft Eigenes, supersede Fremdes.
type Rights struct {
	Write     bool `json:"write"`
	Supersede bool `json:"supersede"`
}

// String nennt die Rechte, read immer zuerst: „read, write“.
func (r Rights) String() string {
	out := []string{"read"}
	if r.Write {
		out = append(out, "write")
	}
	if r.Supersede {
		out = append(out, "supersede")
	}
	return strings.Join(out, ", ")
}

// AccountContent ist der Inhalt einer Account-Zeile: der Hash des Tokens
// (sha256 in Hex) und die Rechte in dieser Collection. Der Hub führt den Hash
// maßgeblich in seiner Tabelle accounts; die Zeile trägt eine Kopie.
type AccountContent struct {
	Hash   string `json:"hash"`
	Rights Rights `json:"rights"`
}

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// IsTokenHash sagt, ob h ein sha256 in Hex ist, wie Token-Hashes gespeichert
// werden (64 Zeichen 0-9a-f).
func IsTokenHash(h string) bool { return hashPattern.MatchString(h) }

// EncodeAccountContent liefert den Inhalt einer Account-Zeile als JSON-Text,
// immer in derselben Form.
func EncodeAccountContent(c AccountContent) (string, error) {
	if !IsTokenHash(c.Hash) {
		return "", fmt.Errorf("Account-Zeile: Hash ist kein sha256 in Hex")
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeAccountContent liest den Inhalt einer Account-Zeile und prüft den
// Hash. Der Fehler nennt den Inhalt nicht.
func DecodeAccountContent(s string) (AccountContent, error) {
	var c AccountContent
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return AccountContent{}, fmt.Errorf("Account-Zeile: Inhalt ist kein gültiges JSON")
	}
	if !IsTokenHash(c.Hash) {
		return AccountContent{}, fmt.Errorf("Account-Zeile: Hash ist kein sha256 in Hex")
	}
	return c, nil
}
