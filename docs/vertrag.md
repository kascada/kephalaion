# Vertrag zwischen Node und Hub

Fassung 1. Dieser Text ist verbindlich; der Code folgt ihm. Im Code steht der Vertrag im
neutralen Paket `internal/contract` (Typen, Fehler, Schnittstelle `Hub`). Der Hub setzt die
Schnittstelle um (`internal/hub/replication`), der Node benutzt sie und kennt nur sie. Welche
Umsetzung er bekommt, entscheidet `cmd/kephalaion`: `local` ist ein Funktionsaufruf im selben
Prozess, später kommt HTTP mit JSON dazu. Beide prüfen dasselbe.

Diese Fassung enthält nur den Abgleich (`sync`). Begriffe: [`begriffe.md`](begriffe.md);
Hintergrund: [`konzept.md`](konzept.md), „Abgleich“ und „Authentifizierung“.

## Fassung

Jede Anfrage nennt die Fassung des Nodes, jede Antwort die des Hubs. Fassung 1 ist die
einzige. In Go ist sie ein Feld der Anfrage (`SyncRequest.Version`), über HTTP steht sie im
Pfad (`/v1/…`) und nicht im Body. Eine Fassung, die der Hub nicht kennt oder nicht bedient,
beantwortet er mit `unsupported_version`, noch vor der Anmeldung.

## Anmeldung

Jede Anfrage trägt den Namen des Nodes am Hub und sein Token (`NodeAuth`). Über HTTP stehen
beide in Headern, nicht im Body; das Token steht nie in einem Log.

Der Hub schlägt den Node über seinen Namen nach, hasht das vorgelegte Token (sha256) und
vergleicht es in konstanter Zeit mit dem gespeicherten Hash. Gibt es den Node nicht, vergleicht
er gegen einen festen Ersatz-Hash, damit die Arbeit dieselbe ist. Unbekannter Node, falsches
Token und gesperrter Node ergeben dieselbe Antwort: `unauthenticated`. Der Hub prüft bei jedem
Aufruf, auch auf dem lokalen Weg.

## sync

### Anfrage

| Feld | JSON | Bedeutung |
|---|---|---|
| Fassung | — (Pfad) | Fassung des Nodes, 1 |
| Node, Token | — (Header) | Anmeldung, siehe oben |
| Collections | `collections` | Liste von Paaren (`collection`, `since`): alles aus dieser Collection mit `revision > since`. Jede Collection höchstens einmal, `since ≥ 0`; eine neue Collection fragt ab 0. Die Liste darf leer sein. |
| Seitengröße | `page_size` | gewünschte Zeilen je Seite, > 0. Standard des Nodes: 500 (`contract.DefaultPageSize`). |

### Antwort

| Feld | JSON | Bedeutung |
|---|---|---|
| Hub-Kennung | `hub_id` | die `hub_id` aus `db_info` des Hubs |
| Fassung | `version` | Fassung der Antwort, 1 |
| Status je Collection | `collections` | jede angefragte Collection in der Reihenfolge der Anfrage, mit `allowed` wahr oder falsch. Eine unbekannte Collection und eine nicht erlaubte sind dieselbe Antwort (falsch). |
| Erlaubte Collections | `allowed` | alle Collections, die der Node abgleichen darf (`node_collections`), sortiert — auch nicht angefragte |
| Zeilen | `rows` | siehe unten |
| Hub-Revision | `hub_revision` | die Revision H des Hubs |
| bis | `until` | die Revision, bis zu der der Node nach dieser Seite alles hat |
| mehr | `more` | es folgen weitere Zeilen; der Node fragt ab `until` weiter |

**Zeilen.** Aus jeder angefragten und erlaubten Collection alle Zeilen von `documents` mit
`revision > since` dieser Collection und `revision ≤ H`, sortiert nach Revision, dann nach
`id`. Jede Zeile trägt alle Spalten, so wie sie am Hub steht:

| Spalte | JSON | Typ |
|---|---|---|
| `id` | `id` | Text |
| `collection` | `collection` | Text |
| `name` | `name` | Text |
| `content` | `content` | Text oder `null` (Löschmarke) |
| `meta` | `meta` | Text (JSON als Text, der Hub deutet es nicht) oder `null` |
| `deleted` | `deleted` | wahr oder falsch |
| `revision` | `revision` | Zahl |
| `created_at`, `updated_at` | `created_at`, `updated_at` | Zahl, ms seit Epoche |
| `created_by`, `updated_by` | `created_by`, `updated_by` | Text |

Auch Löschmarken und `SYSTEM:`-Zeilen kommen mit (Account-Zeilen siehe unten). `null` ist ausdrücklich: Der Node speichert
genau die Zeile des Hubs und leitet `content` nicht aus `deleted` ab. In Go sind die nullbaren
Spalten Zeiger (`*string`, `nil` = NULL).

**Lesestand ohne Transaktion.** Der Hub liest H zuerst und liefert danach nur Zeilen mit
`revision ≤ H`. Was während der Anfrage geschrieben wird, trägt eine Revision über H und kommt
mit der nächsten Seite. So passen Seite und H zusammen, ohne Lese-Transaktion.

### Seiten

- **Eine Seite endet an einer Revisionsgrenze.** Sie nimmt ganze Revisionen, bis die
  Seitengröße erreicht ist; die Revision, die über die Seitengröße hinausreichte, bleibt für
  die nächste Seite. Ein Schreibvorgang mit vielen Zeilen kommt deshalb ganz oder gar nicht an.
- **Eine einzelne Revision, die größer ist als die Seitengröße, kommt ganz**, als eigene
  Seite.
- **`until`:** Gibt es mehr (`more` wahr), ist `until` die Revision der letzten Zeile der
  Seite. Ist es die letzte Seite (`more` falsch), ist `until` = H — auch bei einer leeren
  Seite: Der Node hat dann alles bis H.
- **Seitengröße:** ≤ 0 ist `invalid`. Über der Obergrenze des Hubs (5000 Zeilen) begrenzt er
  sie still darauf.

### Regeln für den Node

- **Die Revision ist global**, über alle Collections des Hubs. Nach jeder Seite setzt der Node
  je angefragter Collection ihren Stand auf max(`since`, `until`) — nie zurück. Eine
  Collection, die schon weiter war als `until`, bleibt, wo sie war.
- Jede Seite wird in einer Transaktion angewendet (Zeilen per `id` einfügen oder ersetzen,
  Stände fortschreiben). Ein abgebrochener Abgleich setzt beim letzten Stand fort.
- **`hub_id`:** Beim ersten Kontakt merkt der Node sie sich. Weicht sie später ab, verwirft er
  die Replica und gleicht von vorn ab.
- **`since` über H:** Liegt ein `since` über `hub_revision`, wurde der Hub aus einer Sicherung
  mit gleicher `hub_id` zurückgespielt. Der Node behandelt das wie einen Wechsel der `hub_id`.
  Das greift nur, bis der Hub wieder über dieses `since` hinaus geschrieben hat. **Grenze:** Ein
  aus einer Sicherung zurückgespielter Hub braucht deshalb eine neue `hub_id`; bis es dafür ein
  Kommando gibt, verwirft der Node die Replica selbst (`node hub rm` und `node hub add`).
- Collections mit `allowed` falsch — nicht erlaubt, nicht mehr erlaubt oder unbekannt — entfernt
  der Node aus seiner Replica.

## Account-Zeilen

Je Account und Collection steht in `documents` eine Zeile mit dem Namen `SYSTEM:A:<account>`
(`contract.AccountRowPrefix`). Sie gleicht sich ab wie jede andere Zeile; der Node prüft seine
Clients gegen sie. `content` ist JSON in genau dieser Form:

```json
{"hash":"<sha256 des Tokens, 64 Zeichen hex>","rights":{"write":false,"supersede":false}}
```

- `hash` ist eine Kopie; maßgeblich führt der Hub den Hash in seiner Tabelle `accounts`.
- `rights` sind die Rechte in dieser Collection über `read` hinaus; `read` ergibt sich aus der
  Zeile selbst. `write` und `supersede` sind unabhängig.
- Sperren, Entziehen und Entfernen machen die Zeile zur Löschmarke (`content` NULL). Bekommt
  der Account die Collection wieder, wird die Löschmarke mit neuer Revision wiederbelebt; ihre
  `id` bleibt.
- `meta` ist immer NULL.

## Fehler

Ein Fehler hat einen Code und eine Meldung (`contract.Error`, über HTTP als JSON
`{"code": …, "message": …}`). Nach einem Fehler gibt es keine Antwortdaten.

| Code | Bedeutung |
|---|---|
| `unauthenticated` | nicht angemeldet: Node unbekannt, Token falsch oder Node gesperrt — dieselbe Meldung für alle drei |
| `invalid` | ungültige Anfrage: Seitengröße ≤ 0, Collection doppelt, `since` negativ |
| `unsupported_version` | Fassung nicht unterstützt |

Fehler des Transports oder der Datenbank sind keine Fehler des Vertrags; der Node versucht es
später wieder.

## Bekannte Grenzen

- **Ein großer Import ist eine unbegrenzte Seite.** `hub import` schreibt alle Dokumente unter
  einer Revision, und eine Revision kommt immer ganz. Über HTTP wird das später ein Datenstrom
  oder eine Obergrenze je Schreibvorgang; in Fassung 1 gibt es kein Limit.
- **Wiederherstellung aus einer Sicherung** braucht eine neue `hub_id` (siehe oben).
