# Task 009 — Lesen über MCP: list, read, changes

Der Node bietet Clients `list`, `read` und `changes` über der Replica an — genug für die
Erweiterung in VS Code und für die KI.

## Intent

Ein Client liest über MCP alles, was sein Account lesen darf, aus der Replica, ohne Netz —
so, dass die Erweiterung für VS Code darauf ihren `FileSystemProvider` bauen kann.
- Inhalte nur aus Collections, auf denen der gültig angemeldete Account `read` hat.
- `list` bildet Verzeichnisse, `read` mit `content: false` beantwortet `stat`, `changes`
  erlaubt lückenloses Weiterfragen.
- `SYSTEM:`-Namen erscheinen nie; Löschmarken nur in `changes`.
- Keine Werkzeuge nur für VS Code.

## Referenzen

- `docs/konzept.md` — „Werkzeuge“ → „Allgemein — lesen“ (Adresse, `list`, `read`,
  `changes`), „Abgleich“.
- `docs/vscode.md` — Tabelle „Die Vorgänge und ihre Entsprechung“.
- `docs/begriffe.md` — name, mask, tool, document.
- `internal/node/mcpnode/mcpnode.go` (`Check`), `internal/node/replica/replica.go`
  (Abfragen, `sync_state`), `cmd/kephalaion/doccmd.go` (`node doc list|get`, Verzeichnisse
  aus Namen), `internal/ident` (`ParseAddress`, `DocDirPrefix`).

## Kontext

- **Voraussetzung:** Task 006 und 008 abgeschlossen (User als Urheber, Anmeldung über alle
  Hubs als gemeinsame Funktion in `mcpnode`, Abgleich im Hintergrund).
- **Adresse:** `collection` als `<hub>:<collection>`; Hub-Teil darf fehlen, wenn der Client
  an genau einem Hub gültig angemeldet ist, sonst Fehler mit den möglichen Hubs. Darin
  `path` (Verzeichnis) bzw. `name` (Dokument).
- **Anmeldung und Recht** je Aufruf über `Check`. Die Rechte stehen in den `SYSTEM:A:`-Zeilen
  der Replica, es zählen also nur Collections, die der Node führt. Ohne gültige Anmeldung
  oder ohne `read` dieselbe Meldung „nicht lesbar“, gleich ob die Collection existiert —
  auch bei Recht am Hub auf eine Collection, die der Node nicht führt. Fehlt die Replica des
  Hubs: eigene Meldung „noch nie abgeglichen“ wie `sync` in `whoami`, nicht leer.
- **Nie:** `SYSTEM:`-Namen, Token, Hash; Löschmarken nicht in `list`/`read`.
- **`list`:** ohne `collection` die lesbaren Collections aller gültig angemeldeten Hubs (Art
  `collection`, Adresse); mit `collection: "<hub>:"` (Wurzel eines Hubs) nur die dieses Hubs.
  Sonst `path` (leer = Wurzel), `recursive` (Standard nein); ohne
  `recursive` zuerst die Verzeichnisse der nächsten Ebene (Art `directory`, nur Name, nach
  Name), abgeleitet aus den Namen darunter. `sort` `name`|`created`|`updated`, `order`
  `asc`|`desc`, `limit` (Standard und Höchstwert festlegen), `cursor` (undurchsichtig; ohne
  Lücke und Doppel nur bei unveränderter Replica — Änderungen dazwischen deckt `changes`).
  `mask`: Glob auf das letzte Segment, nur für Dokumente, `*` nicht über `/`, in Go
  ausgewertet. Je Dokument: Name, `id`, Revision, angelegt und geändert (wann, von wem),
  Größe. Abfrage über den Präfix auf dem Index `(collection, name)`.
- **`read`:** `collection` + `name`, oder `id` (Hub nötig, wenn mehrere); `content`
  (Standard ja). Antwort mit Art `document`, `directory` oder `none` — `none` ist kein Fehler.
  Dokument: Name, `id`, Revision, angelegt/geändert, Größe in Byte, `writable` (`write` auf
  der Collection), Inhalt als Text des Ergebnisses. Verzeichnis: existiert, wenn ein lebendes
  Dokument darunter liegt. Die lesbare Wurzel eines Hubs (`<hub>:`) oder einer Collection
  (leerer `name`) ist `directory`. Abschnitte kommen mit der Suche.
- **`changes`:** `collection` (ohne: alle lesbaren aller angemeldeten Hubs), `path` als
  Präfix, `cursor` oder `since` (RFC 3339), `limit`. Ohne `cursor` und `since` nur der Cursor
  für „ab jetzt“. Je geändertem Dokument einmal der aktuelle Stand: Adresse, Name, `id`,
  Revision, gelöscht ja/nein, geändert (wann, von wem); kein alter Name. Dazu neuer Cursor
  und „mehr“ ja/nein.
  - **Lückenlos:** Der Cursor trägt den Stand je Collection — eine nachhinkende Collection
    wird nicht übersprungen, eine neu lesbare liefert alles. Nach Neuanlage der Replica
    (`hub_id`-Wechsel, zurückgespielte Sicherung) meldet `changes` `reset`. Die Neuanlage muss
    auch bei gleicher `hub_id` erkennbar sein: `reset` leert die Replica in derselben Datei,
    nach einer Sicherung bleibt die `hub_id` und die Revisionen kommen neu. Der Cursor trägt
    darum je Hub eine Kennung, die bei jeder Neuanlage und jedem `reset` seiner Replica
    wechselt; `reset` gilt je Hub, die anderen laufen lückenlos weiter. Replica-Schemafassung
    +1, ohne Migration. `docs/konzept.md` zieht `changes` über alle Hubs nach.
  - Fällt eine Collection aus dem Cursor weg (Recht entzogen, am Node abgewählt, entfernt),
    meldet `changes` sie als weggefallen — ihre Dokumente verschwinden ohne Löschmarke.
  - `since` ist die Zeit des Hubs beim Schreiben, nicht des Abgleichs — nicht lückenlos.
- **Beschreibungen** kurz und deutsch wie bei `whoami`; jedes Wort geht in den Kontext der KI.
- `node doc list|get` bleiben; gemeinsame Abfragen in `replica`, wo es passt.
- **Nicht in diesem Task:** `search`, Abschnitte, Schreiben, die Erweiterung selbst.

## Zu bauen

### Etappe 1 — Adresse, Anmeldung, Abfragen

- Adresse auflösen, Anmeldung und Recht als gemeinsamer Schritt; Abfragen in `replica`.
- Tests: mit/ohne Hub-Teil, mehrere Hubs, ohne Recht = unbekannt, Collection nicht am Node =
  unbekannt, Replica fehlt = „noch nie abgeglichen“.

### Etappe 2 — `list`

- Tests: Verzeichnisse, `recursive`, Wurzel = Collections, `<hub>:` = Collections des Hubs,
  Sortierung und Richtung, `limit` mit `cursor` ohne Lücke und Doppel bei unveränderter
  Replica, `mask` (`*.md`, `0*-*.md`, nicht über `/`), `SYSTEM:`
  und Löschmarken fehlen.

### Etappe 3 — `read`

- Tests: per Name und `id`, `content: false`, Verzeichnis, Wurzel von Hub und Collection =
  `directory`, `none`, Löschmarke = `none`, `writable`.

### Etappe 4 — `changes`

- Tests: „ab jetzt“, Cursor über zwei Collections mit verschiedenem Stand, neue Collection,
  Löschmarke, Umbenennen (gleiche `id`), mehrfach geändert = einmal, `reset`, `reset` nach
  Sicherung mit gleicher `hub_id`, `reset` eines von zwei Hubs, weggefallene Collection (Recht entzogen, abgewählt),
  `since`.

### Etappe 5 — Durchlauf und Doku

- `serve` mit Hub und Node, MCP-Client des go-sdk: Dokument am Hub → Abgleich im Hintergrund
  → `changes` meldet es → `read` liefert es.
- `README.md`, `docs/begriffe.md`, `docs/konzept.md` (Stand), `docs/vscode.md` (Stand),
  `k-playbook-local/k-playbook.md` (Werkzeuge in `mcpnode`), `docs/fortschritt.md`.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — Adresse, Anmeldung, Abfragen | offen | | |
| 2 — `list` | offen | | |
| 3 — `read` | offen | | |
| 4 — `changes` | offen | | |
| 5 — Durchlauf und Doku | offen | | |

---
## Review-Log (2026-09-26)

**Pfad:** k-playbook-local/tasks
**Intent:** inline
**Runden:** 2 (gemeinsam mit Task 008)

### Diskussion
- **1/N3 (reset erkennen):** Der Critic zeigte, dass ein Cursor aus `hub_id` und Revision eine zurückgespielte Sicherung mit gleicher `hub_id` nicht erkennt: Die Revisionen kommen neu und werden still übersprungen. Der Editor übernahm deshalb die Anforderung einer Kennung der Replica-Generation im Cursor, ohne den Ort festzulegen. Runde 2 präzisierte: je Hub, und `reset` je Hub.
- **3 (nicht abgeglichen):** Die Rechte stehen in `SYSTEM:A:`-Zeilen der Replica. Der Fall „Recht, aber nicht abgeglichen“ ist so nicht prüfbar. Der Editor bestätigte das am Code (`accountRows`, `dropUnwanted`); unterschieden wird jetzt nur „Replica fehlt“.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| 1 | FEHLEND | 009 | changes, `reset` | Die Neuanlage ist bei gleicher `hub_id` nicht erkennbar | Kennung der Generation im Cursor, Test |
| 2 | FEHLEND | 009 | changes, Cursor | Eine weggefallene Collection verschwindet ohne Meldung | als weggefallen melden, Test |
| 3 | WARNUNG | 009 | Anmeldung und Recht | „Recht, aber nicht abgeglichen“ ist nicht prüfbar | nach Prüfbarem fassen |
| 6 | WARNUNG | 009 | Etappe 2, Cursor | „ohne Lücke und Doppel“ ist bei laufendem Abgleich nicht erreichbar | auf unveränderte Replica eingrenzen |
| 7 | WARNUNG | 009 | list/read | Wurzel eines Hubs fehlt; `read` auf eine Wurzel ist offen | Form festlegen, `directory` |
| 10 | WARNUNG | 008/009 | Zusammenspiel | Anmeldung über alle Hubs womöglich doppelt gebaut | gemeinsame Funktion |
| N3 | Lücke | 009 | changes, Kennung | Kennung in der Einzahl bei mehreren Hubs | je Hub, `reset` je Hub, Test |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| 1 | pass | gefährdet „lückenlos“ | behoben |
| 2 | pass | gefährdet „lückenlos“ | behoben |
| 3 | pass | Anforderung so nicht umsetzbar | behoben |
| 6 | pass | Zusage nicht erfüllbar | behoben |
| 7 | pass | vom Konzept und von vscode.md verlangt | behoben |
| 10 | pass | vermeidet doppelte Umsetzung | behoben |
| N3 | decide | enge Klarstellung | behoben |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| 1 | fixed | Kennung der Generation als Anforderung, Replica-Schemafassung +1, Test mit gleicher `hub_id` |
| 2 | fixed | `changes` meldet eine weggefallene Collection; Test |
| 3 | fixed | am Code bestätigt; nur Collections des Nodes zählen; Replica fehlt = „noch nie abgeglichen“ |
| 6 | fixed | Zusage gilt nur bei unveränderter Replica |
| 7 | fixed | `collection: "<hub>:"` passt zur Adressform; `read` auf eine Wurzel ergibt `directory` |
| 10 | fixed | die Voraussetzung verweist auf die gemeinsame Funktion aus 008 |

### Moderator-Entscheidungen
- N3 aus Runde 2 hat der Moderator direkt eingetragen, ohne eine weitere Editor-Runde: Es ist eine enge Klarstellung.

### Intent-Alignment
Ja. Gelesen wird nur aus der Replica, mit `Check` je Aufruf und derselben Meldung für „nicht lesbar“. `list` bildet Verzeichnisse. `read` mit `content: false` reicht für `stat`. `changes` ist lückenlos (Stand je Collection, Kennung je Hub, `reset`, weggefallene Collections). `SYSTEM:` erscheint nie, Löschmarken nur in `changes`, und es gibt keine Werkzeuge nur für VS Code.

### Geänderte Dateien
- 009-lesen-ueber-mcp.md: Voraussetzung (10), Anmeldung und Recht (3), `list` mit `<hub>:` und Cursor-Zusage (6, 7), `read` auf eine Wurzel (7), `changes` mit Kennung je Hub, `reset` je Hub und weggefallener Collection (1, 2, N3), Tests in den Etappen 1 bis 4

### Offen (nicht gefixt)
- keine
