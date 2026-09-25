# Task 003 — Collections, Nodes und Hubs einrichten und auslesen

Am Hub lassen sich Collections und Nodes anlegen und Nodes Collections erlauben; am Node lassen
sich Hubs und die gewünschten Collections eintragen. Alles über die CLI, noch ohne Verbindung.

## Intent

Hub und Node wissen voneinander, bevor sie miteinander reden: Der Hub kennt seine Nodes und
was sie abgleichen dürfen, der Node kennt seine Hubs, sein Token dort und was er haben will.
- Jede lokale Einstellung ist über die CLI anlegbar, lesbar, änderbar und entfernbar.
- Ein Node-Token wird vom Hub erzeugt, genau einmal angezeigt und nur als Hash gespeichert;
  am Node wird es nie als Argument übergeben, sondern über stdin.
- `status` zeigt die Verbindungen: am Hub Collections und Nodes, am Node Hubs und gewünschte
  Collections.
- `config export`/`import` sichern auch die neuen Tabellen.
- Hub und Node bleiben im Code getrennt; `transport local` prüft nur die config, nicht den Hub.

## Referenzen

- `docs/konzept.md` — „Datenmodell“ (Tabellen `collections`, `nodes`, `node_collections`,
  `hubs`, `hub_collections`, Regeln darunter), „Abgleich“ (`hub_id`), „Authentifizierung“
  (Einrichtung, Token-Format `keph_`, sha256), „Mehrere Hubs“, „Ein Programm, zwei Rollen“
  (Testaufbau mit `http` auf `localhost`).
- `docs/begriffe.md` — node entry, hub entry, hub_id, transport, replicate.
- `k-playbook-local/tasks/done/002-datenbank-config-status.md` — was es schon gibt.
- `cmd/kephalaion/roles.go`, `configcmd.go`, `internal/hub/store`, `internal/node/store`,
  `internal/sqlitedb`, `internal/sqlq`.

## Ziel

```text
Hub:
  kephalaion hub collection add <name> [--description …] | list | rm <name>
  kephalaion hub collection set <name> --description …
  kephalaion hub node add <name> [--description …]      → zeigt das Token einmal
  kephalaion hub node list | show <name> | rm <name>
  kephalaion hub node set <name> --description …
  kephalaion hub node lock <name> | unlock <name>
  kephalaion hub node grant <node> <collection> | revoke <node> <collection>
  kephalaion hub node token <name>                       → neues Token, altes ungültig
Node:
  kephalaion node hub add <alias> --transport local|http|https|ssh
                          [--address …] [--ssh-key …] --token-stdin
  kephalaion node hub list | show <alias> | rm <alias>
  kephalaion node hub set <alias> [--transport …] [--address …] [--ssh-key …]
  kephalaion node hub token <alias> --token-stdin
  kephalaion node collection add <hub>:<collection> | list | rm <hub>:<collection>
```

Dazu `hub_id` bei `hub init`, erweitertes `status`, erweitertes `config export|import`.

## Kontext

- **Schemafassung 2** für Hub und Node, ohne Migration: Die Meldung aus Task 002 („neu
  anlegen, Einstellungen per export/import retten“) greift. Der Export aus Fassung 1 muss
  sich in Fassung 2 importieren lassen, soweit er `settings` enthält.
- **Tabellen genau nach „Datenmodell“** im Konzept, mit einer Ausnahme: `actions` bekommt
  die Spalte `subject TEXT` — das Ziel einer Handlung ohne Dokument, also Collection- oder
  Node-Name, bei `grant`/`revoke` `<node>:<collection>`. Das Konzept wird in Etappe 6
  nachgezogen. Die Tabellen gleichen sich nicht ab.
  `documents.collection` bekommt keinen Fremdschlüssel — Löschmarken und spätere Abläufe
  sollen nicht daran hängen.
- **`hub_id`:** ULID, von `hub init` in `db_info` geschrieben, in `hub status` gezeigt. ULID
  per gepflegter Bibliothek (etwa `github.com/oklog/ulid/v2`); sie wird später auch für
  `documents.id` gebraucht. `hubs.hub_id` am Node bleibt leer bis zum ersten Kontakt.
  `db_info` und damit `hub_id` wird nicht exportiert: Nach Neuanlage und Import hat der Hub
  eine neue `hub_id`, und Nodes gleichen von vorn ab — so gewollt.
- **Namen** von Collections, Nodes und Hub-Aliasen: `[a-z0-9][a-z0-9._-]{0,62}`. Kein `:`
  (Adressen sind `<hub>:<collection>`), kein Präfix `system` in beliebiger Schreibweise.
  Bei `<hub>:<collection>` am Node werden beide Teile so geprüft.
- **Gemeinsames Paket:** Namensregel, Token-Erzeugung, Hash, `keph_`-Formatprüfung und
  gekürzte Anzeige liegen in einem neutralen Paket unter `internal/` (etwa `internal/ident`),
  nicht unter `internal/hub` — Hub und Node nutzen es beide, später auch Accounts.
- **Node-Namen sind gemeinsam mit Account-Namen eindeutig.** Accounts gibt es noch nicht;
  die Prüfung gegen `SYSTEM:A:<name>` in `documents` wird trotzdem jetzt gebaut. Treffer ist
  jede Zeile mit `name = 'SYSTEM:A:<name>'` in beliebiger Collection, auch eine Löschmarke.
  Diese Abfrage bleibt im Hub-Store.
- **Token:** `keph_` + 32 Zufallsbytes (`crypto/rand`), base64url ohne Padding. Gespeichert
  am Hub `sha256` hex. `hub node add` und `hub node token` geben es genau einmal aus, mit
  dem Hinweis, dass es nicht wieder angezeigt wird.
- **Token am Node über stdin:** `--token-stdin` liest eine Zeile; nie als Argument (Shell-
  Verlauf, Prozessliste). Ohne gültiges `keph_`-Format abbrechen. Anzeigen nur gekürzt
  (`keph_…` plus die letzten 4 Zeichen).
- **Transporte am Node:** `local` verlangt, dass in derselben config ein Hub eingerichtet
  ist, und keine `--address`; höchstens ein `local`-Eintrag je Node. `http` verlangt eine
  URL `http://` mit Host `localhost`, `127.0.0.1` oder `::1` (etwa `http://localhost:8080`).
  `https` verlangt eine URL `https://`. `ssh` verlangt eine nicht leere Adresse
  (`[user@]host[:port]`), die nicht weiter geprüft wird, und speichert sie mit `--ssh-key`.
  Die Prüfung von `local` liest nur die config und importiert nichts aus `internal/hub`.
  `--ssh-key` nur bei `ssh`. `node hub set` ändert nur die angegebenen Felder und prüft den
  resultierenden Eintrag mit denselben Regeln; ein Wechsel des Transports verwirft Felder, die
  zum neuen nicht passen (Adresse bei `local`, Schlüssel außer bei `ssh`). `hub_collections`
  und `hub_id` bleiben — eine abweichende `hub_id` erkennt der Node beim Kontakt.
- **Flags vor und nach Positionsargumenten:** `hub node add laptop --description …` muss
  gehen. `parseFlags` in `roles.go` hört bisher beim ersten Positionsargument auf; es wird
  ohne CLI-Bibliothek so erweitert, dass es danach weiterparst. Bestehende Kommandos laufen
  unverändert weiter.
- **Entfernen:** `hub collection rm` nur, wenn kein Node sie erlaubt hat und keine Dokumente
  in ihr stehen — als Dokument zählt jede Zeile in `documents` mit dieser Collection, auch
  Löschmarken und `SYSTEM:`-Zeilen; sonst abbrechen mit dem Grund. `hub node rm` entfernt
  auch seine `node_collections`. `node hub rm` entfernt auch seine `hub_collections`.
- **`grant` verlangt, dass es Node und Collection gibt.** `node collection add` prüft nur,
  dass es den Hub-Eintrag gibt — ob der Hub die Collection erlaubt, weiß der Node erst beim
  Abgleich.
- **Protokoll:** Jede Änderung am Hub schreibt eine Zeile in `actions`, `account` = `admin`,
  `action` etwa `collection.add`, `collection.set`, `node.add`, `node.set`, `node.grant`,
  `node.lock`, `node.token`, mit dem Ziel in `subject`. `config import` schreibt eine Zeile
  `config.import` in die Hub-Datenbank, nur wenn der Export einen Hub-Teil hat; am Node wird
  nichts protokolliert (dort gibt es kein `actions`).
  `created_by` = `admin`.
- **Export:** Format 2 mit `collections`, `nodes` (samt `token_hash`), `node_collections`,
  `hubs` (samt Token im Klartext), `hub_collections`. Datei bleibt `0600`. Import in Format 2
  ersetzt je Rolle alles in einer Transaktion; Import in Format 1 ersetzt nur `settings` und
  lässt die neuen Tabellen unberührt. `actions` und `db_info` werden weder exportiert noch
  ersetzt.
- **Import prüft wie die CLI**, mit denselben Funktionen und vollständig vor dem Schreiben
  (wie in Task 002 für `settings`): Namensregel, `keph_`-Format, Transportregeln,
  Eindeutigkeit samt `SYSTEM:A:`. Er bricht auch ab, wenn er eine Collection entfernen
  würde, in der Dokumente stehen — dieselbe Regel wie bei `rm`.
- **Nachbesserungen aus der Code-Review von Task 002** (Befunde im Ausführungsbericht von
  `tasks/done/002-datenbank-config-status.md`):
  1. **Revision sicher hochzählen.** `nextRevision` im Hub-Store liest erst und schreibt dann;
     unter PostgreSQL vergeben zwei Schreiber so dieselbe Revision, unter SQLite scheitert der
     Wechsel vom Lesen zum Schreiben mit `SQLITE_BUSY` — das trifft jede Transaktion, die erst
     liest und dann schreibt, auch die aus Etappe 2 und 3. Unter SQLite öffnet der Unterbau
     Transaktionen daher als IMMEDIATE, über den DSN-Parameter `_txlock=immediate` von modernc
     (kein PRAGMA; gilt für Hub und Node). Für PostgreSQL sperrt `nextRevision` die Zeile
     zuerst schreibend (`UPDATE db_info SET value = value WHERE key = $1`), liest dann und
     schreibt den neuen Wert, ohne `FOR UPDATE`.
  2. **WAL nur beim Anlegen.** `journal_mode(WAL)` bleibt in der Datei stehen; heute setzt
     jedes Öffnen es, sodass `status` eine fremde SQLite-Datei umstellt, bevor `CheckInfo` sie
     ablehnt. WAL steht nur in der DSN von `Create` (heute im gemeinsamen `dsn()` in
     `internal/sqlitedb/sqlitedb.go`, das auch `Open` nutzt), kein eigenes PRAGMA danach.
     `Open` setzt nur die verbindungsbezogenen `foreign_keys` und `busy_timeout`, die die Datei
     nicht ändern. Der bestehende Test, der nach `Open` `wal` erwartet, gilt künftig für mit
     `Create` angelegte Dateien.
  3. **Import sagt, was geschrieben ist.** Import schreibt erst den Hub, dann den Node; die
     Zeile `config.import` steht in derselben Hub-Transaktion wie das Ersetzen. Scheitert das
     Schreiben des Node, nachdem der Hub schon committet ist, nennt die Meldung je Rolle
     „ersetzt“ bzw. „nicht geschrieben“.
  4. **Kein stilles Leeren beim Import.** Steht eine Rolle in der config des Exports, fehlt
     aber ihr Teil (`settings`, in Format 2 auch die Tabellen) oder ist er `null`, bricht der
     Import vor dem Schreiben ab. Nur ein ausdrücklich leerer Teil (`{}` bzw. `[]`) leert.
     Dafür schreibt der Export jeden Teil jeder Rolle immer, auch leer (kein `omitempty`,
     kein fehlender Schlüssel), und das Einlesen unterscheidet „fehlt“, `null` und leer
     (heute werden alle drei zu `nil`).
- **Nicht in diesem Task:** Accounts, `serve`, jede Verbindung, `rotate`, Abgleich; `status`
  auf eine schreibgeschützte Datenbank (scheitert heute an `mode=rw`, Befund 3 aus Task 002).

## Zu bauen

### Etappe 1 — Schema und hub_id

- Hub: `collections`, `nodes`, `node_collections`; `actions.subject`; `hub_id` in `db_info`
  bei `Create`.
- Node: `hubs`, `hub_collections`.
- Schemafassung beider auf 2. Tests: anlegen, Fassung 1 wird mit der bekannten Meldung
  abgewiesen.
- Nachbesserungen 1 und 2 aus dem Kontext. Tests: zwei gleichzeitige Transaktionen mit
  `nextRevision` erhalten verschiedene Revisionen; zwei gleichzeitige schreibende
  Transaktionen, die vorher lesen (am Hub Lesen + `nextRevision`, am Node Lesen + Schreiben),
  scheitern nicht an `SQLITE_BUSY`; `Open` auf eine fremde SQLite-Datei ohne WAL lässt deren
  `journal_mode` unverändert; `Create` legt die Datei mit `wal` an.

### Etappe 2 — Hub: Collections und Nodes im Store

- Methoden der Hub-Schnittstelle für alles aus „Ziel“, je eine Transaktion samt `actions`.
  SQL bleibt PostgreSQL-tauglich.
- Das gemeinsame Paket (Namensregel, Token, Hash, Formatprüfung, gekürzte Anzeige) wie im
  Kontext; die Eindeutigkeitsprüfung gegen `SYSTEM:A:` im Hub-Store.
- Tests: anlegen, doppelt, ungültige Namen, grant ohne Collection, rm mit Abhängigkeiten
  (auch nur Löschmarke), Name belegt durch `SYSTEM:A:`-Löschmarke, Token-Format, Hash
  gespeichert statt Token, `actions`-Zeilen samt `subject`; `collection set`/`node set`
  ändern die Beschreibung, Grants und Token-Hash bleiben.

### Etappe 3 — Node: Hubs und Collections im Store

- Methoden der Node-Schnittstelle für alles aus „Ziel“.
- Transportregeln; Namensregel, Token-Format und gekürzte Anzeige aus dem gemeinsamen Paket.
- Tests: jede Transportregel, `local` ohne Hub in der config, zweites `local`, ungültiger
  Teil in `<hub>:<collection>`, rm entfernt Abhängiges; `hub set` mit ungültigem Ergebnis
  (etwa zweites `local`) wird abgewiesen, Wechsel auf `local` verwirft die Adresse,
  `hub_collections` bleiben.

### Etappe 4 — CLI

- Kommandos aus „Ziel“ in `cmd/kephalaion`, Hilfetexte im bestehenden Stil, `usage` ergänzt.
- `parseFlags` parst nach Positionsargumenten weiter. Test: Flag vor und nach dem Namen,
  bestehende Kommandos unverändert.
- `--token-stdin` über einen hereingereichten `io.Reader` (testbar).
- Tests über `run()`: Ablauf Hub (collection add → collection set → node add → node set →
  grant → show → lock → token → rm) und Node (hub add local/http → hub set → collection add →
  list → rm).

### Etappe 5 — status und config

- `status`: Hub mit `hub_id`, Collections mit Namen (aus `collections`), Nodes mit
  gesperrt/erlaubte Collections; Node mit jedem Hub (Alias, Transport, Adresse, `hub_id` oder
  „noch kein Kontakt“) und seinen gewünschten Collections.
- `config export|import` Format 2 wie im Kontext; Format 1 importierbar (ersetzt nur
  `settings`).
- Nachbesserungen 3 und 4 aus dem Kontext, für Format 1 und 2.
- Tests: Export → neue Datenbanken → Import ergibt dieselben Tabellen (neue `hub_id`);
  ungültiger Import schreibt nichts; Import, der eine Collection mit Dokumenten entfernen
  würde, bricht ab; Import in Format 1 lässt die neuen Tabellen unberührt; `config.import` in
  `actions` am Hub, nur bei Export mit Hub-Teil; Rolle ohne `settings`-Teil oder mit `null`
  bricht ab und lässt alles unverändert, `settings: {}` leert; Rundlauf mit leeren Tabellen
  und leeren `settings`; scheitert das Schreiben des Node (über einen ersetzbaren
  Eingriffspunkt, Wahl dem Ausführenden), nennt die Meldung den Hub als ersetzt, samt
  `config.import`, und den Node als nicht geschrieben.

### Etappe 6 — Doku

- `README.md`: Beispiel Einrichtung Hub + Node auf einem Rechner (`local` und `http`).
- `docs/begriffe.md`: nur, was neu ist.
- `k-playbook-local/k-playbook.md`: Token nie als Argument; Namensregeln; Eintrag `sqlitedb`
  nachziehen (WAL nur bei `Create`, Transaktionen IMMEDIATE über `_txlock`).
- `docs/konzept.md`: `actions.subject` ins „Datenmodell“; sonst nur nachziehen, wo die
  Umsetzung abweicht.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — Schema und hub_id | erledigt | 2026-09-25 | Fassung 2 (Hub: collections, nodes, node_collections, actions.subject; Node: hubs, hub_collections), hub_id als ULID; `_txlock=immediate`, WAL nur bei Create, Sperrzeile in nextRevision |
| 2 — Hub: Collections und Nodes im Store | offen | | |
| 3 — Node: Hubs und Collections im Store | offen | | |
| 4 — CLI | offen | | |
| 5 — status und config | offen | | |
| 6 — Doku | offen | | |

---
## Review-Log (2026-09-25)

**Pfad:** k-playbook-local/tasks/003-collections-nodes-hubs.md
**Intent:** inline (`## Intent`)
**Runden:** 3

### Diskussion
- **Protokollziel (FEHLER-01):** `actions` hat im Konzept keine Spalte für das Ziel von
  Handlungen ohne Dokument; der Task verbot zugleich Abweichungen vom Datenmodell. Moderator
  entschied eine Spalte `subject` als benannte Ausnahme, Konzept wird in Etappe 6 nachgezogen.
- **Export mit Klartext-Token auf stdout (WARNUNG-06):** Critic wollte `--output` erzwingen;
  Moderator übersprang, weil `export` ausdrücklich aufgerufen wird und stdout das Verhalten aus
  Task 002 ist. Critic akzeptierte in Runde 2 (Anregung: Hinweis im Hilfetext, nicht nötig).
- **„änderbar“ (Intent-Check Runde 2):** Beschreibungen und Hub-Einträge ließen sich nur über
  rm + add ändern, wobei Grants, Token oder `hub_collections` verloren gingen. Ergänzt um
  `set`-Kommandos; die Folgefragen zum Transportwechsel (R3-1, R3-2) entschied der Moderator.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| FEHLER-01 | FEHLER | 003 | Kontext „Protokoll“/„Tabellen genau nach Datenmodell“ | `actions` hat keine Spalte für das Ziel (Collection, Node) | Spalte ergänzen oder Ablage festlegen |
| WARNUNG-01 | WARNUNG | 003 | Etappe 2 „eigenes kleines Paket“ | Unter `internal/hub` dürfte der Node es nicht nutzen | neutrales Paket; `SYSTEM:A:`-Abfrage im Hub-Store |
| WARNUNG-02 | WARNUNG | 003 | Ziel, Flags nach dem Namen | `parseFlags` stoppt beim ersten Positionsargument | Flag-Reihenfolge festlegen |
| WARNUNG-03 | WARNUNG | 003 | Kontext „Export“ | Import könnte Collections mit Dokumenten entfernen | gleiche Prüfung wie `rm` |
| WARNUNG-04 | WARNUNG | 003 | Kontext „Entfernen“ | Offen, ob Löschmarken/`SYSTEM:` als Dokumente zählen | jede Zeile zählt |
| WARNUNG-05 | WARNUNG | 003 | Kontext Eindeutigkeit | Löschmarken bei `SYSTEM:A:` offen | jede Zeile, jede Collection |
| WARNUNG-06 | WARNUNG | 003 | Kontext „Export“ | Klartext-Token auf stdout ohne `--output` | `--output` verlangen oder warnen |
| FEHLEND-01 | FEHLEND | 003 | Export/Import | Import prüft nicht wie die CLI | dieselben Prüfungen vor dem Schreiben |
| FEHLEND-02 | FEHLEND | 003 | Transporte | Adressformat, Anzahl `local` offen | festlegen |
| FEHLEND-03 | FEHLEND | 003 | Protokoll | `actions` beim Import offen | `config.import`, `actions` nie ersetzen |
| FEHLEND-04 | FEHLEND | 003 | `<hub>:<collection>` | Collection-Teil nicht geprüft | beide Teile prüfen |
| FEHLEND-05 | FEHLEND | 003 | `hub_id`/Export | Offen, ob `hub_id` exportiert wird | nicht exportieren |
| NEU-01 | Unklarheit | 003 | Protokoll | `config.import` am Node, wo es kein `actions` gibt | nur Hub |
| NEU-02 | Unklarheit | 003 | Schemafassung 2 / Import | Format-1-Import könnte neue Tabellen leeren | nur `settings` ersetzen |
| ALIGN-01 | Intent | 003 | Ziel | Beschreibungen nicht änderbar | `set`-Kommandos |
| ALIGN-02 | Intent | 003 | Ziel | Hub-Einträge am Node nicht änderbar | `node hub set` |
| ALIGN-03 | Intent | 003 | Etappe 5 | `status` am Hub nur Anzahl Collections | Namen zeigen |
| R3-1 | Widerspruch | 003 | Transporte / `node hub set` | Wechsel auf `local` scheitert an alter Adresse; `--ssh-key` bei anderen Transporten offen | unpassende Felder verwerfen; Schlüssel nur bei `ssh` |
| R3-2 | Lücke | 003 | `hub_id` / `node hub set` | `hub_id` bleibt nach Adresswechsel stehen | Regel festlegen |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| FEHLER-01 | decide + pass | Datenmodell-Entscheidung, klein | `actions.subject` |
| WARNUNG-01 | decide + pass | blockiert wegen Trennregel | neutrales Paket |
| WARNUNG-02 | decide + pass | blockiert die Zielsyntax | Flags vor und nach Positionsargumenten |
| WARNUNG-03 | decide + pass | umgeht Regel von `rm` | Import bricht ab |
| WARNUNG-04 | decide + pass | Mehrdeutigkeit mit Folgen | jede Zeile zählt |
| WARNUNG-05 | decide + pass | Mehrdeutigkeit mit Folgen | jede Zeile, auch Löschmarken |
| WARNUNG-06 | skip | nicht blockierend, bestehendes Verhalten | Critic akzeptiert |
| FEHLEND-01 | pass | Import könnte CLI-Regeln umgehen | umgesetzt |
| FEHLEND-02 | decide + pass | Transportregeln unvollständig | umgesetzt |
| FEHLEND-03 | decide + pass | offen, klein | umgesetzt |
| FEHLEND-04 | pass | klar | umgesetzt |
| FEHLEND-05 | pass | klar, konzepttreu | umgesetzt |
| NEU-01 | decide + pass | klar | umgesetzt |
| NEU-02 | decide + pass | stiller Datenverlust möglich | umgesetzt |
| ALIGN-01..03 | decide + pass | Intent nicht erfüllt | umgesetzt |
| R3-1 | decide, Moderator-Edit | eng umrissen | umgesetzt |
| R3-2 | decide, Moderator-Edit | Konzept: `hub_id` wird beim Kontakt geprüft | umgesetzt |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| FEHLER-01 | umgesetzt | Ausnahme im Kontext, Etappe 1/2/6 |
| WARNUNG-01 | umgesetzt | Kontext „Gemeinsames Paket“, Etappe 2/3 |
| WARNUNG-02 | umgesetzt | Kontext + Etappe 4 mit Test |
| WARNUNG-03 | umgesetzt | „Import prüft wie die CLI“, Test Etappe 5 |
| WARNUNG-04 | umgesetzt | „Entfernen“, Test Etappe 2 |
| WARNUNG-05 | umgesetzt | Eindeutigkeit, Test Etappe 2 |
| FEHLEND-01 | umgesetzt | eigener Kontext-Punkt, Test Etappe 5 |
| FEHLEND-02 | umgesetzt | Transporte, Test Etappe 3 |
| FEHLEND-03 | umgesetzt | Protokoll/Export, Test Etappe 5 |
| FEHLEND-04 | umgesetzt | Namen, Test Etappe 3 |
| FEHLEND-05 | umgesetzt | `hub_id`, Test Etappe 5 |
| NEU-01 | umgesetzt | Protokoll, Test Etappe 5 |
| NEU-02 | umgesetzt | Export, Etappe 5 |
| ALIGN-01 | umgesetzt | Ziel, Protokoll, Tests Etappe 2/4 |
| ALIGN-02 | umgesetzt | Ziel, Transporte, Tests Etappe 3/4 |
| ALIGN-03 | umgesetzt | Etappe 5 |

### Moderator-Entscheidungen
- WARNUNG-06 übersprungen: `export` wird ausdrücklich aufgerufen, stdout ist das Verhalten aus
  Task 002, am Hub stehen nur Hashes. Critic akzeptierte.
- FEHLER-01: `actions.subject TEXT` statt Ziel in `document_id` zu schreiben; Abweichung vom
  Konzept wird in Etappe 6 nachgezogen.
- WARNUNG-02: Flags vor und nach Positionsargumenten statt die Zielsyntax umzustellen.
- R3-1/R3-2 ohne weitere Editor-Runde direkt eingearbeitet (zwei Sätze, eng umrissen):
  `--ssh-key` nur bei `ssh`, Transportwechsel verwirft unpassende Felder, `hub_id` bleibt und
  wird beim Kontakt geprüft (so in `docs/begriffe.md`).

### Intent-Alignment
Runde 2: Nein — „änderbar“ unvollständig, `status` am Hub nur Anzahl (behoben).
Runde 3: **Ja** — alle Intent-Punkte abgedeckt. Randnotiz: nicht ausdrücklich gesagt, dass
`internal/separation_test.go` das neue neutrale Paket mit abdeckt (ohne Folgen für den Intent).

### Geänderte Dateien
- 003-collections-nodes-hubs.md: `actions.subject`, neutrales Paket, Flag-Parsing,
  Import-Prüfungen und Format-1-Verhalten, Zählregeln für Dokumente und `SYSTEM:A:`,
  Transportformate, `set`-Kommandos, `status` mit Collection-Namen, Tests je Etappe
  (FEHLER-01, WARNUNG-01..05, FEHLEND-01..05, NEU-01/02, ALIGN-01..03, R3-1/2)

### Offen (nicht gefixt)
- WARNUNG-06: bewusst übersprungen (Hinweis im Hilfetext von `config export` wäre möglich).

---
## Review-Log (2026-09-25, Nachprüfung)

**Pfad:** k-playbook-local/tasks/003-collections-nodes-hubs.md
**Intent:** inline (`## Intent`)
**Runden:** 2
**Anlass:** Task ergänzt um „Nachbesserungen aus der Code-Review von Task 002“ (Kontext 1–4,
Etappe 1 und 5).

### Diskussion
- **IMMEDIATE statt Sperrzeile (R2-FEHLER-01):** Das `UPDATE … SET value = value` zuerst hilft
  unter SQLite nur, wenn es die erste Anweisung der DEFERRED-Transaktion ist; die Methoden aus
  Etappe 2/3 lesen aber vorher. Moderator entschied `_txlock=immediate` in der DSN (modernc
  v1.59 unterstützt es, kein PRAGMA) für SQLite; die Sperrzeile bleibt für PostgreSQL.
- **Format-1-Exporte nach Nachbesserung 4:** Critic prüfte, dass der Export aus Task 002 für
  jede Rolle ein nicht leeres Map-Objekt schreibt (`{}` bei leer); echte Format-1-Exporte
  bleiben importierbar, abgewiesen werden nur handgebaute Dateien ohne `settings`-Teil.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| R2-FEHLER-01 | FEHLER | 003 | Nachbesserung 1 / Etappe 1 | Sperrzeile vermeidet `SQLITE_BUSY` nur als erste Anweisung; Lesen-dann-Schreiben in Etappe 2/3 betroffen | Sperre als erste Anweisung oder `_txlock=immediate` |
| R2-WARNUNG-01 | WARNUNG | 003 | Nachbesserung 4 | Export muss jeden Teil immer schreiben; Decodieren unterscheidet fehlt/`null`/leer nicht | festlegen, Rundlauf-Test mit leeren Tabellen |
| R2-WARNUNG-02 | WARNUNG | 003 | Nachbesserung 3 / Etappe 5 | Reihenfolge der Rollen, Ort von `config.import`, Herbeiführen des Scheiterns offen | Hub zuerst, gleiche Transaktion, Eingriffspunkt |
| R2-WARNUNG-03 | WARNUNG | 003 | Nachbesserung 2 | WAL im gemeinsamen `dsn()`, bestehender Test erwartet `wal` nach `Open` | WAL nur in DSN von `Create`, Test anpassen |
| R2-FEHLEND-01 | FEHLEND | 003 | Nachbesserung 2 | Rest von Befund 3 (schreibgeschützte DB, `CheckInfo` vor Pragmas) offen | übernehmen oder ausschließen |
| R2-NEU-01 | WARNUNG | 003 | Etappe 6 | `k-playbook.md` beschreibt `sqlitedb` mit WAL beim Öffnen | Eintrag nachziehen |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| R2-FEHLER-01 | decide + pass | blockiert jede schreibende Transaktion unter Last | `_txlock=immediate`, Sperrzeile für PG |
| R2-WARNUNG-01 | decide + pass | sonst scheitert der Rundlauf | umgesetzt |
| R2-WARNUNG-02 | decide + pass | Test sonst nicht bestimmbar | umgesetzt |
| R2-WARNUNG-03 | decide + pass | klein, bestehender Test betroffen | umgesetzt |
| R2-FEHLEND-01 | decide + pass | Pragmas beim Öffnen ändern die Datei nicht mehr | „Nicht in diesem Task“ |
| R2-NEU-01 | decide, Moderator-Edit | eine Zeile in Etappe 6 | umgesetzt |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| R2-FEHLER-01 | umgesetzt | Nachbesserung 1 umformuliert, Test Etappe 1 (Lesen + `nextRevision` im Unterbau, da Etappe-2-Methoden dort noch fehlen) |
| R2-WARNUNG-01 | umgesetzt | Nachbesserung 4, Test Etappe 5 |
| R2-WARNUNG-02 | umgesetzt | Nachbesserung 3, Test Etappe 5 an Reihenfolge angepasst |
| R2-WARNUNG-03 | umgesetzt | Nachbesserung 2, Test Etappe 1 „`Create` legt mit `wal` an“ |
| R2-FEHLEND-01 | umgesetzt | „Nicht in diesem Task“ ergänzt |

### Moderator-Entscheidungen
- R2-FEHLER-01: `_txlock=immediate` statt „Sperre als erste Anweisung“ — gilt ohne Disziplin in
  jeder Methode; Treiberunterstützung in modernc v1.59 geprüft.
- R2-FEHLEND-01: `status` auf schreibgeschützte DB verschoben; bei Bedarf als Todo erfassen.
- R2-NEU-01 ohne Editor-Runde direkt eingearbeitet (eine Zeile).
- Intent-Check-Randnotizen übergangen: fehlende Schritte im CLI-Testablauf (revoke, unlock,
  hub token/show) decken die Store-Tests; die Trennung prüft `internal/separation_test.go`
  bereits.

### Intent-Alignment
**Ja** — alle Intent-Punkte abgedeckt; die Nachbesserungen widersprechen dem Intent nicht,
Punkte 3/4 stützen export/import, Punkte 1/2 vergrößern Etappe 1. Hinweis beim Ausführen:
Mit `_txlock=immediate` nimmt auch eine lesende Transaktion eine Schreibsperre.

### Geänderte Dateien
- 003-collections-nodes-hubs.md: Nachbesserungen 1–4 präzisiert, „Nicht in diesem Task“,
  Tests Etappe 1 und 5, Etappe 6 `sqlitedb`-Eintrag (R2-FEHLER-01, R2-WARNUNG-01..03,
  R2-FEHLEND-01, R2-NEU-01)

### Offen (nicht gefixt)
- `status` auf eine schreibgeschützte Datenbank (bewusst verschoben).
