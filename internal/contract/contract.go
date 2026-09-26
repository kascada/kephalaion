// Package contract ist der Vertrag zwischen Node und Hub (docs/vertrag.md)
// als Go-Typen: Anfragen, Antworten, Fehler und die Schnittstelle Hub. Das
// Paket ist neutral — es kennt weder internal/hub noch internal/node. Der Hub
// setzt die Schnittstelle um, der Node benutzt sie; welche Umsetzung er
// bekommt (local oder später HTTP), entscheidet cmd/kephalaion.
//
// Die Typen sind so geschnitten, dass sie unverändert als JSON laufen. Was
// über HTTP nicht im Body steht — die Fassung (im Pfad) und die Anmeldung des
// Nodes (in Headern) —, trägt das Tag json:"-".
package contract

import (
	"context"
	"errors"
)

// Version ist die Fassung des Vertrags, die dieses Binary spricht. Fassung 1
// ist die einzige.
const Version = 1

// DefaultPageSize ist die Seitengröße, mit der ein Node fragt, wenn nichts
// anderes eingestellt ist: Zeilen je Seite. Keine Nutzeroption; Tests fragen
// mit eigener Seitengröße.
const DefaultPageSize = 500

// Hub ist, was ein Node vom Hub braucht. Jeder Aufruf trägt die Anmeldung des
// Nodes und wird am Hub geprüft, auch auf dem lokalen Weg. Fehler sind *Error
// mit einem der Codes dieses Pakets oder Fehler des Transports bzw. der
// Datenbank.
type Hub interface {
	// Whoami bestätigt den Node und nennt seine erlaubten Collections; mit
	// einem Account-Teil prüft es zusätzlich den Account.
	Whoami(ctx context.Context, req WhoamiRequest) (WhoamiResponse, error)
	// Rotate ersetzt das Token eines Accounts: altes Token zur Anmeldung,
	// Hash des neuen. Nicht wiederholbar — danach gilt das alte Token nicht
	// mehr; ein Transport wiederholt es deshalb nie.
	Rotate(ctx context.Context, req RotateRequest) (RotateResponse, error)
	// Sync liefert eine Seite des Abgleichs.
	Sync(ctx context.Context, req SyncRequest) (SyncResponse, error)
}

// NodeAuth ist die Anmeldung des Nodes am Hub: sein Name dort und sein Token.
// Über HTTP steht sie in Headern, nicht im Body.
type NodeAuth struct {
	Node  string `json:"-"`
	Token string `json:"-"`
}

// AccountAuth ist die Anmeldung eines Accounts: sein Name und sein Token. Sie
// steht im Body — der Node trägt die Anfrage, der Account sagt, in wessen
// Namen.
type AccountAuth struct {
	Account string `json:"account"`
	Token   string `json:"token"`
}

// WhoamiRequest fragt den Hub, wer der Node für ihn ist, und wahlweise, ob ein
// Account gilt.
type WhoamiRequest struct {
	// Version ist die Fassung des Nodes; über HTTP im Pfad.
	Version int `json:"-"`
	// Auth ist die Anmeldung des Nodes; über HTTP in Headern.
	Auth NodeAuth `json:"-"`
	// Account ist wahlweise ein Account, den der Hub prüfen soll.
	Account *AccountAuth `json:"account,omitempty"`
}

// WhoamiResponse bestätigt den Node.
type WhoamiResponse struct {
	HubID   string `json:"hub_id"`
	Version int    `json:"version"`
	// Node ist der Name des Nodes am Hub.
	Node string `json:"node"`
	// Allowed sind die Collections, die der Node abgleichen darf, sortiert.
	Allowed []string `json:"allowed"`
	// Account ist gesetzt, wenn die Anfrage einen Account-Teil hatte.
	Account *AccountStatus `json:"account,omitempty"`
}

// AccountStatus sagt, ob ein Account gilt. Unbekannt, falsches Token und
// gesperrt sind dieselbe Antwort: Valid false, kein User, keine Collections.
type AccountStatus struct {
	Account string `json:"account"`
	Valid   bool   `json:"valid"`
	// User ist, wem der Account gehört; nur wenn Valid gilt, sonst leer.
	User string `json:"user"`
	// Collections sind die Collections des Accounts, die dieser Node
	// abgleichen darf, sortiert; nur wenn Valid gilt.
	Collections []string `json:"collections"`
}

// RotateRequest ersetzt das Token eines Accounts.
type RotateRequest struct {
	// Version ist die Fassung des Nodes; über HTTP im Pfad.
	Version int `json:"-"`
	// Auth ist die Anmeldung des Nodes, des Trägers; über HTTP in Headern.
	Auth NodeAuth `json:"-"`
	// Account ist der Name des Accounts.
	Account string `json:"account"`
	// Token ist das bisherige Token des Accounts.
	Token string `json:"token"`
	// NewHash ist sha256 des neuen Tokens, 64 Zeichen hex. Das neue Token
	// selbst verlässt den Node nie.
	NewHash string `json:"new_hash"`
}

// RotateResponse liefert die Zeilen des Accounts nach dem Wechsel,
// beschränkt auf die Collections, die der Node abgleichen darf. Den User
// nennt der Inhalt jeder Zeile (AccountContent).
type RotateResponse struct {
	HubID   string `json:"hub_id"`
	Version int    `json:"version"`
	// Rows sind die Account-Zeilen (SYSTEM:A:<account>) mit dem neuen Hash
	// und dem User, je eine erlaubte Collection des Accounts, nach
	// Collection; updated_by ist der User.
	Rows []Row `json:"rows"`
}

// Since ist ein Paar der Anfrage: alles aus Collection mit einer Revision
// größer als Since.
type Since struct {
	Collection string `json:"collection"`
	Since      int64  `json:"since"`
}

// SyncRequest ist die Anfrage des Abgleichs.
type SyncRequest struct {
	// Version ist die Fassung des Nodes; über HTTP im Pfad.
	Version int `json:"-"`
	// Auth ist die Anmeldung des Nodes; über HTTP in Headern.
	Auth NodeAuth `json:"-"`
	// Collections sind die gewünschten Collections, jede höchstens einmal.
	Collections []Since `json:"collections"`
	// PageSize ist die gewünschte Seitengröße, > 0. Der Hub begrenzt sie
	// nach oben auf eine eigene Obergrenze.
	PageSize int `json:"page_size"`
}

// CollectionStatus sagt zu einer angefragten Collection, ob der Node sie
// abgleichen darf. Unbekannt und nicht erlaubt sind dieselbe Antwort.
type CollectionStatus struct {
	Collection string `json:"collection"`
	Allowed    bool   `json:"allowed"`
}

// Row ist eine Zeile aus documents mit allen Spalten, so wie sie am Hub
// steht — auch Löschmarken und SYSTEM:-Zeilen. Nullbare Spalten sind Zeiger:
// nil heißt NULL, der Node speichert es genau so.
type Row struct {
	ID         string `json:"id"`
	Collection string `json:"collection"`
	Name       string `json:"name"`
	// Content ist nil bei einer Löschmarke.
	Content *string `json:"content"`
	// Meta ist freies JSON als Text, oder nil; der Hub deutet es nicht.
	Meta      *string `json:"meta"`
	Deleted   bool    `json:"deleted"`
	Revision  int64   `json:"revision"`
	CreatedAt int64   `json:"created_at"`
	CreatedBy string  `json:"created_by"`
	UpdatedAt int64   `json:"updated_at"`
	UpdatedBy string  `json:"updated_by"`
}

// SyncResponse ist eine Seite des Abgleichs.
type SyncResponse struct {
	// HubID ist die Kennung des Hubs.
	HubID string `json:"hub_id"`
	// Version ist die Fassung, in der der Hub antwortet.
	Version int `json:"version"`
	// Collections nennt jede angefragte Collection, in der Reihenfolge der
	// Anfrage, mit erlaubt oder nicht erlaubt.
	Collections []CollectionStatus `json:"collections"`
	// Allowed sind alle Collections, die der Node abgleichen darf, sortiert.
	Allowed []string `json:"allowed"`
	// Rows sind die Zeilen der erlaubten angefragten Collections mit
	// revision > Since der jeweiligen Collection und ≤ HubRevision, sortiert
	// nach Revision, dann id. Die Seite endet an einer Revisionsgrenze.
	Rows []Row `json:"rows"`
	// HubRevision ist die Revision H des Hubs, gelesen vor den Zeilen.
	HubRevision int64 `json:"hub_revision"`
	// Until ist die Revision, bis zu der der Node nach dieser Seite alles
	// hat: die Revision der letzten Zeile, wenn More gilt, sonst HubRevision.
	// Der Node setzt je Collection max(Since, Until), nie zurück.
	Until int64 `json:"until"`
	// More sagt, dass nach Until weitere Zeilen folgen; der Node fragt dann
	// ab Until weiter.
	More bool `json:"more"`
}

// Code ist die Art eines Fehlers des Vertrags; über HTTP steht er im Body.
type Code string

// Fehlercodes der Fassung 1.
const (
	// CodeUnauthenticated: Node unbekannt, Token falsch oder Node gesperrt —
	// dieselbe Antwort für alle drei.
	CodeUnauthenticated Code = "unauthenticated"
	// CodeInvalid: die Anfrage ist ungültig.
	CodeInvalid Code = "invalid"
	// CodeUnsupportedVersion: der Hub kennt oder bedient die Fassung des
	// Nodes nicht.
	CodeUnsupportedVersion Code = "unsupported_version"
	// CodeAccountUnauthenticated: der Node ist angemeldet, der Account nicht
	// — unbekannt, Token falsch oder gesperrt, dieselbe Antwort.
	CodeAccountUnauthenticated Code = "account_unauthenticated"
	// CodeNoSharedCollection: der Account hat keine der Collections, die der
	// Node abgleichen darf (rotate).
	CodeNoSharedCollection Code = "no_shared_collection"
)

// Codes sind alle Fehlercodes des Vertrags.
var Codes = []Code{CodeUnauthenticated, CodeInvalid, CodeUnsupportedVersion, CodeAccountUnauthenticated,
	CodeNoSharedCollection}

// Error ist ein Fehler des Vertrags: ein Code und eine Meldung. errors.Is
// vergleicht nur den Code, so dass jeder *Error mit gleichem Code die
// passende Fehlervariable trifft.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// Is meldet gleichen Code.
func (e *Error) Is(target error) bool {
	var t *Error
	return errors.As(target, &t) && t.Code == e.Code
}

// Fehlervariablen für errors.Is, je Code eine, mit der Standardmeldung.
var (
	ErrUnauthenticated        = &Error{Code: CodeUnauthenticated, Message: "nicht angemeldet"}
	ErrInvalid                = &Error{Code: CodeInvalid, Message: "ungültige Anfrage"}
	ErrUnsupportedVersion     = &Error{Code: CodeUnsupportedVersion, Message: "Fassung nicht unterstützt"}
	ErrAccountUnauthenticated = &Error{Code: CodeAccountUnauthenticated,
		Message: "Account nicht angemeldet"}
	ErrNoSharedCollection = &Error{Code: CodeNoSharedCollection,
		Message: "der Account hat keine der Collections, die dieser Node abgleichen darf"}
)

// ErrOutcomeUnknown meldet einen Transportfehler, nach dem offen ist, ob der
// Hub die Anfrage ausgeführt hat — etwa eine Zeitüberschreitung, nachdem sie
// abgeschickt war. Für rotate heißt das: prüfen (whoami mit Account-Teil),
// nicht wiederholen. Ein Fehler, der vor dem Abschicken entstand
// (Verbindung abgelehnt), ist es nicht.
var ErrOutcomeUnknown = errors.New("Ausgang unklar")

// Invalid liefert einen Fehler mit Code CodeInvalid und einem Grund.
func Invalid(reason string) *Error {
	return &Error{Code: CodeInvalid, Message: "ungültige Anfrage: " + reason}
}
