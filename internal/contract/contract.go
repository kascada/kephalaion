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
// Nodes und wird am Hub geprüft, auch auf dem lokalen Weg.
type Hub interface {
	// Sync liefert eine Seite des Abgleichs. Fehler sind *Error mit einem
	// der Codes dieses Pakets oder Fehler des Transports bzw. der Datenbank.
	Sync(ctx context.Context, req SyncRequest) (SyncResponse, error)
}

// NodeAuth ist die Anmeldung des Nodes am Hub: sein Name dort und sein Token.
// Über HTTP steht sie in Headern, nicht im Body.
type NodeAuth struct {
	Node  string `json:"-"`
	Token string `json:"-"`
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
)

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
	ErrUnauthenticated    = &Error{Code: CodeUnauthenticated, Message: "nicht angemeldet"}
	ErrInvalid            = &Error{Code: CodeInvalid, Message: "ungültige Anfrage"}
	ErrUnsupportedVersion = &Error{Code: CodeUnsupportedVersion, Message: "Fassung nicht unterstützt"}
)

// Invalid liefert einen Fehler mit Code CodeInvalid und einem Grund.
func Invalid(reason string) *Error {
	return &Error{Code: CodeInvalid, Message: "ungültige Anfrage: " + reason}
}
