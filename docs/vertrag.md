# Vertrag zwischen Node und Hub

Fassung 1. Dieser Text ist verbindlich; der Code folgt ihm. Im Code steht der Vertrag im
neutralen Paket `internal/contract` (Typen, Fehler, Schnittstelle `Hub`). Der Hub setzt die
Schnittstelle um (`internal/hub/replication`), der Node benutzt sie und kennt nur sie. Welche
Umsetzung er bekommt, entscheidet `cmd/kephalaion`: `local` ist ein Funktionsaufruf im selben
Prozess, `http` ist HTTP mit JSON (`internal/contract/httpapi`, siehe „HTTP“). Beide prüfen
dasselbe; die Tests des Vertrags laufen gegen beide.

Drei Vorgänge: `whoami` (wer bin ich, gilt dieser Account), `rotate` (Token eines Accounts
ersetzen) und `sync` (Abgleich). Begriffe: [`begriffe.md`](begriffe.md); Hintergrund:
[`konzept.md`](konzept.md), „Abgleich“ und „Authentifizierung“.

## Fassung

Jede Anfrage nennt die Fassung des Nodes, jede Antwort die des Hubs. Fassung 1 ist die
einzige. In Go ist sie ein Feld jeder Anfrage (`Version`), über HTTP steht sie im
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

## Anmeldung eines Accounts

`whoami` und `rotate` tragen zusätzlich einen Account: seinen Namen und sein Token, im Body
(der Node ist der Träger, der Account sagt, in wessen Namen). Der Hub prüft gegen seine Tabelle
`accounts` — dort steht der maßgebliche Hash, auch für einen gesperrten Account —, hasht das
Token und vergleicht in konstanter Zeit; einen unbekannten Account vergleicht er gegen einen
festen Ersatz-Hash. Unbekannt, falsches Token und gesperrt sind dieselbe Antwort.

## whoami

Bestätigt den Node und nennt, was er abgleichen darf. Mit Account-Teil prüft es zusätzlich den
Account — so prüft ein Node nach einem unklaren `rotate`, welches Token gilt.

| Feld | JSON | Bedeutung |
|---|---|---|
| Fassung, Node, Token | — (Pfad, Header) | wie bei `sync` |
| Account (wahlweise) | `account` | `{"account": <name>, "token": <token>}` |

Antwort:

| Feld | JSON | Bedeutung |
|---|---|---|
| Hub-Kennung | `hub_id` | die `hub_id` des Hubs |
| Fassung | `version` | 1 |
| Node | `node` | der Name des Nodes am Hub |
| Erlaubte Collections | `allowed` | alle Collections, die der Node abgleichen darf, sortiert |
| Account | `account` | nur mit Account-Teil: `{"account", "valid", "collections"}`. `valid` falsch heißt unbekannt, falsches Token oder gesperrt — dieselbe Antwort, kein Fehler. `collections` sind die Collections des Accounts, die dieser Node abgleichen darf, sortiert; leer, wenn `valid` falsch ist. |

## rotate

Ersetzt das Token eines Accounts. Der Node erzeugt das neue Token, schickt nur seinen Hash und
behält das Token selbst; zur Anmeldung dient das alte.

| Feld | JSON | Bedeutung |
|---|---|---|
| Fassung, Node, Token | — (Pfad, Header) | wie bei `sync`; der Node ist der Träger |
| Account | `account` | Name des Accounts |
| Altes Token | `token` | das bisherige Token des Accounts |
| Neuer Hash | `new_hash` | sha256 des neuen Tokens, 64 Zeichen hex |

Der Hub prüft in dieser Reihenfolge: Fassung, Form von `new_hash` (sonst `invalid`),
Anmeldung des Nodes, altes Token gegen `accounts` (gesperrt gilt nicht:
`account_unauthenticated`). Hat der Account keine der Collections, die der Node abgleichen
darf, antwortet er `no_shared_collection` — **vor** jeder Änderung. Sonst ersetzt er den Hash in
`accounts` und in allen Zeilen des Accounts in **einer** Transaktion. Sie beginnt mit dem
bedingten Schreiben in `accounts` (nur wenn das alte Token noch gilt und der Account nicht
gesperrt ist) — so gelingt von zwei gleichzeitigen `rotate` mit demselben alten Token nur einer.
Es ist ein Schreibvorgang mit
einer Revision und genau einer Zeile in `actions` (`account` = der Account, `carrier` = der
Node, `action` = `rotate`, `subject` = der Account). Fehlversuche stehen nicht in `actions`,
nur im Log des Hubs (ohne Token).

Antwort: `hub_id`, `version` und `rows` — die Account-Zeilen mit dem neuen Hash, beschränkt auf
die Collections, die der Node abgleichen darf, nach Collection; Form wie bei `sync`. Alles, was
die Antwort braucht, liest der Hub **vor** dem Commit; danach stellt er sie nur noch zusammen.

**Nicht wiederholbar.** Nach einem erfolgreichen `rotate` gilt das alte Token nicht mehr; ein
zweiter Versuch mit ihm scheitert. Ein Transport wiederholt `rotate` deshalb nie. Ist nach dem
Abschicken offen, ob der Hub es ausgeführt hat, meldet der Transport das als eigenen Fall
(`contract.ErrOutcomeUnknown`), und der Node prüft mit `whoami`. Eindeutig gescheitert ist
`rotate` nur mit einem Fehler des Vertrags (einem der Codes unten) oder wenn die Anfrage den Hub
nachweislich nicht erreicht hat. Jeder andere Fehler — auch einer der Datenbank nach dem
Commit — ist unklar: über HTTP als 500, über `local` meldet der Transport ihn ebenso als
`contract.ErrOutcomeUnknown`.

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

| Code | HTTP | Bedeutung |
|---|---|---|
| `unauthenticated` | 401 | nicht angemeldet: Node unbekannt, Token falsch oder Node gesperrt — dieselbe Meldung für alle drei |
| `account_unauthenticated` | 403 | der Node ist angemeldet, der Account nicht: unbekannt, Token falsch oder gesperrt (`rotate`) |
| `no_shared_collection` | 409 | der Account hat keine der Collections, die der Node abgleichen darf (`rotate`) |
| `invalid` | 400 | ungültige Anfrage: Seitengröße ≤ 0, Collection doppelt, `since` negativ, `new_hash` kein sha256, kein gültiges JSON; über HTTP auch 404 (unbekannter Vorgang), 405 (nicht POST), 413 (Body zu groß) |
| `unsupported_version` | 404 | Fassung nicht unterstützt |

Fehler des Transports oder der Datenbank sind keine Fehler des Vertrags; der Node versucht es
später wieder. Über HTTP antwortet der Hub dann mit 500 und dem Code `internal`, der kein Code
des Vertrags ist.

## HTTP

- **Pfad:** `POST /v<Fassung>/<Vorgang>`, also `/v1/whoami`, `/v1/rotate`, `/v1/sync`. Die
  Fassung im Pfad ist die Fassung des Vertrags. Eine fremde Fassung (`/v2/…`, auch `/v0/…`)
  beantwortet der Hub mit 404 und `unsupported_version`, vor der Anmeldung; ein unbekannter
  Vorgang ist 404 mit `invalid`, eine andere Methode als POST 405.
- **Anmeldung des Nodes** in Headern: `X-Keph-Node: <name>` und `Authorization: Bearer
  <token>`. Der Body ist JSON (die Felder oben); ein leerer Body gilt als `{}`.
- **Antwort:** 200 mit JSON, gzip-komprimiert, wenn die Anfrage `Accept-Encoding: gzip` trägt
  (der Client des Nodes bittet immer darum). Fehler: Status nach der Tabelle oben, Body
  `{"code": …, "message": …}`.
- **Grenzen:** Der Body einer Anfrage darf höchstens 1 MiB groß sein (sonst 413). Antworten
  sind nicht begrenzt — eine Seite des Abgleichs kann eine große Revision ganz tragen. Der
  Server liest Kopf und Body innerhalb von 10 s bzw. 60 s und darf eine Antwort bis zu 10
  Minuten lang schreiben. Der Client wartet auf `whoami` und `rotate` 30 s, auf eine Seite
  von `sync` 10 Minuten.
- **Wiederholung:** `whoami` und `sync` wiederholt der Client bei Fehlern des Transports und
  bei 5xx bis zu dreimal, mit wachsendem Abstand (0,5 s, 1 s, 2 s). `rotate` nie.
- **Weiterleitungen:** Der Client folgt keiner Weiterleitung. Eine 3xx-Antwort ist ein Fehler,
  ohne Wiederholung; bei `rotate` ein eindeutiger — der Hub hat nicht ausgeführt, und Body und
  Token gehen an kein anderes Ziel.
- **Unklarer Ausgang bei `rotate`:** Kam die Verbindung nicht zustande oder antwortet der Hub
  mit einer Weiterleitung, ist nichts geschehen. Jeder andere Fehler nach dem Abschicken —
  Zeitüberschreitung, abgebrochene Verbindung, unlesbare Antwort, 5xx — ist unklar
  (`contract.ErrOutcomeUnknown`).
- **Host:** Der Hub beantwortet nur Anfragen, deren `Host` dieser Rechner (`localhost`,
  `127.0.0.1`, `[::1]`) mit dem Port ist, auf dem die Anfrage ankam — dieselbe Prüfung wie am
  Node vor `/mcp`. Sonst antwortet er 403 ohne Vertragsform, noch vor Pfad und Anmeldung. Ein
  Tunnel geht damit nur mit gleichem Port (`ssh -L 7434:localhost:7434`), bis `ssh` und `https`
  als Transport kommen.
- **Log:** eine Zeile je Anfrage mit Methode, Pfad, Status, Dauer, Node- und Account-Namen;
  Namen, die der Namensregel nicht folgen, erscheinen maskiert. Nie ein Token, nie ein Body.

## Bekannte Grenzen

- **Ein großer Import ist eine unbegrenzte Seite.** `hub import` schreibt alle Dokumente unter
  einer Revision, und eine Revision kommt immer ganz. Über HTTP wird das später ein Datenstrom
  oder eine Obergrenze je Schreibvorgang; in Fassung 1 gibt es kein Limit.
- **Wiederherstellung aus einer Sicherung** braucht eine neue `hub_id` (siehe oben).
- **Fehlversuche werden nicht begrenzt.** Ein Node kann beliebig viele Account-Tokens
  probieren; er muss dafür aber selbst angemeldet sein. Eine Begrenzung kommt später.
