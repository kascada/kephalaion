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
  `actions`; auf dem Node zählt die `id`), „Abgleich“ (Revision als Zähler in einer Zeile).
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
- **Kapselung:** je eine Schnittstelle für Hub- und Node-Datenbank, SQLite als erste
  Umsetzung. Hub: nur SQL, das auch PostgreSQL versteht (Platzhalter über eine kleine
  Hilfe, kein `INSERT OR …`, kein SQLite-eigenes `PRAGMA` in Abfragen; die Verbindung
  darf es setzen). Node: SQLite-eigenes erlaubt — dort kommt später FTS5.
- **SQLite-Verbindung:** WAL, `foreign_keys=ON`, `busy_timeout`. `CGO_ENABLED=0` bleibt.
- **Schema anlegen, keine Migrationen.** Die Schemafassung steht in `meta`. Passt sie nicht
  zum Binary, bricht jeder Zugriff mit klarer Meldung ab: Datenbank neu anlegen, die
  Einstellungen über `config export`/`import` retten. Ein Rahmen für Migrationen entsteht
  erst, wenn es Daten gibt, die bleiben müssen.
- **Rolle in der Datenbank:** `meta` hält auch die Rolle (`hub`/`node`). Eine Hub-Datenbank
  als Node zu öffnen (oder umgekehrt) ist ein Fehler.
- **Revision:** Zähler als Zeile in `meta`, erhöht in der schreibenden Transaktion — keine
  `SEQUENCE`, kein Autoincrement. In diesem Task schreibt noch niemand; der Zähler steht
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

- `internal/hub/store/` (Name nach Ermessen, Hauptsache getrennt vom Node): Schnittstelle,
  SQLite-Umsetzung, Schema anlegen.
- Schema: `meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)` mit `schema_version`, `role`,
  `revision`, `created_at`; `settings (key, value)`; `documents` und `actions` samt Indizes
  genau nach „Datenmodell“ im Konzept (Hub: mit eindeutigem Index auf `(collection, name)
  WHERE deleted = 0` und dem Index für `SYSTEM:`-Zeilen).
- Abfragen für `status`: Schemafassung, Revision, Anzahl Dokumente (ohne Löschmarken),
  Anzahl Collections (verschiedene `collection`).
- Tests in `t.TempDir()`: anlegen, erneut öffnen, falsche Schemafassung, falsche Rolle.

### Etappe 3 — Datenbank des Nodes

- `internal/node/store/`: Schnittstelle, SQLite-Umsetzung, Schema `meta` und `settings`.
  Replicas kommen später als eigene Dateien daneben, nicht in diesem Task.
- Tests wie beim Hub.

### Etappe 4 — CLI: init und status

- `kephalaion hub init [--db …]`, `kephalaion node init [--db …]`, jeweils mit `--config`.
  Bricht ab, wenn die Rolle in der config steht oder die Datenbankdatei schon existiert.
  Meldet am Ende, was angelegt wurde.
- `kephalaion status [--config …]`: Ort der config (und ob es sie gibt); je Rolle
  eingerichtet ja/nein, db-Adresse, öffnet sie, Schemafassung; Hub zusätzlich Revision,
  Dokumente, Collections; Node: „Hubs: keine“. Ist nichts eingerichtet, Hinweis auf
  `hub init`/`node init`. Eine Rolle, deren Datenbank fehlt oder nicht passt, wird als
  Fehler dieser Rolle gemeldet; Exit-Code ≠ 0 nur dann.
- Hilfetexte im Stil von `main.go`; `usage` um die neuen Kommandos ergänzen.
- Tests über `run()`: nichts eingerichtet, nur Hub, beide, zweites `init` scheitert.

### Etappe 5 — config show, export, import

- `kephalaion config show`: Ort und Inhalt der config, dazu die `settings` je Rolle.
- `kephalaion config export [--output datei]`: YAML mit Fassung des Formats, der config und
  den `settings` je Rolle — keine Inhalte. Datei mit `0600`, weil `settings` später Token
  enthalten.
- `kephalaion config import <datei>`: schreibt die `settings` in bereits eingerichtete
  Rollen und ersetzt sie dort. Fehlt eine Rolle, die der Export enthält: abbrechen mit
  Hinweis auf `init` (mit dem `db` aus dem Export als Vorschlag). Die config selbst wird
  nicht überschrieben.
- Tests: Export → neue Datenbanken → Import ergibt dieselben `settings`.

### Etappe 6 — Doku

- `docs/begriffe.md`: `export`, `import` (unter `config`), `settings`, `meta` eintragen.
- `README.md`: Einrichten mit `init`, `status`.
- `k-playbook-local/k-playbook.md`: Aufbau um config und die beiden Datenbankpakete
  ergänzen; Regel „Hub-SQL bleibt PostgreSQL-tauglich“; „keine Migrationen, Schema neu
  anlegen“.
- `docs/konzept.md`: nur nachziehen, wo die Umsetzung vom Konzept abweicht.
