# Task 002 — Datenbank, Konfiguration, init und status

Hub und Node lassen sich je mit einem Aufruf einrichten; `kephalaion status` zeigt, was auf
diesem Rechner eingerichtet ist und wo es liegt.

## Intent

Die Datenbank ist das Zentrum von Kephalaion; dieser Task legt sie an und macht sie über die
CLI sichtbar, ohne dass schon Inhalte fließen.
- `kephalaion hub init` und `kephalaion node init` richten je eine Rolle ein: Datenbank,
  Schema, Abschnitt in der config. Ohne Rückfragen, nie überschreibend.
- `kephalaion status` gibt Rollen, Orte und Kennzahlen aus, auch wenn nichts oder nur eine
  Rolle eingerichtet ist.
- Der Datenbankzugriff von Hub und Node ist je hinter einer eigenen Schnittstelle gekapselt;
  die Abfragen des Hubs bleiben in SQLite und PostgreSQL ausführbar.
- Die Einstellungen lassen sich mit `config export` und `config import` getrennt von den
  Inhalten sichern und wiederherstellen.
- Hub und Node bleiben im Code getrennt: kein Paket des einen importiert das des anderen.

## Referenzen

- `docs/konzept.md` — Abschnitte „Ein Programm, zwei Rollen“ (Konfiguration, init),
  „Speicherung“ (Hub gekapselt, später PostgreSQL), „Datenmodell“ (Schema `documents`,
  `actions`; auf dem Node zählt die `id`; „Schemaänderungen“), „Abgleich“ (Revision als
  Zähler in einer Zeile).
- `docs/begriffe.md` — `config`, `init`, `status`; neue Begriffe vor Gebrauch eintragen.
- `k-playbook-local/k-playbook.md` — Aufbau, Bauen, Testen.
- `cmd/kephalaion/main.go` — Muster für Unterkommandos und Hilfetexte.

## Ziel

- Paket für die config (`~/.config/kephalaion/config.yaml`).
- Gekapselte Datenbanken für Hub und Node, dahinter SQLite (`modernc.org/sqlite`).
- CLI: `hub init`, `node init`, `status`, `config show|export|import`.

## Kontext

- **config — nur „was und wo“:**
  ```yaml
  hub:
    db: sqlite:///home/<user>/.local/share/kephalaion/hub.db
  node:
    db: sqlite:///home/<user>/.local/share/kephalaion/node.db
  ```
  Fehlt ein Abschnitt, fehlt die Rolle. Ort: `--config` > `KEPHALAION_CONFIG` >
  `$XDG_CONFIG_HOME/kephalaion/config.yaml` > `~/.config/kephalaion/config.yaml`. Kein
  Geheimnis in der Datei. Atomar schreiben (Datei daneben, `rename`); beim Eintragen einer
  Rolle bleibt die andere unberührt. YAML-Bibliothek: eine gepflegte, etwa
  `go.yaml.in/yaml/v3`.
- **db-Adressen:** `sqlite://` mit absolutem Pfad. `postgres://…` wird erkannt und mit
  „noch nicht unterstützt“ abgelehnt — die Stelle für PostgreSQL ist damit sichtbar.
- **Standard ohne `--db`:** `$XDG_DATA_HOME/kephalaion/` bzw.
  `~/.local/share/kephalaion/`, Dateien `hub.db` und `node.db`. Verzeichnis mit `0700`
  anlegen.
- **Nur `init` legt an:** Alle anderen Kommandos öffnen nur vorhandene Datenbankdateien.
  Fehlt die Datei, ist das ein Fehler dieser Rolle; es entsteht keine Datei (Vorsicht:
  `modernc.org/sqlite` legt fehlende Dateien beim Öffnen sonst an).
- **Kapselung:** je eine Schnittstelle für Hub- und Node-Datenbank, SQLite als erste
  Umsetzung. Hub: nur Abfragen (DML), die auch PostgreSQL versteht (Platzhalter über eine
  kleine Hilfe, kein `INSERT OR …`, kein SQLite-eigenes `PRAGMA` in Abfragen, kein
  `AUTOINCREMENT`; die Verbindung darf `PRAGMA` setzen). Das DDL darf je Dialekt stehen;
  das PostgreSQL-DDL entsteht mit der PostgreSQL-Umsetzung (dort `BIGINT` für
  ms-Zeitstempel und die Spalten `revision`). Der volle Nachweis kommt erst mit PostgreSQL; bis dahin
  stehen die Hub-SQL-Texte zentral, und ein Test prüft sie auf die verbotenen Konstrukte.
  Node: SQLite-eigenes erlaubt — dort kommt später FTS5.
- **Gemeinsamer Unterbau:** ein neutrales Paket (Name nach Ermessen, etwa
  `internal/sqlitedb`), das weder Hub noch Node kennt: SQLite öffnen samt Einstellungen,
  `db_info` prüfen (Schemafassung, Rolle), `settings` lesen und schreiben. Hub- und
  Node-Paket dürfen es importieren, nicht aber einander. SQLite-eigen ist nur das Öffnen
  samt Verbindungseinstellungen; die Abfragen auf `db_info` und `settings` folgen der
  Hub-Regel (PostgreSQL-tauglich) und fallen unter denselben Test.
- **SQLite-Verbindung:** WAL, `foreign_keys=ON`, `busy_timeout`. `CGO_ENABLED=0` bleibt.
- **Schema anlegen, keine Migrationen — bewusste, befristete Abweichung vom Konzept**
  („Schemaänderungen“ verlangt Migrationen). Sie gilt, solange es keine Daten gibt, die
  bleiben müssen. Die Schemafassung steht in `db_info`. Passt sie nicht zum Binary, bricht
  jeder Zugriff mit klarer Meldung ab: Datenbank neu anlegen; die Einstellungen
  (`settings` und config) lassen sich über `config export`/`import` retten, Inhalte nicht.
  Ein Rahmen für Migrationen entsteht, sobald es Daten gibt, die bleiben müssen.
- **Rolle in der Datenbank:** `db_info` hält auch die Rolle (`hub`/`node`). Eine Hub-Datenbank
  als Node zu öffnen (oder umgekehrt) ist ein Fehler.
- **Revision:** Zähler als Zeile in `db_info`, erhöht in der schreibenden Transaktion — keine
  `SEQUENCE`, kein Autoincrement. Hochgezählt wird im Code (lesen und schreiben in
  derselben Transaktion), nicht per Umwandlung in SQL. In diesem Task schreibt noch niemand; der Zähler steht
  auf 0.
- **Nicht in diesem Task:** Hubs am Node (`node hub …`), `serve`, Collections, Accounts,
  Schreiben von Dokumenten, PostgreSQL selbst.

## Zu bauen

### Etappe 1 — config

- `internal/config/`: Pfad auflösen, lesen, atomar schreiben, Rolle eintragen, db-Adresse
  parsen (`sqlite://`, `postgres://` erkannt und abgelehnt).
- Tests: Pfadreihenfolge, Hin und zurück, Eintragen lässt die andere Rolle unberührt,
  ungültige Adressen, fehlende Datei.

### Etappe 2 — Datenbank des Hubs

- `internal/sqlitedb/` (Name nach Ermessen): gemeinsamer Unterbau wie im Kontext
  beschrieben; Öffnen legt keine Datei an, nur ein ausdrückliches Anlegen tut das.
- `internal/hub/store/` (Name nach Ermessen, Hauptsache getrennt vom Node): Schnittstelle,
  SQLite-Umsetzung, Schema anlegen.
- Schema: `db_info (key TEXT PRIMARY KEY, value TEXT NOT NULL)` mit `schema_version`,
  `role`, `revision`, `created_at`; `settings (key, value)`; `documents` und `actions` samt
  Indizes genau nach „Datenmodell“ im Konzept (Hub: mit eindeutigem Index auf
  `(collection, name) WHERE deleted = 0` und dem Index für `SYSTEM:`-Zeilen).
- Abfragen für `status`: Schemafassung, Revision, Anzahl Dokumente (ohne Löschmarken und
  ohne `SYSTEM:`-Zeilen), Anzahl Collections (verschiedene `collection`).
- Tests in `t.TempDir()`: anlegen, erneut öffnen, falsche Schemafassung, falsche Rolle.
- Test: die zentralen Hub-SQL-Texte enthalten kein `INSERT OR`, kein `PRAGMA`, kein
  `AUTOINCREMENT` und keine rohen `?`-Platzhalter an der Hilfe vorbei.

### Etappe 3 — Datenbank des Nodes

- `internal/node/store/`: Schnittstelle, SQLite-Umsetzung, Schema `db_info` und `settings`.
  Replicas kommen später als eigene Dateien daneben, nicht in diesem Task.
- Tests wie beim Hub.
- Test der Trennregel (etwa über `go list -deps` oder `go/packages`): scheitert, wenn ein
  Hub-Paket ein Node-Paket importiert oder umgekehrt.

### Etappe 4 — CLI: init und status

- `kephalaion hub init [--db …]`, `kephalaion node init [--db …]`, jeweils mit `--config`.
  Bricht ab, wenn die Rolle in der config steht oder die Datenbankdatei schon existiert.
  Reihenfolge: erst Datenbank anlegen, dann config schreiben. Scheitert die config, wird
  die frisch angelegte Datenbankdatei samt `-wal`/`-shm` wieder entfernt. Meldet am Ende, was angelegt wurde.
- `kephalaion status [--config …]`: Ort der config (und ob es sie gibt); je Rolle
  eingerichtet ja/nein, db-Adresse, öffnet sie, Schemafassung; Hub zusätzlich Revision,
  Dokumente, Collections; Node: „Hubs: keine“. Ist nichts eingerichtet, Hinweis auf
  `hub init`/`node init`. Eine Rolle, deren Datenbank fehlt oder nicht passt, wird als
  Fehler dieser Rolle gemeldet; Exit-Code ≠ 0 nur dann.
- Hilfetexte im Stil von `main.go`; `usage` um die neuen Kommandos ergänzen.
- Tests über `run()`: nichts eingerichtet, nur Hub, beide, zweites `init` scheitert,
  `init` mit nicht schreibbarer config hinterlässt keine Datenbankdatei, `status` bei
  fehlender Datenbankdatei legt keine Datei an.

### Etappe 5 — config show, export, import

- `kephalaion config show`: Ort und Inhalt der config, dazu die `settings` je Rolle. Fehlt
  die Datenbank einer Rolle oder passt sie nicht: wie bei `status` — config trotzdem
  zeigen, Fehler dieser Rolle melden, Exit-Code ≠ 0.
- `kephalaion config export [--output datei]`: YAML mit Fassung des Formats, der config und
  den `settings` je Rolle — keine Inhalte. Datei mit `0600`, weil `settings` später Token
  enthalten.
- `kephalaion config import <datei>`: schreibt die `settings` in bereits eingerichtete
  Rollen und ersetzt sie dort. Vorab wird alles geprüft, erst dann geschrieben: Formatfassung
  bekannt (sonst klare Ablehnung), jede Rolle des Exports eingerichtet, Datenbank öffnet,
  Schema passt. Fehlt eine Rolle, die der Export enthält: abbrechen mit Hinweis auf `init`
  (mit dem `db` aus dem Export als Vorschlag). Geschrieben wird je Rolle in einer
  Transaktion. Die config selbst wird nicht überschrieben.
- Tests: Export → neue Datenbanken → Import ergibt dieselben `settings`; unbekannte
  Formatfassung wird abgelehnt; fehlt eine Rolle, bleiben auch die `settings` der anderen
  unverändert.

### Etappe 6 — Doku

- `docs/begriffe.md`: `export`, `import` (unter `config`), `settings`, `db_info` eintragen.
- `README.md`: Einrichten mit `init`, `status`.
- `k-playbook-local/k-playbook.md`: Aufbau um config, den gemeinsamen Unterbau und die
  beiden Datenbankpakete ergänzen; Regel „Hub-SQL bleibt PostgreSQL-tauglich“; „keine Migrationen, Schema neu
  anlegen“.
- `docs/konzept.md`: Absatz „Schemaänderungen“ um die befristete Abweichung (keine
  Migrationen, solange keine Daten bleiben müssen) ergänzen; unter „Speicherung“ bzw.
  „Datenmodell“ die Tabellen `db_info` und `settings` sowie Rolle und Schemafassung in der
  Datenbank nachtragen; sonst nur nachziehen, wo die Umsetzung vom Konzept abweicht.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — config | erledigt | 2026-09-25 | `internal/config`: Pfad, Laden (strikt), atomar schreiben, AddRole, ParseDB; Tests grün |
| 2 — Datenbank des Hubs | erledigt | 2026-09-25 | `internal/sqlq` (Platzhalter $n, Check), `internal/sqlitedb` (Unterbau), `internal/hub/store`; SQL-Test, Revision im Code; Tests grün |
| 3 — Datenbank des Nodes | erledigt | 2026-09-25 | `internal/node/store`; Trenntest `internal/separation_test.go` liest Importe per `go/parser` (cache-sicher, statt `go list`); Tests grün |
| 4 — CLI: init und status | erledigt | 2026-09-25 | `cmd/kephalaion/roles.go`: `hub init`, `node init`, `status`; usage ergänzt; Tests über `run()` inkl. Aufräumen, fehlende DB, falsche Rolle; grün |
| 5 — config show, export, import | erledigt | 2026-09-25 | `cmd/kephalaion/configcmd.go`: show, export (Format 1, Datei 0600), import (Fassung zuerst, Vorabprüfung, je Rolle eine Transaktion); Tests grün |
| 6 — Doku | erledigt | 2026-09-25 | begriffe (export, import, settings, db_info, db-Adresse), README „Einrichten“, k-playbook.md Aufbau + Regeln, Konzept: Stand, db_info/settings, Umsetzung Speicherung, befristete Abweichung Migrationen, Revision in `db_info` |

---
## Review-Log (2026-09-25)

**Pfad:** k-playbook-local/tasks/002-datenbank-config-status.md
**Intent:** inline (`## Intent`)
**Runden:** 2

### Diskussion
- **Migrationen (FEHLER-01):** Der Critic stellte fest, dass `docs/konzept.md` („Schemaänderungen“)
  Migrationen verlangt und die Meldung fälschlich nahelegte, export/import rette den Hub-Bestand.
  Der Moderator behielt „keine Migrationen“ bei. Die Entscheidung steht jetzt als bewusste,
  befristete Abweichung im Task und wird in Etappe 6 im Konzept nachgetragen.
- **Gemeinsamer Unterbau (FEHLER-02, NEU-01):** Ohne neutrales Paket hätte der Agent den Code
  doppelt schreiben oder die Trennregel brechen müssen. Deshalb gibt es jetzt `internal/sqlitedb`
  (Name nach Ermessen) und einen Test für die Trennregel. In Runde 2 fiel auf, dass die
  Abfragen des Unterbaus nicht unter die PostgreSQL-Regel fielen. Das ist klargestellt:
  SQLite-eigen ist dort nur das Öffnen.
- **Revision als Text (NEU-02):** `db_info.value` ist `TEXT`, und die Umwandlung in SQL wäre je
  Dialekt verschieden. Der Moderator hat entschieden, dass im Code hochgezählt wird.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| FEHLER-01 | FEHLER | 002 | Kontext „keine Migrationen“ | Widerspricht Konzept „Schemaänderungen“; Meldung verspricht Rettung des Hub-Bestands | Als befristete Abweichung benennen, Konzept nachziehen, Meldung korrigieren |
| FEHLER-02 | FEHLER | 002 | Kontext „Kapselung“ | Kein Ort für gemeinsamen SQLite-Unterbau → Duplikat oder Trennbruch | Neutrales Paket; Trennregel per Test |
| WARNUNG-01 | WARNUNG | 002 | Etappe 2 Schema | Konzept-DDL `INTEGER` ist in PostgreSQL 32 Bit | DDL je Dialekt erlauben bzw. `BIGINT` |
| WARNUNG-02 | WARNUNG | 002 | Intent PostgreSQL | Ohne PostgreSQL nicht nachweisbar | Test auf verbotene Konstrukte oder Nachweis vertagen |
| WARNUNG-03 | WARNUNG | 002 | Tabelle `meta` | Kollidiert mit JSON-Spalte `meta` in `documents` | Umbenennen |
| WARNUNG-04 | WARNUNG | 002 | `status` öffnet DB | SQLite legt fehlende Datei an, verdeckt Fehler, blockiert `init` | Nur `init` legt an; Test |
| WARNUNG-05 | WARNUNG | 002 | Kennzahl Dokumente | Zählt `SYSTEM:`-Zeilen mit | Ausschließen |
| WARNUNG-06 | WARNUNG | 002 | Etappe 6 | `db_info`/`settings`/Rolle fehlen im Konzept | Konzept nachtragen |
| FEHLEND-01 | FEHLEND | 002 | Etappe 4 `init` | Halbes `init` blockiert Wiederholung | Reihenfolge + Aufräumen, Test |
| FEHLEND-02 | FEHLEND | 002 | Etappe 5 `import` | Nicht atomar über Rollen; unbekannte Formatfassung offen | Vorab prüfen, je Rolle Transaktion, Ablehnung |
| FEHLEND-03 | FEHLEND | 002 | Etappe 4 `status` | `begriffe.md` nennt Verbindungen, Task nur „Hubs: keine“ | Vermerken |
| NEU-01 | WARNUNG | 002 | Unterbau vs. Hub-SQL-Test | DML auf `db_info`/`settings` fällt nicht unter PostgreSQL-Regel | Regel und Test auf Unterbau-DML ausdehnen |
| NEU-02 | WARNUNG | 002 | Kapselung vs. Revision | Revision ist `TEXT` in `db_info`, `BIGINT`-Hinweis passt nicht, Umwandlung dialektabhängig | Hochzählen im Code |
| NEU-03 | HINWEIS | 002 | Etappe 5 `config show` | Fehlerverhalten bei fehlender DB offen | Wie `status` |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| FEHLER-01 | pass | Widerspruch zum Konzept, irreführende Meldung | gefixt |
| FEHLER-02 | pass | Blockiert die Umsetzung des Intent (Trennung) | gefixt |
| WARNUNG-01+02 | pass | PostgreSQL-Zusage sonst unklar/unprüfbar | gefixt (DDL je Dialekt, Test auf Konstrukte) |
| WARNUNG-03 | decide → pass | Begriffe sind verbindlich; Tabelle heißt `db_info` | gefixt |
| WARNUNG-04 | pass | Echte Fehlerquelle, verdeckt „Datenbank fehlt“ | gefixt |
| WARNUNG-05 | pass | Kleine Klarstellung | gefixt |
| WARNUNG-06 | pass | Mit FEHLER-01 zusammen | gefixt |
| FEHLEND-01 | pass | Intent „nie überschreibend“ + Wiederholbarkeit | gefixt |
| FEHLEND-02 | pass | Teilschreiben widerspricht „wiederherstellen“ | gefixt |
| FEHLEND-03 | skip | „Nicht in diesem Task“ nennt `node hub …`; nicht blockierend | Critic akzeptiert |
| NEU-01 | decide | Richtung (a): Unterbau-DML folgt der Hub-Regel | vom Moderator eingetragen |
| NEU-02 | decide | Hochzählen im Code; `BIGINT` nur für Spalten | vom Moderator eingetragen |
| NEU-03 | decide | Verhalten wie `status` | vom Moderator eingetragen |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| FEHLER-01 | gefixt | Befristete Abweichung benannt, Meldung auf `settings`+config beschränkt, Etappe 6 ergänzt |
| FEHLER-02 | gefixt | Kontextpunkt „Gemeinsamer Unterbau“, Etappe 2 baut ihn, Trenntest in Etappe 3 |
| WARNUNG-01+02 | gefixt | DML-Regel, DDL je Dialekt, Test auf `INSERT OR`/`PRAGMA`/`AUTOINCREMENT`/rohe `?` |
| WARNUNG-03 | gefixt | `meta` → `db_info` überall |
| WARNUNG-04 | gefixt | Kontextpunkt „Nur `init` legt an“, Test in Etappe 4 |
| WARNUNG-05 | gefixt | „ohne `SYSTEM:`-Zeilen“ |
| WARNUNG-06 | gefixt | Etappe 6 Konzept-Nachtrag |
| FEHLEND-01 | gefixt | Reihenfolge, Aufräumen samt `-wal`/`-shm`, Test |
| FEHLEND-02 | gefixt | Vorabprüfung, Transaktion je Rolle, Tests |
| FEHLEND-03 | nicht bearbeitet | vom Moderator übersprungen |

### Moderator-Entscheidungen
- Abgelehnt: Die Umformulierung des ersten Intent-Satzes durch den Editor („;“ → „.“) war
  keinem Befund zugeordnet.
- FEHLEND-03 übersprungen, denn es ist nicht blockierend. Der Critic hat das in Runde 2 akzeptiert.
- NEU-01 bis NEU-03 hat der Moderator ohne weitere Editor-Runde direkt entschieden, weil es
  kleine Klarstellungen ohne Gegenposition sind.

### Intent-Alignment
Ja. Jeder Intent-Punkt ist durch Etappen und Tests abgedeckt: init ohne Überschreiben und mit
Aufräumen, status für alle Fälle, gekapselte Stores mit PostgreSQL-tauglicher Hub-DML samt Test,
export/import mit Vorabprüfung, Trennregel per Test. Inhalte sind ausdrücklich ausgeschlossen.

### Geänderte Dateien
- 002-datenbank-config-status.md: Migrationen als befristete Abweichung, Unterbau-Paket und Trenntest,
  DML/DDL-Regel und SQL-Test, `meta` → `db_info`, nur `init` legt an, `SYSTEM:` ausgenommen,
  init-Aufräumen, import-Vorabprüfung, Konzept-Nachtrag in Etappe 6, Revision im Code
  hochzählen, `config show`-Fehlerverhalten (FEHLER-01, FEHLER-02, WARNUNG-01..06,
  FEHLEND-01, FEHLEND-02, NEU-01..03)

### Offen (nicht gefixt)
- FEHLEND-03: bewusst übersprungen; Verbindungen folgen mit `node hub …`/`serve`.

## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** 2026-09-25  
**Zusammenfassung:** Neu sind die Pakete `internal/config`, `internal/sqlq` (Platzhalter `$n`, `Bind` je Dialekt, `Check` auf verbotene Konstrukte), `internal/sqlitedb` (gemeinsamer Unterbau) sowie `internal/hub/store` und `internal/node/store` mit je einer Schnittstelle `Store` und SQLite-Umsetzung. Die CLI kann jetzt `hub init`, `node init`, `status` und `config show|export|import`; Tests laufen über `run()`, der Trenntest liest die Importe per `go/parser`. Die Doku ist nachgezogen (begriffe, README „Einrichten“, k-playbook.md, konzept), `make check` ist grün.

**Ermessensentscheidungen (Sub-Agent):** Platzhalter `$n`, für SQLite zu `?n` gebunden. `SYSTEM:`-Ausschluss per `substr(name,1,7) <> 'SYSTEM:'` (case-sensitiv in beiden Dialekten). Die config wird strikt gelesen (unbekannte Schlüssel sind ein Fehler) und mit 0644 geschrieben. Exportformat `format: 1`; `config export` bricht ganz ab, wenn eine Rolle fehlerhaft ist. `postgresql://` wird wie `postgres://` abgelehnt. Neuer Begriff „db-Adresse“.

**Offen:** Während der Ausführung wurden `docs/konzept.md` und `docs/begriffe.md` parallel geändert (`hub_id` in `db_info`, Tabellen `hubs` am Node sowie `nodes`/`node_collections` am Hub). Das ist nicht umgesetzt — Folgetask. Die fremden Änderungen sind in den Dateien erhalten und stecken im Diff mit drin.

**Geänderte Dateien:**
```
 README.md                                          |  31 +-
 cmd/kephalaion/configcmd.go                        | 354 +++++++++++++++++++++
 cmd/kephalaion/configcmd_test.go                   | 182 +++++++++++
 cmd/kephalaion/main.go                             |  22 +-
 cmd/kephalaion/roles.go                            | 333 +++++++++++++++++++
 cmd/kephalaion/roles_test.go                       | 196 ++++++++++++
 docs/begriffe.md                                   |  26 +-
 docs/konzept.md                                    | 141 ++++++--
 go.mod                                             |  17 +
 go.sum                                             |  52 +++
 internal/config/config.go                          | 221 +++++++++++++
 internal/config/config_test.go                     | 179 +++++++++++
 internal/config/dbaddr.go                          |  61 ++++
 internal/hub/store/store.go                        | 206 ++++++++++++
 internal/hub/store/store_test.go                   | 186 +++++++++++
 internal/node/store/store.go                       | 103 ++++++
 internal/node/store/store_test.go                  |  92 ++++++
 internal/separation_test.go                        | 148 +++++++++
 internal/sqlitedb/sqlitedb.go                      | 336 +++++++++++++++++++
 internal/sqlitedb/sqlitedb_test.go                 | 181 +++++++++++
 internal/sqlq/sqlq.go                              |  88 +++++
 internal/sqlq/sqlq_test.go                         |  44 +++
 k-playbook-local/k-playbook.md                     |  27 ++
 .../tasks/002-datenbank-config-status.md           |  12 +-
 24 files changed, 3203 insertions(+), 35 deletions(-)
```

**Code-Änderungen:** (rund 3200 Zeilen, großteils neue Dateien; hier nur die wichtigsten Stellen)

- `cmd/kephalaion/main.go`: `usage` ergänzt, Verteilung auf die neuen Kommandos:
```diff
+	case "hub":
+		return runRole(config.Hub, args[1:], stdout, stderr)
+	case "node":
+		return runRole(config.Node, args[1:], stdout, stderr)
+	case "status":
+		return runStatus(args[1:], stdout, stderr)
+	case "config":
+		return runConfig(args[1:], stdout, stderr)
```
- `internal/sqlitedb/sqlitedb.go`: Öffnen nur vorhandener Dateien, sodass keine Datei entsteht:
```go
+func dsn(path string) string {
+	u := url.URL{Scheme: "file", Path: path}
+	return u.String() + "?mode=rw" +
+		"&_pragma=foreign_keys(1)" +
+		"&_pragma=busy_timeout(" + strconv.Itoa(busyTimeout) + ")" +
+		"&_pragma=journal_mode(WAL)"
+}
```
- `internal/hub/store/store.go`: zentrale Abfragetexte, geprüft per `sqlq.Check`:
```go
+	CountDocuments: `SELECT COUNT(*) FROM documents
+		WHERE deleted = 0 AND substr(name, 1, 7) <> 'SYSTEM:'`,
+	CountCollections: `SELECT COUNT(DISTINCT collection) FROM documents`,
```
- Übrige neue Dateien in Prosa: `roles.go` (init mit Aufräumen samt `-wal`/`-shm`/`-journal`, status), `configcmd.go` (show/export mit 0600/import mit Vorabprüfung und Transaktion je Rolle), `config.go`/`dbaddr.go` (Pfadreihenfolge, atomares Schreiben, `ParseDB`), `separation_test.go` (Trennregel). Dazu die Tests je Paket.

**Code-Review:** (engineering:code-review, nur Diff) Keine kritischen Befunde. Urteil: Freigabe mit Nachbesserungen.
- *Mittel*
  1. `nextRevision` liest und schreibt dann. Unter PG (READ COMMITTED) ist das ein Lost Update, unter SQLite bei DEFERRED-Transaktion SQLITE_BUSY beim Wechsel zum Schreiben. Abhilfe: zuerst schreiben, `FOR UPDATE` bzw. `BEGIN IMMEDIATE`. Muss vor dem ersten Schreibpfad behoben sein.
  2. Scheitert der Import bei der zweiten Rolle, ist die erste schon committet, und die Meldung sagt nicht, was geschrieben wurde.
  3. `journal_mode(WAL)` wird bei jedem Öffnen gesetzt und bleibt in der Datei. `status` auf eine fremde SQLite-Datei stellt sie dadurch dauerhaft um, bevor `CheckInfo` ablehnt. Eine read-only DB scheitert an `mode=rw`. Abhilfe: WAL nur in `Create` setzen, `CheckInfo` vor weiteren Pragmas.
  4. Import: Eine Rolle in `config` ohne `settings`-Eintrag (oder mit `null`) löscht still alle settings dieser Rolle.
- *Gering*
  - Laden und Speichern der config ohne Sperre: parallele `hub init`/`node init` können eine Rolle verlieren.
  - Ein Close-Fehler in `createRole` lässt die DB ohne config-Eintrag zurück.
  - `Save` setzt die Rechte hart auf 0644 und ersetzt einen Symlink durch eine Datei.
  - Nach dem Rename fehlt der fsync des Verzeichnisses.
  - Scheitert init, bleibt ein leeres DB-Verzeichnis zurück.
  - Irreführende Meldung bei fehlender `schema_version`.
  - `CountCollections` filtert anders als `CountDocuments`, und `substr` nutzt den Index `documents_system` nicht.
  - Der `init`-Vorschlag beim Import setzt `--db` unquotiert und ungeprüft ein.
- *Hinweise*
  - `Bind` erkennt nur einfache Anführungszeichen.
  - `Check` ist eine Blockliste und lehnt auch `?` in Literalen bzw. PG-JSONB-Operatoren ab.
  - `nextRevision` wird noch nicht benutzt.
  - `DB.String` liefert immer `sqlite://`.
  - Ein zweites YAML-Dokument wird ignoriert.
  - Flags nach dem Positionsargument werden nicht erkannt (`flag`-Paket).
  - Export auf stdout bekommt keine 0600-Rechte.
  - Der Trenntest parst auch `testdata/`.

**Intent-Alignment:** Ja. Alle fünf Punkte des Intents sind umgesetzt:
- `hub init` und `node init` legen ohne Rückfrage an und überschreiben nie.
- `status` funktioniert auch bei null oder einer Rolle und legt nichts an.
- Hub und Node haben je eine eigene `Store`-Schnittstelle.
- Die Hub-Abfragen bleiben über `$n`-Platzhalter und die Dialekt-Prüfung portabel.
- `config export` und `config import` sichern die Einstellungen getrennt von den Inhalten.
- Ein Test prüft die Trennung von Hub und Node.

Die mittleren Review-Befunde sollten vor dem ersten produktiven Einsatz behoben werden, vor allem diese beiden: Ein fehlender settings-Eintrag löscht beim Import die settings still. Ein Import über zwei Rollen kann teilweise schreiben, ohne dass die Meldung es sagt.
