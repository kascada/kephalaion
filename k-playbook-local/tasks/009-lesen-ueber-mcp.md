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

- **Voraussetzung:** Task 006 und 008 abgeschlossen (User als Urheber, derselbe Code in
  `mcpnode`, Abgleich im Hintergrund).
- **Adresse:** `collection` als `<hub>:<collection>`; Hub-Teil darf fehlen, wenn der Client
  an genau einem Hub gültig angemeldet ist, sonst Fehler mit den möglichen Hubs. Darin
  `path` (Verzeichnis) bzw. `name` (Dokument).
- **Anmeldung und Recht** je Aufruf über `Check`. Ohne gültige Anmeldung oder ohne `read`
  dieselbe Meldung „nicht lesbar“, gleich ob die Collection existiert. Mit Recht, aber noch
  nicht abgeglichen: eigene Meldung, nicht leer.
- **Nie:** `SYSTEM:`-Namen, Token, Hash; Löschmarken nicht in `list`/`read`.
- **`list`:** ohne `collection` die lesbaren Collections aller gültig angemeldeten Hubs (Art
  `collection`, Adresse). Sonst `path` (leer = Wurzel), `recursive` (Standard nein); ohne
  `recursive` zuerst die Verzeichnisse der nächsten Ebene (Art `directory`, nur Name, nach
  Name), abgeleitet aus den Namen darunter. `sort` `name`|`created`|`updated`, `order`
  `asc`|`desc`, `limit` (Standard und Höchstwert festlegen), `cursor` (undurchsichtig).
  `mask`: Glob auf das letzte Segment, nur für Dokumente, `*` nicht über `/`, in Go
  ausgewertet. Je Dokument: Name, `id`, Revision, angelegt und geändert (wann, von wem),
  Größe. Abfrage über den Präfix auf dem Index `(collection, name)`.
- **`read`:** `collection` + `name`, oder `id` (Hub nötig, wenn mehrere); `content`
  (Standard ja). Antwort mit Art `document`, `directory` oder `none` — `none` ist kein Fehler.
  Dokument: Name, `id`, Revision, angelegt/geändert, Größe in Byte, `writable` (`write` auf
  der Collection), Inhalt als Text des Ergebnisses. Verzeichnis: existiert, wenn ein lebendes
  Dokument darunter liegt. Abschnitte kommen mit der Suche.
- **`changes`:** `collection` (ohne: alle lesbaren aller angemeldeten Hubs), `path` als
  Präfix, `cursor` oder `since` (RFC 3339), `limit`. Ohne `cursor` und `since` nur der Cursor
  für „ab jetzt“. Je geändertem Dokument einmal der aktuelle Stand: Adresse, Name, `id`,
  Revision, gelöscht ja/nein, geändert (wann, von wem); kein alter Name. Dazu neuer Cursor
  und „mehr“ ja/nein.
  - **Lückenlos:** Der Cursor trägt den Stand je Collection — eine nachhinkende Collection
    wird nicht übersprungen, eine neu lesbare liefert alles. Nach Neuanlage der Replica
    (`hub_id`-Wechsel, zurückgespielte Sicherung) meldet `changes` `reset`.
  - `since` ist die Zeit des Hubs beim Schreiben, nicht des Abgleichs — nicht lückenlos.
- **Beschreibungen** kurz und deutsch wie bei `whoami`; jedes Wort geht in den Kontext der KI.
- `node doc list|get` bleiben; gemeinsame Abfragen in `replica`, wo es passt.
- **Nicht in diesem Task:** `search`, Abschnitte, Schreiben, die Erweiterung selbst.

## Zu bauen

### Etappe 1 — Adresse, Anmeldung, Abfragen

- Adresse auflösen, Anmeldung und Recht als gemeinsamer Schritt; Abfragen in `replica`.
- Tests: mit/ohne Hub-Teil, mehrere Hubs, ohne Recht = unbekannt, nicht abgeglichen.

### Etappe 2 — `list`

- Tests: Verzeichnisse, `recursive`, Wurzel = Collections, Sortierung und Richtung, `limit`
  mit `cursor` ohne Lücke und Doppel, `mask` (`*.md`, `0*-*.md`, nicht über `/`), `SYSTEM:`
  und Löschmarken fehlen.

### Etappe 3 — `read`

- Tests: per Name und `id`, `content: false`, Verzeichnis, `none`, Löschmarke = `none`,
  `writable`.

### Etappe 4 — `changes`

- Tests: „ab jetzt“, Cursor über zwei Collections mit verschiedenem Stand, neue Collection,
  Löschmarke, Umbenennen (gleiche `id`), mehrfach geändert = einmal, `reset`, `since`.

### Etappe 5 — Durchlauf und Doku

- `serve` mit Hub und Node, MCP-Client des go-sdk: Dokument am Hub → Abgleich im Hintergrund
  → `changes` meldet es → `read` liefert es.
- `README.md`, `docs/begriffe.md`, `docs/konzept.md` (Stand), `docs/vscode.md` (Stand),
  `k-playbook-local/k-playbook.md` (Werkzeuge in `mcpnode`), `docs/fortschritt.md`.
