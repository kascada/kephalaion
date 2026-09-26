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
  `hub_id`-Wechsel). Mindestens: kein falscher Stand, kein Absturz, die nächste Runde stimmt.
  Wenn nötig eine Sperre je Replica.
- **Log:** keine Zeile für einen Abgleich ohne Änderung; eine Zeile, wenn Dokumente kamen
  (Hub, Anzahl, Revision), wenn einer scheitert und wenn es danach wieder geht. Nie ein Token.
- **Stand je Hub:** eigene Tabelle in der Node-DB (Hub, letzter Erfolg, letzter Fehler mit
  Zeit), geschrieben vom Hintergrund und von `node sync`. Abgeleitet: nicht im Export;
  `node hub rm` und `config import` räumen mit ab. Node-Schemafassung +1, ohne Migration.
- **whoami:** `version`; je **Hub-Eintrag des Nodes** `hub`, `login` (`ok`/`invalid`/
  `missing`), `node`, `sync` (letzter Erfolg, Revision der Replica, letzter Fehler mit Zeit);
  bei `ok` zusätzlich `account`, `user`, `collections` (Collection, Adresse, Rechte).
  `authenticated` entfällt. `invalid` trennt nicht zwischen unbekanntem Account, falschem
  Token und gesperrt; ein halbes Header-Paar ist `invalid`. Revision der Replica = Stand, bis
  zu dem alle ihre Collections abgeglichen sind; ohne Replica leer („noch nie abgeglichen“).
  Der Textteil bleibt eine Zeile je Hub. Offen für Refine: ob Header zu Aliasen, die der Node
  nicht kennt, gemeldet werden (Hilfe bei falsch eingerichteten Clients).
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
  während `serve`; gleichzeitiger `node sync`; Beenden während eines Abgleichs.

### Etappe 3 — `whoami`

- Antwort nach Konzept, eine Funktion für MCP und CLI.
- Tests über MCP: alle Hubs, `ok`/`invalid`/`missing`, User, Node-Name, Stand, Version; in der
  rohen Antwort kein Token, Hash, Adresse, Transport, `hub_id`.

### Etappe 4 — `kephalaion node whoami`

- Liste und Einzelansicht, `--hub`, `--json`.
- Tests: Liste nach `rotate`, gesperrter Account fehlt, `--json` gleich der MCP-Antwort.

### Etappe 5 — Doku

- `README.md`, `docs/begriffe.md` (`sync_interval`, `config set`, `node whoami`, whoami),
  `docs/konzept.md` (Stand), `docs/vscode.md` (Stand), `k-playbook-local/k-playbook.md`
  (Satz „kein Abgleich im Hintergrund“ ersetzen), `docs/fortschritt.md`.
