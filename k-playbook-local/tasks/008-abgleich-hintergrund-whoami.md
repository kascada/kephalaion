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
