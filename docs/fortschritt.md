# Fortschritt

Stand: 2026-09-26 (Task 008 abgeschlossen, in `done/`; Task 010 erledigt)

## So wird diese Datei aktualisiert

- **Wann:** nach jeder abgeschlossenen Etappe, nach jedem Task, der nach `done/` wandert, und
  wenn im Gespräch etwas entschieden oder neu als offen erkannt wird.
- **Quellen, in dieser Reihenfolge:**
  1. `k-playbook-local/tasks/*.md` — Tabelle „Fortschritt“ je Task (offene Etappen);
  2. `k-playbook-local/tasks/done/*.md` — Abschnitte „Offen (nicht gefixt)“, „Offen:“ und
     „Intent-Alignment“ der Ausführung (was nicht oder nur teilweise geprüft wurde);
  3. `git log --oneline` und `git status` — was committet ist, was gerade in Arbeit ist;
  4. `docs/konzept.md` — Abschnitte „Offene Punkte“, „Stufen“, „Werkzeuge“ und alles mit
     „offen“, „zurückgestellt“, „vorgemerkt“;
  5. `k-playbook-local/material/befunde/` — Befunde mit Status ungleich `geklaert`.
- **Wie:** nur Stichpunkte, jeder mit Verweis auf seine Quelle (Task/Etappe, Abschnitt).
  Erledigtes aus „In Arbeit“ und „Zu tun“ nach „Erledigt“ verschieben, dort knapp halten.
  Entschiedenes aus „Zu besprechen“ streichen — die Entscheidung gehört in `konzept.md`,
  nicht hierher. Datum oben anpassen.
- **Auftrag an die KI:** „Aktualisiere `docs/fortschritt.md` nach der Anleitung am Anfang der
  Datei.“

## Erledigt

- **Task 001 — Gerüst:** Modul, `version`, Makefile, CI, Release per Tag (Entwurf → Assets →
  veröffentlichen), `install.sh`, `upgrade` mit `SHA256SUMS`; v0.1.0 und v0.1.1 veröffentlicht.
- **Task 002 — Datenbank, config, status:** `hub init`, `node init`, `status`,
  `config show|export|import`; `sqlq`, `sqlitedb`, Hub-/Node-Store, Trenntest Hub ↔ Node.
- **Task 003 — Collections, Nodes, Hubs:** Schema 2, `hub_id`, `ident`, CLI
  `hub collection|node …`, `node hub|collection …`, Token nur über stdin, Export Format 2.
- **Task 004, Etappe 1–4:**
  - `listen` in der config, `hubs.node_name` (Node-Schema 3, Export Format 3);
  - Dokumente am Hub: `hub doc put|get|list|rm`, `hub import` (eine Revision je Import);
  - Vertrag Fassung 1 (`docs/vertrag.md`, `internal/contract`), Hub-Seite des `sync`;
  - Replica je Hub unter `replicas/`, Abgleich am Node (`internal/node/replica`), Reset bei
    `hub_id`-Wechsel bzw. `since` > Hub-Revision.
- **Task 006 — User je Account** (2026-09-26): `hub account add|set --user` (ohne Angabe der
  Name des Accounts), `list --user`, `show`; `accounts."user"` mit Index, Hub-Schema 5; User in
  jeder `SYSTEM:A:`-Zeile, `set --user` schreibt alle lebenden Zeilen unter einer Revision;
  `rotate` schreibt `updated_by` = User; `whoami` (`/v1/`, `local`, MCP) nennt ihn nur bei
  gültiger Anmeldung; Export Format 5 mit `user` (Pflicht), Format 4 → User = Name
  (`konzept.md`, „Account und User“).
- **Task 008 — Abgleich im Hintergrund, whoami, node whoami** (2026-09-26, Etappen 1–5,
  in `done/`; Intent-Alignment „Teilweise“, siehe „Zu tun“):
  - `serve` gleicht als Node selbst ab: beim Start, dann je `sync_interval` (Standard 30 s,
    `0` aus), je Eintrag eine Goroutine, `https`/`ssh` übergangen; Log nur bei Zeilen, beim
    ersten Fehler, bei Wechsel der Art des Fehlers und bei Erholung;
  - `config set|unset <rolle> <schlüssel> [<wert>]`, erster Schlüssel `sync_interval`;
  - Stand je Hub in `hub_sync` (Node-Schema 4), geschrieben von `serve` und `node sync`,
    gezeigt von `status` und `whoami`; nicht im Export;
  - Nebenläufigkeit: `hubs.entry_id` (ULID, nie wiederkehrend) auch in `db_info` der Replica
    (Replica-Schema 3); jede schreibende Transaktion prüft Eintrag, `hub_id` und Stand, Zeilen
    und Stände nur vorwärts (Task 008, Etappe 2);
  - MCP `whoami` nach Konzept: `version`, alle Hubs mit `login`, `node`, `sync`,
    `unknown_hubs`; Anmeldung über alle Hubs als `mcpnode.Authenticate` (Grundlage für Task 009);
  - `kephalaion node whoami [<account>] [--hub] [--json]` aus derselben Funktion.
- **Task 010 — Arbeitsbranch `dev`** (2026-09-26): gearbeitet und gesichert wird auf `dev`,
  `main` rückt nur über `make -C k-playbook-local release` per Fast-Forward vor (von `dev`,
  gepusht, CI grün; alle Prüfungen vor dem ersten Push, wiederholbar); `sichern` nur auf
  `dev`; CI-Push nur für `main`/`dev`; Dependabot gegen `dev`; Ruleset „main und dev“ sperrt
  Force-Push und Löschen (`k-playbook-local/k-playbook.md`, „Branches“, „Release“).

## In Arbeit

- **Task 004, Etappe 5 — CLI und status am Node** (uncommittete Änderungen im Arbeitsbaum):
  - `openHubOf`/`openNodeOf` in `cmd/kephalaion/command.go` (Verdrahtung `local`);
  - `printListing` für Hub und Replica gemeinsam (`listedDoc` in `doccmd.go`);
  - `config import` am Node entfernt Replicas nicht mehr enthaltener Hub-Einträge.
  - Noch offen: `node sync`, `node doc list|get`, `status` je Hub/Collection, Tests über
    `run()` (import → sync → list/get → rm → sync).

## Zu tun

- **Task 004, Etappe 6 — Doku:** README (einspielen, abgleichen), `begriffe.md`,
  `k-playbook-local/k-playbook.md` (Vertrag, Replica), `konzept.md` nachziehen: Rolle
  `replica` in `db_info`, Replica als Ausnahme von „nur `init` legt an“, `hubs.node_name`,
  Abgleich-Verfeinerungen aus `vertrag.md`; Absatz „Stand“ oben im Konzept mit Etappe 5 nachziehen.
- **Task 004 abschließen** und nach `done/` — Vorbedingung für Task 005.
- **Task 005 — Kommunikation** (reviewt, nicht begonnen):
  1. Accounts am Hub (`hub account …`, `SYSTEM:A:`-Zeilen, `admin` reserviert, Export);
  2. Vertrag um `whoami`/`rotate`, Hub über HTTP `/v1/`;
  3. Node: Transport `http`, `node hub check`, `--create`, `node account rotate|check`;
  4. `kephalaion serve` (nur Loopback, Sperrdatei, Logs ohne Token);
  5. Node als MCP-Server `/mcp` mit Werkzeug `whoami`;
  6. Durchlauf mit Testaccounts und Doku.
- **Rotation des Node-Tokens** (vorgemerkt 2026-09-26): Ein Node rotiert sein Token bei einem
  Hub selbst, anders als ein Account — er hält es in `node.db` (`hubs.token`) und kann das neue
  dort ablegen, ohne fremde Konfiguration. Vorbild `rotate` der Accounts: neues Token vor dem
  Aufruf als ausstehend merken, altes zur Anmeldung, Hash des neuen, danach ersetzen.
  **Entschieden 2026-09-26: Auch beim Node ist der erste Vorgang ein `rotate`** — das Token aus
  `hub node add` taugt nur zur Einrichtung; das erste `rotate` prüft Verbindung und
  Zusammenspiel gleich bei der Einrichtung. Kommt erst, wenn `rotate` gebaut ist. Grundsatz: Nodes werden wie Accounts behandelt, außer wo es anders sinnvoll
  ist (`konzept.md`, „Offene Punkte“, Token-Rotation).
- **Danach (Konzept, „Stufen“):**
  - Stufe 1: Zerlegung in Abschnitte, FTS5-Index, MCP-Werkzeuge `search`, `read`, `list`
    (Task 009); Ereignisstrom (SSE/Long-Polling, Todo #10); Abgleich unmittelbar nach eigenem
    Schreiben;
  - Stufe 2: Schreiben über den Node mit Rechten (`create`, `write`, `delete`,
    `create_numbered`), Fehlercodes „Name vergeben“, Revision als Vorbedingung;
  - Stufe 3: `append`, `replace_section`, `rename`, `supersede`, `replace_directory`;
  - Transporte `https` und `ssh`; Hub außerhalb von Loopback;
  - Einrichtung als Dienst (systemd `--user`, launchd), Neustart nach `upgrade`;
  - Lauschen auf der Docker-Bridge für Devcontainer;
  - PostgreSQL-Umsetzung des Hub-Stores (DDL, `BIGINT`);
  - Migrationsrahmen, sobald Daten bleiben müssen;
  - Kommando für eine neue `hub_id` nach Wiederherstellung aus einer Sicherung;
  - Markdown-Export des Stores;
  - Begrenzung von Fehlversuchen bei der Anmeldung.
- **Nachbesserung Task 008, vor dem nächsten Release** (Code-Review und Intent-Alignment,
  `done/008-…`, „Ausführung“) — Task 012:
  - `whoami`/`node whoami` scheitern insgesamt, wenn eine einzige Replica nicht lesbar ist
    (alte Schemafassung, beschädigt); Fehler je Hub abbilden (Befund 1, Hoch);
  - Race in `replica.Create` zwischen `os.Link` und `Open`: nach `Open` die `entry_id` prüfen,
    sonst `ErrChanged` (Vorschlag 1);
  - `serve` wartet beim Beenden ohne Frist auf den Abgleich (`<-bgDone`); mit
    `shutdownGrace` begrenzen (Vorschlag 2).
- **Kleinere Punkte aus dem Review von Task 008** (`done/008-…`, „Code-Review“, Vorschläge 3–12):
  - `reset` prüft nur `entry_id`, nicht die alte `hub_id` — doppeltes Verwerfen bei zwei
    parallelen Resets (3);
  - `DescribeSync` dereferenziert `Revision` ohne nil-Prüfung (4);
  - neues `sync_interval` wirkt erst nach dem laufenden Timer, bei `0` bis zu 30 s
    (`syncIdle`); in der Hilfe nennen (5, Ausführung);
  - `outcome`/`skipped` in `bgsync.go` für entfernte Einträge nicht geräumt; https → http →
    https ohne zweite Logzeile (6);
  - verwaiste `.db.new-<ULID>`-Dateien nach einem Absturz; `os.Link` setzt Hardlinks voraus (7);
  - je Runde ein neuer `connector`, bei `http` womöglich offene Idle-Verbindungen (8);
  - `whoami` liest die Hubs zweimal und öffnet je Replica bis zu zweimal (9);
  - nach `Remove` in `openForSync` sehen lesende Pfade eines alten `*sql.DB` womöglich die neue
    Datei — nur schreibende Pfade geschützt; im Kommentar festhalten (10);
  - Log-Tests nur langsam über `serve` (`TestBackgroundSyncErrors` ~9 s); Unit-Test der
    Log-Zustandsmaschine mit gefälschtem Connector (11);
  - geloggte „Revision“ ist das Minimum über die Collections; so benennen (12);
  - ungültiges `sync_interval` aus `config import` wird nicht abgewiesen, `serve` nimmt 30 s
    (Ausführung, Restrisiken);
  - `last_error` in `whoami` nur als fester Satz je Fehlerart (Adresse); bewusst, ggf.
    besprechen (Ausführung, Restrisiken).
- **`TestBackgroundSync` wackelt in CI:** scheiterte auf `dev` (a92d9e1, nur Task-Dateien
  geändert) mit „wartet vergeblich auf: Logzeilen“; `release` verlangt grüne CI auf `dev`
  (Task 010, Etappe 2).
- **Kleinere Punkte aus Reviews (Task 003):**
  - `node hub add` prüft Transportregeln erst nach der Token-Eingabe;
  - `parseFlags`: Flag-Wert `--` gilt als Ende der Optionen;
  - `status` auf eine schreibgeschützte Datenbank;
  - Index für `CollectionCountDocs`/`AccountRows` prüfen, jetzt wo es Dokumente gibt.

## Zu testen

- **Task 008:** Schemafassung +1 für `node.db` (4) und Replica (3): Vor dem Update
  `config export` mit dem alten Binary, danach `node init` neu und `config import` (Task 008,
  Review-Punkt 9, offen) — wie man neu anlegt, obwohl `node init` bei eingetragener Rolle
  abbricht, ist nirgends beschrieben. Abgleich im Hintergrund mit einem echten Client über längere Zeit;
  Der Race-Detector lief über `cmd/kephalaion` und `internal/node/...` sauber.

- **Task 004, Etappe 5:** Durchlauf Hub + Node in einer config über `local`.
- **Task 005, Etappe 6:** Durchlauf `rotate` über `local` und `http`, `whoami` per MCP aus
  einem echten Client, `lock` → `sync` → `whoami` scheitert.
- **Erster Security-PR von Dependabot** gegen `main`: lokal nach `dev` holen und prüfen, ob
  GitHub ihn nach dem Release als gemergt markiert — auch wenn Dependabot den Branch rebased
  (Task 010, Review-Punkt 5, vertagt).
- **macOS:** Installation und `upgrade` nie echt getestet (Task 001, Intent-Alignment).
- **`upgrade`-Abbruch:** nur per httptest belegt, nicht durch einen echten Abbruch.
- **PostgreSQL:** Tauglichkeit der Hub-Abfragen nur per Check auf verbotene Konstrukte;
  Eindeutigkeit bei gleichzeitigen Schreibern liefert dort rohe Treiberfehler (Task 003).
- **Großer Import:** eine Revision = eine unbegrenzte Seite; Verhalten über HTTP prüfen.
- **Ranking mit FTS5:** Korrektur aus k-playbook Task 056 (Zeiger vor Zielen) neu nachweisen,
  sobald die Suche steht.

## Zu besprechen

- **Name:** TMview-Recherche (griechische nationale Marken, wegen Kefalaio) und Domain
  reservieren. Marke erst bei Entscheidung zur Vermarktung (siehe Konzept, „Der Name“).
- **Release-Signatur** statt nur `SHA256SUMS` (cosign/minisign/Attestations) — wann?
- **Welcher entfernte Transport zuerst:** `https` oder `ssh`?
- **Obergrenze je Schreibvorgang** oder Datenstrom für große `sync`-Seiten über HTTP.
- **Verwaltung über MCP:** eigenes Recht (`admin` je Hub?), wer am Node verwalten darf, ob
  Werkzeuge nur mit Recht erscheinen, Token-Ausgabe ohne KI-Kontext.
- **k-playbook ↔ Kephalaion:** welche k-playbook-Werkzeuge (Eingang, Warteschlange, Todos,
  Tasks, `publish`, Status) Kephalaion trägt; welche die KI nicht sehen soll; in welcher
  Collection die Tasks eines Projekts liegen; wie Werkzeuge zuschaltbar werden.
- **Token-Rotation mit Frist** für Menschen/KIs; wie ein neues Token zu k-playbook gelangt.
- **Persönliche Verzeichnisse** (`personal`, `numbered` als Eigenschaften eines Verzeichnisses)
  — vorgemerkt; ob es sie braucht, wer sie setzt, Übergabe an einen anderen User
  (`konzept.md`, „Persönliche Verzeichnisse“).
- **Collection „nur nach Bestätigung“** schreiben — ja/nein?
- **Was eine Collection im Betrieb ist** (Team, Produkt, Thema).
- **Ausgangskorb** bei nicht erreichbarem Hub — derzeit nein.
- **`append`:** gemeinsamer Vertrag mit k-playbook Task 078.
- **Semantische Suche (Stufe 5):** Einbettung der Frage lokal oder BM25 zuerst.
- **Zurückgestellt, bei Bedarf:** Schnipsel und Einordnen durch den Hub, KI im Hub (Kosten,
  Anbieterbindung, Protokoll, Warteschlange), Dopplungen, History (`document_versions`),
  stdio-Bridge, `write` als Namenspräfixe.
