# Task 008 — Abgleich im Hintergrund, whoami, node whoami

`serve` gleicht die Replicas selbst ab; `whoami` zeigt Version, alle Hubs, Anmeldung und
Stand des Abgleichs; `kephalaion node whoami` zeigt dasselbe auf der Kommandozeile.

## Intent

Ein laufender Node hält seine Replicas ohne Zutun aktuell, und Clients wie Menschen sehen an
einer Stelle, wer sie sind und wie der Node steht.
- Eine Änderung am Hub ist spätestens nach einem Abstand (Standard 30 s) in der Replica,
  ohne `node sync`.
- Fehlschläge stehen im Log und je Hub in der Node-DB; `whoami` und `status` zeigen dasselbe.
- `whoami` liefert die Felder aus `docs/konzept.md`, „`whoami` — festgelegt am 2026-09-26“ —
  nie Token, Hash, Adresse, Transport oder `hub_id`.
- `kephalaion node whoami` listet die bekannten Accounts und zeigt mit Account, was `whoami`
  ihm antworten würde — aus derselben Funktion wie das Werkzeug.

## Referenzen

- `docs/konzept.md` — „Abgleich“ (Im Hintergrund), „Werkzeuge“ (`whoami` — festgelegt am
  2026-09-26, Adresse).
- `docs/vscode.md` — „Status und Auswahl“: wofür die Erweiterung `whoami` braucht.
- `docs/begriffe.md` — serve, sync, settings, whoami, check.
- `k-playbook-local/k-playbook.md` — Aufbau, `serve`, MCP am Node, keine Migrationen.
- `k-playbook-local/tasks/006-user-je-account.md` — liefert den User.
- `cmd/kephalaion/serve.go`, `cmd/kephalaion/synccmd.go` (`connector`, `localHub`),
  `internal/node/replica/sync.go` (`Syncer`), `internal/node/mcpnode/mcpnode.go` (`whoami`,
  `Check`), `internal/node/store/store.go`.

## Kontext

- **Voraussetzung:** Task 006 abgeschlossen (User in `whoami`).
- **Stand heute:** `serve` gleicht nicht ab, `local` ist in `serve` nicht verdrahtet.
  `whoami` nennt nur Hubs mit Header-Paar, Feld `authenticated`.
- **Abstand:** Schlüssel `sync_interval` in den `settings` des Nodes, Go-Dauer (`30s`),
  Standard 30 s, kleinster Wert 1 s, `0` schaltet den Abgleich im Hintergrund ab. Je Runde
  gelesen. Setzen über neues `kephalaion config set <rolle> <schlüssel> <wert>` bzw.
  `config unset <rolle> <schlüssel>` — nur bekannte Schlüssel, Wert geprüft; `config show`
  zeigt sie wie bisher.
- **Ablauf:** Beim Start von `serve` (Rolle node) je Hub-Eintrag ein Abgleich, danach je
  Abstand. Hub-Einträge je Runde aus der DB — `node hub add|rm` wirkt ohne Neustart. Ein
  langsamer oder hängender Hub hält die anderen nicht auf. `https`/`ssh` werden übergangen,
  mit einer Logzeile beim Start, nicht je Runde. Beim Beenden bricht ein laufender Abgleich
  ab; jede Seite ist eine Transaktion.
- **Verdrahtung:** wie `node sync` über `connector` in `cmd/kephalaion`. `local` braucht den
  Hub der eigenen config — läuft er im selben `serve`, dessen geöffneter Store, sonst
  geöffnet wie bei `node sync`. `internal/node` kennt weiter nur `contract.Hub`.
- **Nebenläufigkeit:** `node sync`, `node hub rm` und `config import` laufen neben `serve`.
  Prüfen und absichern: gleichzeitiger Abgleich aus Hintergrund und CLI auf dieselbe
  Replica; Replica wird während eines Abgleichs entfernt oder neu angelegt (Reset bei
  `hub_id`-Wechsel); `node hub rm` und `node hub add` mit gleichem Alias während eines
  Abgleichs — der alte Abgleich schreibt weder Stand noch `hub_id` in den neuen Eintrag.
  Mindestens: kein falscher Stand, kein Absturz, die nächste Runde stimmt. Die Beteiligten
  sind eigene Prozesse: Eine Sperre im Speicher reicht nicht — sie muss über Prozesse wirken
  (wie `takeLock` in `serve`), oder das Schreiben ist an eine Kennung des Hub-Eintrags
  gebunden, die nie wiederkehrt (beim Anlegen vergeben; nicht Alias, rowid oder Pfad).
- **Log:** keine Zeile für einen Abgleich ohne Änderung; eine Zeile, wenn Dokumente kamen
  (Hub, Anzahl, Revision). Ein Fehler beim ersten Fehlschlag je Hub nach dem Start von
  `serve`, beim Übergang von Erfolg zu Fehler und wenn sich die Art des Fehlers ändert (nicht
  bloß wechselnde Teile der Meldung), sonst still; eine Zeile, wenn es danach wieder geht. Nie ein
  Token.
- **Stand je Hub:** eigene Tabelle in der Node-DB (Hub, letzter Erfolg, letzter Fehler mit
  Zeit), geschrieben vom Hintergrund und von `node sync`. Abgeleitet: nicht im Export;
  `node hub rm` und `config import` räumen mit ab. Node-Schemafassung +1, ohne Migration.
- **whoami:** `version`; je **Hub-Eintrag des Nodes** `hub`, `login` (`ok`/`invalid`/
  `missing`), `node`, `sync` (letzter Erfolg, Revision der Replica, letzter Fehler mit Zeit);
  bei `ok` zusätzlich `account`, `user`, `collections` (Collection, Adresse, Rechte).
  `authenticated` entfällt. `invalid` trennt nicht zwischen unbekanntem Account, falschem
  Token und gesperrt; ein halbes Header-Paar ist `invalid`. Revision der Replica = Stand, bis
  zu dem alle ihre Collections abgeglichen sind; ohne Replica leer („noch nie abgeglichen“).
  Es bleibt bei den drei Werten: Fehlt die Replica und kam ein Header-Paar, ist `login`
  `invalid`; den Grund erkennen Client und Erweiterung an `sync`. Der Textteil bleibt eine
  Zeile je Hub. Header zu Aliasen, die der Node nicht kennt, werden gemeldet — nur der Alias,
  eigenes Feld (Hilfe bei falsch eingerichteten Clients); `docs/konzept.md` zieht nach.
- **Anmeldung über alle Hubs** je Anfrage: eine gemeinsame Funktion in `mcpnode` (über
  `Check`), die `whoami` nutzt und auf die Task 009 aufsetzt (Adresse ohne Hub-Teil, `list`
  ohne `collection`).
- **Eine Funktion für MCP und CLI** baut die Antwort; die CLI setzt für den genannten Account
  `login: ok` dort, wo er lebende `SYSTEM:A:`-Zeilen hat, sonst `missing`.
- **`kephalaion node whoami`:** ohne Argument Version, je Hub Node-Name und Stand, dazu die
  Accounts aus den lebenden `SYSTEM:A:`-Zeilen mit User, Collections und Rechten;
  `node whoami <account>` die Antwort von `whoami` für diesen Account, `--hub <alias>` grenzt
  ein, `--json` gibt die Struktur des Werkzeugs aus. Ohne Token — wer die CLI aufruft, kann
  die DBs ohnehin lesen; ob ein Token gilt, prüft weiter `node account check`.
- **`status`** zeigt am Node je Hub den Stand aus derselben Tabelle.
- **Nicht in diesem Task:** Ereignisstrom (Todo #10), Abgleich unmittelbar nach eigenem
  Schreiben (kommt mit dem Schreiben), `list`/`read`/`changes` (Task 009).

## Zu bauen

### Etappe 1 — Stand des Abgleichs und `sync_interval`

- Tabelle, Schemafassung, `node sync` schreibt, `status` zeigt, Aufräumen bei `hub rm`/Import.
- `config set|unset`, erster Schlüssel `sync_interval`.
- Tests: Erfolg und Fehler landen in der Tabelle, Fehler nach Erfolg leer; unbekannter
  Schlüssel und ungültige Dauer abgewiesen; Stand nicht im Export.

### Etappe 2 — Abgleich im Hintergrund

- Schleife in `serve`, Verdrahtung `local` und `http`, Log wie oben.
- Tests über `local` und `http`: `hub doc put` → nach dem Abstand in der Replica; Hub nicht
  erreichbar → Fehler in Tabelle und Log, danach Erholung; `sync_interval 0`; `node hub add`
  während `serve`; gleichzeitiger `node sync`; `node hub rm` + `node hub add` gleicher Alias
  während eines Abgleichs; Beenden während eines Abgleichs; Log: erster Fehler nach dem Start,
  gleicher Fehler je Runde nur einmal.

### Etappe 3 — `whoami`

- Antwort nach Konzept, eine Funktion für MCP und CLI.
- Tests über MCP: alle Hubs, `ok`/`invalid`/`missing`, Header-Paar ohne Replica = `invalid`
  mit „noch nie abgeglichen“, Header zu unbekanntem Alias gemeldet, User, Node-Name, Stand,
  Version; in der
  rohen Antwort kein Token, Hash, Adresse, Transport, `hub_id`.

### Etappe 4 — `kephalaion node whoami`

- Liste und Einzelansicht, `--hub`, `--json`.
- Tests: Liste nach `rotate`, gesperrter Account fehlt, `--json` gleich der MCP-Antwort.

### Etappe 5 — Doku

- `README.md`, `docs/begriffe.md` (`sync_interval`, `config set`, `node whoami`, whoami),
  `docs/konzept.md` (Stand; `whoami`-Tabelle um das Feld für unbekannte Aliase, Satz zum
  unbekannten Alias unter „Zustandslos“), `docs/vscode.md` (Stand), `k-playbook-local/k-playbook.md`
  (Satz „kein Abgleich im Hintergrund“ ersetzen), `docs/fortschritt.md`.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — Stand des Abgleichs und `sync_interval` | erledigt | 2026-09-26 | Tabelle `hub_sync`, `hubs.entry_id`, Node-Schema 4, Replica-Schema 3; `config set/unset`; make check grün |
| 2 — Abgleich im Hintergrund | erledigt | 2026-09-26 | `backgroundSync` in serve (local/http, eigene Goroutine je Eintrag), Schreiben an `entry_id` gebunden; Tests local/http/Fehler/0/rm+add/Beenden; make check grün |
| 3 — `whoami` | erledigt | 2026-09-26 | `Authenticate` (Anmeldung über alle Hubs), `Whoami` für MCP und CLI, Feld `unknown_hubs`; Tests über MCP; make check grün |
| 4 — `kephalaion node whoami` | erledigt | 2026-09-26 | Liste, Einzelansicht, `--hub`, `--json` (gleich der MCP-Antwort); make check grün |
| 5 — Doku | erledigt | 2026-09-26 | README, begriffe, konzept (Stand, Nebenläufigkeit, `unknown_hubs`, „Zustandslos“), vscode, k-playbook-local/k-playbook.md, fortschritt; make check grün |

---
## Review-Log (2026-09-26)

**Pfad:** k-playbook-local/tasks
**Intent:** inline
**Runden:** 2 (gemeinsam mit Task 009)

### Diskussion
- **4 (login ohne Replica):** Der Critic sah ein Problem: Ohne Replica ergibt ein korrektes Header-Paar `invalid`, und die Erweiterung würde einen Anmeldefehler zeigen. Er schlug einen eigenen Wert vor. Der Moderator bleibt bei den drei Werten aus dem Konzept. Den Grund erkennt man an `sync` („noch nie abgeglichen“); das steht jetzt ausdrücklich im Task und ist getestet.
- **5/N2 (Nebenläufigkeit):** Eine Sperre im Speicher reicht nicht, weil CLI und `serve` eigene Prozesse sind. Der Editor verweist auf die Sperre über Prozesse (`takeLock`) oder auf das Binden an die Identität des Eintrags. Runde 2 wies darauf hin, dass Alias, rowid und Pfad wiederkehren können; deshalb muss die Kennung eine sein, die nie wiederkehrt.
- **8/N1 (Log):** Der Moderator hat entschieden, nur Übergänge zu loggen. In Runde 2 kam dazu: der erste Fehler nach dem Start, und „geändert“ meint die Art des Fehlers, nicht wechselnde Teile der Meldung.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| 4 | WARNUNG | 008 | whoami, `login` | Ohne Replica ergibt ein korrektes Header-Paar `invalid` | Verhalten festlegen, Test |
| 5 | WARNUNG | 008 | Nebenläufigkeit | Eine Sperre im Speicher reicht bei eigenen Prozessen nicht; Fall rm + add mit gleichem Alias fehlt | Sperre über Prozesse oder Identität, Test |
| 8 | WARNUNG | 008 | Log | „Wenn einer scheitert“ ergibt womöglich eine Zeile je Runde | nur Übergänge loggen |
| 9 | FEHLEND | 008 | Stand je Hub, Schemafassung +1 | Hinweis für den Nutzer (Export/Import) fehlt | Doku-Hinweis |
| 10 | WARNUNG | 008/009 | Zusammenspiel | Anmeldung über alle Hubs womöglich doppelt gebaut | gemeinsame Funktion in `mcpnode` |
| N1 | Lücke | 008 | Log | Erster Fehler nach dem Start bleibt still; eine Meldung mit wechselnden Teilen ergibt eine Zeile je Runde | ersten Fehler loggen, Art des Fehlers vergleichen |
| N2 | Umsetzbarkeit | 008 | Nebenläufigkeit | Die Identität des Eintrags kann wiederkehren | Kennung, die nie wiederkehrt |
| N4 | Doku-Konsistenz | 008 | Etappe 5 | Feld für unbekannte Aliase fehlt in der Doku-Liste für `konzept.md` | Etappe 5 ergänzen |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| 4 | decide | Das Konzept legt drei Werte fest | `invalid`, der Grund steht in `sync`; Test |
| 5 | pass | blockiert die Korrektheit | behoben |
| 8 | decide | Log-Politik | nur Übergänge |
| 9 | skip | Die Regel „keine Migrationen“ ist bekannt, nicht blockierend | offen |
| 10 | pass | vermeidet doppelte Umsetzung | behoben |
| N1 | decide | enge Klarstellung | behoben |
| N2 | decide | enge Klarstellung | behoben |
| N4 | decide | Konsistenz | behoben |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| 4 | fixed | Moderator-Entscheidung umgesetzt |
| 5 | fixed | Sperre über Prozesse oder Kennung des Eintrags; Fall im Kontext und in den Tests |
| 8 | fixed | Moderator-Entscheidung umgesetzt |
| 10 | fixed | eigener Kontext-Punkt „Anmeldung über alle Hubs“ |

### Moderator-Entscheidungen
- **„Offen für Refine“ (Header zu unbekannten Aliasen):** werden gemeldet, nur mit dem Alias und in einem eigenen Feld. Das hilft bei falsch eingerichteten Clients und verrät nichts, was der Client nicht schon weiß. `docs/konzept.md` zieht nach (Etappe 5).
- 9 übersprungen: Das Schema neu anzulegen ist Projektregel und nicht blockierend.
- N1, N2 und N4 aus Runde 2 hat der Moderator direkt eingetragen, ohne eine weitere Editor-Runde: Es sind enge Klarstellungen.

### Intent-Alignment
Ja. Alle vier Punkte des Intents sind abgedeckt: Abgleich im Hintergrund mit Abstand, Fehler in Log und Tabelle, `whoami` nach dem Konzept ohne Geheimnisse, `node whoami` aus derselben Funktion (`--json` gleich der MCP-Antwort).

### Geänderte Dateien
- 008-abgleich-hintergrund-whoami.md: Nebenläufigkeit (Sperre über Prozesse oder Kennung, die nie wiederkehrt; rm + add) (5, N2), Log-Regel (8, N1), `login` ohne Replica und unbekannte Aliase (4, Offen für Refine), Anmeldung über alle Hubs (10), Tests in Etappe 2 und 3, Doku in Etappe 5 (N4)

### Offen (nicht gefixt)
- 9: Hinweis für den Nutzer zur Schemafassung +1 (Export vor dem Update); nicht blockierend.

## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** 2026-09-26  
**Zusammenfassung:** `serve` gleicht als Node jeden Hub-Eintrag beim Start und danach je `sync_interval` ab. Neu sind `config set|unset`, eine eigene Goroutine je Eintrag und ein Log nur bei Übergängen. Der Stand je Hub steht in der neuen Tabelle `hub_sync` und wird in `status` und `whoami` gezeigt. `whoami` folgt dem Konzept (alle Hubs, `login` ok/invalid/missing, `node`, `sync`, `unknown_hubs`). Die gemeinsame Funktion `mcpnode.Authenticate` ist die Grundlage für Task 009, und `kephalaion node whoami [<account>] [--hub] [--json]` kommt aus derselben Funktion `mcpnode.Whoami`. Nebenläufigkeit: Das Schreiben ist an eine nie wiederkehrende `entry_id` (ULID) des Hub-Eintrags gebunden, statt eine Sperre über Prozesse zu nehmen. Die Bindung steht in `hubs`, `hub_sync` und in `db_info` der Replica; die Replica wird per tmp-Datei und `os.Link` neu angelegt. Schemafassungen: Node 3 → 4, Replica 2 → 3. `make check` ist grün, `go test -race` über `cmd/kephalaion` und `internal/node/...` ebenfalls.

Die Baseline ist `bd13357`. Der Diff enthält auch die parallelen Commits `973e94a` (Task 010: Refine) und `3d14e27` (Konzept: Installation und Betrieb). Der Code wurde vom Nutzer mit `18da4d0 Zwischenstand` gesichert.

**Geänderte Dateien:**
```
 README.md                                          |  69 +++-
 cmd/kephalaion/bgsync.go                           | 195 +++++++++++
 cmd/kephalaion/bgsync_test.go                      | 364 +++++++++++++++++++++
 cmd/kephalaion/configcmd.go                        |  80 +++++
 cmd/kephalaion/configimport_test.go                |   5 +
 cmd/kephalaion/main.go                             |  12 +-
 cmd/kephalaion/mcp_test.go                         |  55 +++-
 cmd/kephalaion/nodewhoami_test.go                  | 106 ++++++
 cmd/kephalaion/nodewhoamicmd.go                    | 148 +++++++++
 cmd/kephalaion/roles.go                            |  62 +++-
 cmd/kephalaion/serve.go                            |  47 ++-
 cmd/kephalaion/synccmd.go                          |  14 +-
 cmd/kephalaion/synccmd_test.go                     |  75 +++++
 docs/begriffe.md                                   |  66 +++-
 docs/fortschritt.md                                |  26 +-
 docs/konzept.md                                    | 188 ++++++++++-
 docs/vscode.md                                     |  11 +-
 internal/node/mcpnode/login.go                     | 275 ++++++++++++++++
 internal/node/mcpnode/mcpnode.go                   | 176 +---------
 internal/node/mcpnode/mcpnode_test.go              | 183 +++++++++--
 internal/node/mcpnode/whoami.go                    | 188 +++++++++++
 internal/node/replica/accounts.go                  |  39 ++-
 internal/node/replica/concurrency_test.go          | 321 ++++++++++++++++++
 internal/node/replica/replica.go                   | 148 +++++++--
 internal/node/replica/sync.go                      | 231 +++++++++++--
 internal/node/replica/sync_test.go                 |   6 +-
 internal/node/store/hubs.go                        |  64 ++--
 internal/node/store/hubs_test.go                   |  32 +-
 internal/node/store/settings.go                    | 102 ++++++
 internal/node/store/store.go                       |  38 ++-
 internal/node/store/store_test.go                  |   2 +-
 internal/node/store/syncstatus.go                  |  83 +++++
 internal/node/store/syncstatus_test.go             | 147 +++++++++
 internal/sqlitedb/sqlitedb.go                      |  28 ++
 k-playbook-local/k-playbook.md                     |  26 +-
 .../tasks/008-abgleich-hintergrund-whoami.md       |  10 +
 k-playbook-local/tasks/010-arbeitsbranch-dev.md    |  99 +++++-
 37 files changed, 3357 insertions(+), 364 deletions(-)
```

**Code-Änderungen:** (die wichtigsten Hunks, gekürzt)

Replica prüft den Eigentümer in jeder schreibenden Transaktion (`internal/node/replica/replica.go`):
```go
+func (r *Replica) checkOwner(ctx context.Context, tx *sql.Tx) error {
+	info, err := sqlitedb.ReadInfo(ctx, tx)
+	if info[KeyEntryID] != r.entryID || info[KeyHubID] != r.hubID { return ErrChanged }
+	qStateUpsert = `… ON CONFLICT(collection) DO UPDATE SET revision = max(sync_state.revision, excluded.revision), …`
+	qDocUpsert = `INSERT … ON CONFLICT(id) DO UPDATE SET … WHERE excluded.revision >= documents.revision`
+func Create(ctx context.Context, path, hubID, entryID string) (*Replica, error) {
+	tmp := path + ".new-" + ulid.Make().String()   // bauen, schließen, os.Link(tmp, path), tmp entfernen
```

Bindung in node.db (`internal/node/store/hubs.go`, `syncstatus.go`):
```go
-	qHubID = `UPDATE hubs SET hub_id = ? WHERE name = ?`
+	qHubID = `UPDATE hubs SET hub_id = ? WHERE name = ? AND entry_id = ?`
+	qSyncOK = `INSERT INTO hub_sync (…) SELECT name, entry_id, ?, NULL, NULL, NULL FROM hubs WHERE name = ? AND entry_id = ?
+		ON CONFLICT(hub) DO UPDATE SET … error = NULL, error_kind = NULL, error_at = NULL`   // 0 Zeilen → ErrEntryGone
```

Hintergrund (`cmd/kephalaion/bgsync.go`, `serve.go`):
```go
+func (b *backgroundSync) run(ctx) { … d := b.readInterval(ctx); if d > 0 { b.round(ctx); wait = d } else { wait = syncIdle } … }
+func (b *backgroundSync) round(ctx) { … https/ssh: einmal loggen, übergehen; läuft schon: übergehen; sonst go syncOne }
+	if res.Err != nil { if !seen || !prev.failed || prev.kind != res.Kind { log "Abgleich %s gescheitert: %v" } }
+	bg := newBackgroundSync(nodes, cfg, hub, log); go bg.run(bgCtx)
+	stopBg(); <-bgDone   // vor dem Shutdown und dem Schließen der Stores
```

whoami (`internal/node/mcpnode/login.go`, `whoami.go`):
```go
+func (n *Node) Authenticate(ctx, header) (Logins, error)   // je Hub-Eintrag ok/invalid/missing, Unknown = Aliase ohne Eintrag
+func AccountLogins(ctx, nodes, account) (Logins, error)    // CLI: ok, wo lebende SYSTEM:A:-Zeilen stehen, sonst missing
+func Whoami(ctx, nodes, version, logins) (WhoamiOutput, string, error)   // eine Funktion für MCP und CLI
```

Die übrigen Änderungen:
- `config set|unset` in `configcmd.go`, dazu `store/settings.go`.
- `node whoami` in `nodewhoamicmd.go`.
- Die Tests: `bgsync_test.go`, `concurrency_test.go`, `syncstatus_test.go`, `nodewhoami_test.go`, dazu die Erweiterungen in `mcpnode_test.go`.
- Die Doku: README, `begriffe`, `konzept`, `vscode`, `fortschritt`, `k-playbook-local/k-playbook.md`.

**Restrisiken laut Ausführung:**
- `last_error` zeigt in `whoami` nur einen festen Satz je Fehlerart, weil die Meldung die Adresse nennen kann.
- Ein ungültiges `sync_interval` aus `config import` wird nicht abgewiesen. `serve` meldet es und nimmt 30 s.
- Bei Abstand 0 sieht `serve` erst nach 30 s nach, ob der Abgleich wieder an ist.
- `TestBackgroundSyncErrors` braucht etwa 9 s.
- Review-Punkt 9 ist weiter offen: Wie man die Datenbank neu anlegt, ist nicht beschrieben.

**Code-Review:** (nur auf Basis des Diffs von `cmd/` und `internal/`)

Urteil: Request Changes. Befund 1 und Vorschlag 1 sollten vor dem nächsten Release behoben werden, der Rest kann als Folgearbeit laufen.

| # | Datei | Befund | Schwere |
|---|---|---|---|
| 1 | `mcpnode/login.go` (`openReplica`), `whoami.go` (`syncInfo`) | Nur `sqlitedb.ErrNotFound` gilt als „keine Replica“. Jeder andere Fehler bricht `Authenticate` und `Whoami` insgesamt ab, etwa bei einer alten Replica mit Schemafassung 2, einer ohne `entry_id` oder einer beschädigten Datei. Weil `whoami` jetzt über alle Hubs läuft, legt eine einzige solche Replica das Werkzeug für alle Hubs lahm. Bei `sync_interval 0` oder einem Fehler beim Verbinden wird die Datei nie verworfen. Vorschlag: den Fehler je Hub abbilden statt global zu scheitern. | 🟠 Hoch |

| # | Datei | Vorschlag | Kategorie |
|---|---|---|---|
| 1 | `replica/replica.go` (`Create`) | Ein schmales Race zwischen `os.Link` und `Open`: Ersetzt ein anderer Prozess die Datei, trägt die Replica eine fremde `entry_id`, und `checkOwner` prüft nur gegen `r.entryID`. Nach `Open` sollte `EntryID() == entryID` geprüft werden, sonst `ErrChanged`. | Korrektheit |
| 2 | `serve.go` (`<-bgDone`) | Das Warten auf den Abgleich beim Beenden hat keine Frist. Es sollte mit `shutdownGrace` begrenzt werden. | Shutdown |
| 3 | `replica/replica.go` (`reset`) | `reset` prüft nur `entry_id`, nicht die alte `hub_id`. Zwei parallele Resets verwerfen doppelt; das kostet Arbeit, ergibt aber keine falschen Daten. | Korrektheit |
| 4 | `mcpnode/whoami.go` (`DescribeSync`) | `*s.Revision` wird ohne nil-Prüfung dereferenziert. | Robustheit |
| 5 | `bgsync.go` (`run`) | Ein neues `sync_interval` wirkt erst nach Ablauf des laufenden Timers. Das sollte in der Hilfe stehen. | UX |
| 6 | `bgsync.go` | `outcome` und `skipped` werden für entfernte Einträge nicht geräumt. Wechselt ein Eintrag https → http → https, fehlt die zweite Logzeile. | Wartbarkeit |
| 7 | `replica/replica.go` (`Create`) | Nach einem Absturz bleiben verwaiste `.db.new-<ULID>`-Dateien liegen. Außerdem setzt `os.Link` Hardlinks im Dateisystem voraus. | Robustheit |
| 8 | `bgsync.go` (`syncOne`) | Je Runde entsteht ein neuer `connector`. Beim http-Transport bleiben dadurch womöglich Idle-Verbindungen offen. | Performance |
| 9 | `mcpnode/whoami.go`, `login.go` | Die Hubs werden zweimal gelesen, und die Replica wird je Anfrage bis zu zweimal geöffnet. | Wartbarkeit |
| 10 | `replica/sync.go` (`openForSync`) | Nach `Remove` sehen lesende Pfade eines alten `*sql.DB` womöglich die neue Datei. Nur die schreibenden Pfade sind geschützt; das sollte im Kommentar stehen. | Korrektheit |
| 11 | `bgsync_test.go` | Die Log-Zustandsmaschine wird nur langsam über `serve` getestet. Es fehlen Tests zu Befund 1, zu Vorschlag 1 und zu einem Hub, der den ctx nicht beachtet. | Tests |
| 12 | `bgsync.go` | Die geloggte „Revision“ ist das Minimum über die Collections. Das sollte im Kommentar bzw. in der Hilfe so benannt sein. | Wartbarkeit |

Positiv:
- Die Bindung an `entry_id` ist durchgängig und atomar (`INSERT … SELECT … WHERE entry_id`).
- Die Upserts schreiben nie zurück, und eine Lücke im Stand wird erkannt.
- `whoami` enthält keine Geheimnisse; die Tests prüfen das mit `noSecrets`.
- Das Log meldet nur Übergänge.
- `--json` ist per `DeepEqual` gegen die MCP-Antwort geprüft.
- Der `connector` schließt nur, was er selbst geöffnet hat.

**Intent-Alignment:** Teilweise. Der Kern ist erfüllt:
- Der Abgleich läuft im Hintergrund je `sync_interval`.
- Fehler stehen im Log und in `hub_sync`, und `status` und `whoami` zeigen denselben Stand.
- `whoami` enthält keine Geheimnisse.
- `node whoami` kommt aus derselben Funktion wie das Werkzeug.

Offen ist der Review-Befund „Hoch“: Eine einzelne unlesbare Replica (alte Schemafassung, beschädigt) lässt `whoami` und `node whoami` insgesamt scheitern statt nur für diesen Hub. Der Critic nennt außerdem, dass Einträge mit `https`/`ssh` nur übergangen werden. Das verlangt die Task so: Diese Transporte sind noch nicht gebaut.
