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
| 1 — Adresse, Anmeldung, Abfragen | erledigt | 2026-09-26 | `mcpnode/access.go`: `resolve` (Hub-Teil optional), `open`/`eachValid` über `Authenticate`, eine Meldung „nicht lesbar“, „noch nie abgeglichen“, unlesbare Replica nur für ihren Hub; `replica/read.go` (Abfragen ohne Transaktion); `generation` in `db_info`, Schemafassung 4 |
| 2 — `list` | erledigt | 2026-09-26 | `mcpnode/list.go`: Collections (alle Hubs, `<hub>:`), Verzeichnisse zuerst, Dokumente nach `name`/`created`/`updated`, Keyset-Cursor mit Fingerabdruck der Anfrage, `limit` Standard 100, höchstens 1000; `mask` per `path.Match` auf das letzte Segment; `unreadable_hubs` |
| 3 — `read` | erledigt | 2026-09-26 | `mcpnode/read.go`: per Name oder `id` (Hub-Teil entbehrlich bei einem Hub), Inhalt als Text des Ergebnisses, `content: false` nur Angaben; Verzeichnis = lebendes Dokument darunter, Wurzel von Hub und Collection `directory`; Löschmarke, fremde und unbekannte id `none`; `writable` = `write` |
| 4 — `changes` | erledigt | 2026-09-26 | `mcpnode/changes.go`: cursor je Hub (`generation`) und je Collection (Revision, bei halber Revision die letzte id), bis zum Stand in `sync_state`; ab jetzt, `since` (erste Zeile ab dem Zeitpunkt), neu lesbar = alles, `reset` je Hub, `dropped`, `unreadable_hubs` behält den Stand; cursor an `collection`/`path` gebunden |
| 5 — Durchlauf und Doku | erledigt | 2026-09-26 | `TestMCPReadThroughServe` (`cmd/kephalaion/mcp_test.go`): `serve` mit Hub und Node, `hub doc put` → Abgleich im Hintergrund → `changes` → `read`/`list` über den MCP-Client des go-sdk; Doku in README, `begriffe.md`, `konzept.md` („Allgemein — lesen“: Festlegungen), `vscode.md`, `fortschritt.md`, `k-playbook.md` |

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

## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** 2026-09-26  
**Zusammenfassung:** Der Node bietet über MCP neu `list`, `read` und `changes` über der Replica an. Adresse und Anmeldung löst `mcpnode/access.go` auf Basis von `Authenticate` auf, die Abfragen stehen in `replica/read.go`. `changes` ist lückenlos: Der Cursor trägt je Hub die Generation und je Collection die Revision, dazu kommen `reset` je Hub, `dropped` und `unreadable_hubs`. Die Replica hat Schemafassung 4 mit `generation` in `db_info`. Ein Durchlauf über `serve` mit dem MCP-Client des go-sdk ist als `TestMCPReadThroughServe` festgehalten. Die Doku ist nachgezogen in README, begriffe, konzept (Festlegungen: `limit` 100/1000, Cursor-Format, Zeiten), vscode, fortschritt und k-playbook.md. `make check` ist grün, ebenso `-race` über `internal/node/...` und die MCP-Tests. Commits: 7db6a33, 208aed3, c1e0435, b4690d8, 745d8fd.

**Geänderte Dateien** (`git diff 6bcbeaf 745d8fd --stat`, ohne den fremden Commit 778f313 `vscode/*`, ohne Task 013 und `todos.json`):
```
 README.md                                     |  33 +++-
 cmd/kephalaion/mcp_test.go                    |  81 ++++++++++
 docs/begriffe.md                              |  31 +++-
 docs/fortschritt.md                           |  17 ++-
 docs/konzept.md                               |  77 ++++++++--
 docs/vscode.md                                |  29 +++-
 internal/node/mcpnode/access.go               | 307 +++++++++++++++++
 internal/node/mcpnode/access_test.go          | 167 ++++++++++
 internal/node/mcpnode/changes.go              | 376 +++++++++++++++++++++
 internal/node/mcpnode/changes_test.go         | 354 +++++++++++++++++++
 internal/node/mcpnode/cursor.go               |  69 +++++
 internal/node/mcpnode/docenv_test.go          | 261 ++++++++++++++
 internal/node/mcpnode/list.go                 | 312 +++++++++++++++++
 internal/node/mcpnode/list_test.go            | 275 +++++++++++++++
 internal/node/mcpnode/mcpnode.go              |  11 +-
 internal/node/mcpnode/mcpnode_test.go         |  14 +-
 internal/node/mcpnode/read.go                 | 146 ++++++++
 internal/node/mcpnode/read_test.go            | 132 ++++++++
 internal/node/replica/read.go                 | 319 +++++++++++++++++
 internal/node/replica/read_test.go            | 215 ++++++++++++
 internal/node/replica/replica.go              |  50 ++--
 k-playbook-local/k-playbook.md                |  17 ++-
 k-playbook-local/tasks/009-lesen-ueber-mcp.md |  10 +-
 23 files changed, 3246 insertions(+), 57 deletions(-)
```

**Code-Änderungen:** Der Diff umfasst ca. 3700 Zeilen, fast nur neue Dateien und davon rund die Hälfte Tests. Deshalb gibt es hier nur einen Überblick statt Hunks:
- `mcpnode/access.go`: `resolve` (Adresse, Hub-Teil optional), `open`/`openHub`/`eachValid`, `toolFailure` (Meldungen „nicht lesbar“, „noch nie abgeglichen“, unlesbare Replica ohne Pfad).
- `mcpnode/list.go`: Collections, Verzeichnisse der nächsten Ebene, dann die Dokumente. Der Keyset-Cursor hat einen Fingerabdruck der Anfrage, `mask` läuft über `path.Match`.
- `mcpnode/read.go`: per Name oder `id`; ergibt `document`, `directory` oder `none`; `writable`.
- `mcpnode/changes.go`, `cursor.go`: Cursor je Hub (`generation`) und je Collection (Revision, letzte `id`). Gelesen wird bis zum Stand in `sync_state`; dazu `since`, `reset`, `dropped`, `unreadable_hubs`.
- `replica/read.go`, `replica.go`: Abfragen ohne Transaktion und `generation` in `db_info`. `Create` vergibt sie, `reset` ersetzt sie. Die Schemafassung steigt von 3 auf 4.

**Offene Punkte aus der Ausführung:**
- `changes` mit `path` sieht keine Umbenennung aus dem Verzeichnis heraus; das ist dokumentiert.
- Eine Sicherung am Hub mit Weiterschreiben vor dem Abgleich wird nicht erkannt; das ist eine Grenze des Abgleichs.
- „Unlesbar mitten im Lesen“ ist nicht getestet.
- Commit 208aed3 enthält einen fremden Hunk der Konzept-Sitzung in `docs/begriffe.md` (`--token-file`, `tokens/<hub>/<account>.token`).

**Code-Review:**
Grundlage ist nur der Diff (engineering:code-review).
1. **Warnung:** `changes.go`, `unreadable()`. Wird eine Replica erst nach einer vollen Seite unlesbar, bleibt `More = true` stehen.
   - Folge: Die nachfolgenden Hubs werden ausgehungert. Ist der Lesefehler dauerhaft, pollt der Client endlos.
   - Empfehlung: `More` und `len(Changes)` vor dem Hub sichern und in `unreadable()` wiederherstellen.
2. **Warnung:** Der Fingerabdruck in `changes` und `list` bindet den rohen Wert von `collection`, nicht das aufgelöste Ziel.
   - Bei einer Adresse ohne Hub-Teil kann der Hub wechseln. Dann fällt der alte Hub ohne `dropped` oder `reset` aus dem Cursor, und `list` blättert in einer anderen Collection weiter.
   - Empfehlung: den Fingerabdruck über `t.Address()` bilden.
3. **Warnung:** `access.go`, `toolFailure`. Bei abgebrochenem ctx geht `err` unverändert an den Client und kann einen Pfad enthalten.
   - Empfehlung: nur `ctx.Err()` oder eine feste Meldung zurückgeben.
4. **Vorschlag:** Ein Hub, der beim Aufruf „ab jetzt“ unlesbar war, fehlt im Cursor und liefert später seine ganze Historie. Das sollte dokumentiert oder der Startstand festgelegt werden.
5. **Vorschlag:** Die Annahme prüfen und testen, dass `sync_state.revision` nur über vollständig angewendete Revisionen vorrückt. Sonst entsteht in `changes` eine Lücke.
6. **Vorschlag:** `readByName` liefert für `"a.md/"` das Dokument `a.md`. Ein `/` am Ende sollte nur `directory` oder `none` ergeben.
7. **Vorschlag:** `read` per `id` liefert eine verdeckte, ältere Zeile gleichen Namens. `list` und `read` per Name verbergen sie über `newest`; das ist inkonsistent.
8. **Vorschlag (Leistung):**
   - `qChanged` materialisiert über `length(CAST(content AS BLOB))` den Inhalt.
   - `ChildDirs` durchläuft je Seite alle Namen unter dem Präfix.
   - Für `sort` `created`/`updated` gibt es keinen Index.
   - `changes` ohne `collection` öffnet bei jedem Poll alle Replicas.

Gesamturteil: solide und gut getestet. Die Befunde 1 bis 3 sollten vor dem nächsten Release behoben werden.
