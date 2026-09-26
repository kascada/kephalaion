# Projektregeln

Diese Datei gilt nur für dieses Projekt. Sie wird nach der mitgelieferten
Ebene gelesen und kann deren Aussagen ergänzen oder überstimmen.

Was hier hineingehört: Aufbau und Besonderheiten des Projekts, Konventionen,
wiederkehrende Abläufe, alles was ein Assistent in jeder Sitzung wissen sollte.

Was nicht: allgemeine k-playbook-Regeln — die stehen in der mitgelieferten
Ebene und werden bei jedem Update aktualisiert.

## Aufbau

Ein Go-Modul `github.com/kephalaion/kephalaion`, ein Binary `kephalaion`
(`cmd/kephalaion`). Unterkommandos per erstem Argument, Standardbibliothek `flag`,
keine CLI-Bibliothek. Meldungen des Binarys deutsch, Bezeichner englisch. Begriffe stehen
in `docs/begriffe.md` und werden dort eingetragen, bevor sie benutzt werden.

Pakete unter `internal/`:

- `config` — die config (`config.yaml`): Ort auflösen, strikt lesen, atomar schreiben,
  Rolle eintragen, db-Adressen zerlegen.
- `sqlitedb` — gemeinsamer Unterbau beider Datenbanken, kennt weder Hub noch Node: SQLite
  öffnen (`foreign_keys`, `busy_timeout`; eine fehlende Datei wird nie angelegt, nur
  `Create` legt an), `db_info` prüfen (Schemafassung, Rolle), `settings` lesen und schreiben.
  WAL setzt nur `Create`; `Open` ändert die Datei nicht, auch nicht ihren `journal_mode`.
  Transaktionen beginnen IMMEDIATE (DSN-Parameter `_txlock=immediate`, kein PRAGMA): Sie
  nehmen die Schreibsperre sofort, auch wenn sie nur lesen — reine Lesezugriffe laufen
  deshalb ohne Transaktion.
- `ident` — neutral, für Hub und Node: Namensregel, Adresse `<hub>:<collection>`, Token
  erzeugen, hashen, Format prüfen, gekürzt anzeigen; Pfadregeln für Dokumentnamen
  (`CheckDocName`, `SYSTEM:` abgelehnt); `CheckPrincipalName` für Accounts und Nodes (`admin`
  reserviert), `LogName` für Namen im Log.
- `sqlq` — Hilfe für PostgreSQL-taugliche Abfragen: Platzhalter `$n`, `Bind` je Dialekt,
  `Check` auf verbotene Konstrukte.
- `hub/store`, `node/store` — die gekapselten Datenbanken von Hub und Node, je eine
  Schnittstelle `Store` mit SQLite-Umsetzung und DDL.
- `contract` — neutral, der Vertrag zwischen Node und Hub als Go-Typen: Anfragen, Antworten,
  Fehlercodes, Schnittstelle `Hub` (`Whoami`, `Rotate`, `Sync`), Fassung (`contract.Version`),
  Form der Account-Zeilen (`AccountContent`).
- `contract/httpapi` — neutral, der Vertrag über HTTP: `NewHandler` bedient jede Umsetzung von
  `contract.Hub`, `Client` setzt sie über HTTP um (Wiederholung nur für `whoami` und `sync`,
  `rotate` nie; unklarer Ausgang als `contract.ErrOutcomeUnknown`; folgt keiner Weiterleitung,
  3xx ist ein eindeutiger Fehler).
- `reqlog` — neutral, eine Logzeile je HTTP-Anfrage; Namen nur über `ident.LogName`, nie ein
  Token.
- `loopback` — neutral, die Prüfung, dass eine HTTP-Anfrage diesen Rechner meint: `Host`
  Loopback mit dem Port, auf dem sie ankam, sonst 403. Hub-Listener (`serve`) und
  `node/mcpnode` benutzen dieselbe Prüfung.
- `hub/replication` — die Seite des Hubs im Vertrag: setzt `contract.Hub` über dem Hub-Store
  um (Anmeldung von Node und Account, erlaubte Collections, Seitenschnitt); der Store liefert
  nur Zeilen und schreibt `rotate` in einer Transaktion.
- `node/replica` — die Replica des Nodes (je Hub-Eintrag eine SQLite-Datei) und der Abgleich
  (`Syncer`), der sie über `contract.Hub` füllt; dazu die Account-Zeilen (`AccountRows` über
  den Teilindex `documents_system`, `WriteAccountRows` nach `rotate`).
- `node/mcpnode` — der MCP-Eingang des Nodes (`/mcp`, go-sdk, zustandslos): Host/Origin,
  Header-Paare je Hub, Anmeldung über alle Hubs (`Authenticate`), Werkzeug `whoami`
  (`Whoami`, auch für `node whoami`); Werkzeuge `list`, `read`, `changes` (`list.go`,
  `read.go`, `changes.go`) über dem gemeinsamen Schritt in `access.go` (Adresse, Anmeldung,
  Recht, Replica je Hub) und den Abfragen in `node/replica/read.go`.

Regeln dazu:

- **Hub und Node bleiben getrennt.** Kein Paket unter `internal/hub` importiert eines unter
  `internal/node` und umgekehrt, auch nicht über Umwege oder in Tests; Gemeinsames gehört in
  neutrale Pakete. `internal/separation_test.go` prüft das, ebenso, dass `internal/contract`
  weder Hub noch Node importiert.
- **Der Vertrag steht zweimal, gleich.** `docs/vertrag.md` ist verbindlich, `internal/contract`
  folgt ihm; wer den einen ändert, zieht den anderen im selben Commit nach. Er trägt eine
  Fassung (`contract.Version`, derzeit 1), und der Hub soll auch ältere Nodes bedienen.
- **Welche Umsetzung des Vertrags ein Node bekommt, entscheidet nur `cmd/kephalaion`.** Dort
  ist sie verdrahtet (`connector` in `synccmd.go`): `local` ist der Hub der eigenen config mit
  `hub/replication` darüber, `http` der Client aus `contract/httpapi` (nur Loopback);
  `internal/node` kennt nur `contract.Hub`. Auch `local` prüft die Anmeldung wie jeder
  Transport; die Tests des Vertrags (`hub/replication`) laufen gegen `local` und HTTP. Ein
  Fehler von `rotate`, der kein Fehler des Vertrags ist, gilt auf jedem Transport als unklar
  (`contract.ErrOutcomeUnknown`) — über `local` hüllt `localHub` in `synccmd.go` ihn ein.
- **Die Replica ist abgeleitet.** Sie enthält nur, was der Hub geliefert hat, und darf wie
  der Node-Store SQLite-Eigenes benutzen. Angelegt wird sie nur vom Abgleich, nie von `init`;
  `node hub rm` und `config import` (für weggefallene Aliase) entfernen sie mit, innerhalb der
  Transaktion, die den Eintrag entfernt. Passt ihre Schemafassung nicht, gehört sie zu einem
  anderen Eintrag oder ist sie eindeutig beschädigt (`NOTADB`/`CORRUPT`, `db_info`, die IDs oder
  die `generation` fehlen), verwirft der Abgleich sie; vorübergehende Fehler verwerfen nichts.
  Eine unlesbare Replica betrifft in `whoami`, `list`, `read` und `changes` nur ihren Hub.
- **Hub-SQL bleibt PostgreSQL-tauglich.** Die Abfragen (DML) des Hubs und des Unterbaus
  stehen zentral in einer Struktur (`queries` bzw. `sqlitedb.Queries`), mit Platzhaltern
  `$n` über `sqlq.Bind` — kein `INSERT OR`, kein `PRAGMA`, kein `AUTOINCREMENT`, kein rohes
  `?`, `user` nur in Anführungszeichen (`"user"`, in PostgreSQL reserviert; auch im DDL). Ein
  Test je Paket prüft sie mit `sqlq.Check`. `PRAGMA` gibt es nur beim Öffnen der
  Verbindung. Das DDL steht je Dialekt. Zähler wie die Revision werden im Code
  hochgezählt, nicht per Umwandlung in SQL. Der Node darf SQLite-Eigenes benutzen.
- **Token nie als Argument.** Ein Token kommt über `--token-stdin` (eine Zeile) oder
  `--token-file` herein, nie über ein Argument — Shell-Verlauf und Prozessliste. Angezeigt wird
  es nur gekürzt (`ident.MaskToken`); ein am Hub erzeugtes Token genau einmal, gespeichert nur
  als Hash. Kein Token in einem MCP-Werkzeug, einer MCP-Antwort oder einem Log; `rotate` ist
  deshalb ein CLI-Kommando, kein Werkzeug.
- **Namensregeln.** Collections, Nodes, Accounts und Hub-Aliase:
  `[a-z0-9][a-z0-9._-]{0,62}`, kein `:`, kein Präfix `system` in beliebiger Schreibweise —
  geprüft mit `ident.CheckName` bzw. `ident.ParseAddress`; Accounts und Nodes zusätzlich mit
  `ident.CheckPrincipalName` (`admin` reserviert, er steht im Protokoll für den Verwalter).
  Die Pfadregeln für Dokumentnamen stehen nur in `ident` (`CheckDocName`, `DocDirPrefix`);
  Hub-Store und Replica benutzen sie. Node- und Account-Namen sind gemeinsam eindeutig,
  geprüft über die Tabellen `accounts` ↔ `nodes` in beide Richtungen (nicht mehr über
  `SYSTEM:A:`-Zeilen) und abgesichert in der Datenbank über `principal_names` (Primärschlüssel,
  `INSERT … ON CONFLICT DO NOTHING` in derselben Transaktion wie Anlegen, Löschen beim
  Entfernen; eine Verletzung ergibt `ErrExists` mit derselben Meldung wie die Vorprüfung). Die
  Tabelle ist abgeleitet: nicht im Export, `config import` baut sie neu auf. Nach `hub account
  rm` ist der Name frei. **Users** gehören nicht dazu: Namensregel wie Accounts, `admin`
  reserviert, geprüft mit `store.CheckUser` (CLI und Import); ein User darf wie ein Node oder
  ein anderer Account heißen.
- **Accounts am Hub.** Die Tabelle `accounts` führt Beschreibung, gesperrt, die gemerkten
  Rechte eines gesperrten Accounts und **maßgeblich Hash und User** (`"user"`, Index
  `accounts_user`); die `SYSTEM:A:`-Zeilen je Account und Collection tragen Rechte und eine
  Kopie von Hash und User. `created_by`/`updated_by` ist der User des schreibenden Accounts,
  die CLI am Hub schreibt `admin`; `actions.account` bleibt der Account. `set --user` schreibt
  alle lebenden Zeilen des Accounts unter einer Revision neu, Löschmarken und Dokumente
  bleiben. Jede Änderung an den Zeilen
  ist ein Schreibvorgang mit Revision und genau einer Zeile in `actions` (`admin`, bei `rotate`
  der Account mit dem Node als `carrier`) und schreibt Hash in `accounts` und Zeilen in
  derselben Transaktion. **Die Zeile in `accounts` wird zuerst gesperrt**: Die erste Anweisung
  jedes Schreibvorgangs an einem Account (`writeAccount`) ist `UPDATE accounts SET name = name
  WHERE name = $1`, erst danach wird gelesen — kein `SELECT … FOR UPDATE` (SQLite). `rotate`
  beginnt stattdessen mit dem bedingten Schreiben (`… AND token_hash = $3 AND locked = 0`) und
  prüft die Zahl der Zeilen, liest danach den User; der Import sperrt vorher alle Zeilen von
  `accounts`. Zeilen werden
  nie entfernt: Löschmarke, und bei erneutem `grant` wiederbelebt — auch wenn die Collection
  entfernt wird: Löschmarken von `SYSTEM:A:`-Zeilen blockieren das Entfernen nicht (CLI und
  Import) und bleiben stehen; lebende Zeilen und gemerkte Rechte eines gesperrten Accounts
  blockieren weiter.
- **Import prüft wie die CLI.** `config import` benutzt dieselben Prüffunktionen
  (`CheckTables` je Store) und prüft alles, bevor geschrieben wird; erst der Hub, dann der
  Node. Exportformat 5 trägt die Accounts samt User und Rechten; der Import gleicht die
  `SYSTEM:A:`-Zeilen unter einer Revision an. Ein Export vor Format 4 lässt die Accounts und
  darf keinen Accounts-Teil tragen, in keiner Form (geprüft am YAML-Knoten); ab Format 4 ist
  null ein Fehler, nur `accounts: []` leert. `user` je Account ist ab Format 5 Pflicht
  (fehlend oder null am YAML-Knoten geprüft, leer oder ungültig mit `store.CheckUser`); Format 4
  darf ihn nicht tragen und setzt ihn auf den Namen des Accounts.
- **`serve` lauscht nur auf Loopback**, beide Rollen, bis `https`/`ssh` kommen, und beide
  prüfen `Host` (`loopback`); ein Tunnel geht nur mit gleichem Port. Er nimmt je
  Rolle eine Sperre (`flock` auf `<db>.lock` neben der Datenbank); CLI-Kommandos laufen daneben
  über SQLite, `status` prüft die Sperre ohne zu warten. Ein Log je Anfrage auf stderr über
  `reqlog`. Als Node gleicht `serve` im Hintergrund ab (`cmd/kephalaion/bgsync.go`): beim Start
  und je `sync_interval` (`settings`, je Runde gelesen, `0` aus), je Hub-Eintrag eine
  Goroutine, verdrahtet über `connector` wie `node sync` — `local` bekommt den Hub-Store
  desselben `serve`. Beim Beenden bricht er ab, bevor die Stores schließen; `serve` wartet
  darauf höchstens `shutdownGrace`. Log nur bei Zeilen, erstem Fehler, Wechsel der Fehlerart
  (`replica.ErrorKind`) und Erholung.
- **Nebenläufigkeit am Node:** `node sync`, `node hub rm|add`, `config import` und `rotate`
  laufen als eigene Prozesse neben `serve`. Keine Sperre über Prozesse: Jedes Schreiben des
  Abgleichs ist an `hubs.entry_id` gebunden (ULID, beim Anlegen vergeben, nie wiederkehrend,
  `config import` behält sie für bleibende Aliase), die auch in `db_info` der Replica steht.
  Jede schreibende Transaktion der Replica prüft zuerst `entry_id`, `hub_id` und dass kein
  Stand unter dem `since` der Seite liegt (`ErrChanged`); `SetHubID` und `RecordSync` in
  `node.db` schreiben nur für die `entry_id` (`ErrEntryGone`). Zeilen per id und Stände gehen
  nur vorwärts. `replica.Create` baut unter eigenem Namen und linkt fertig an den Ort.
- **Stand des Abgleichs** je Hub in `hub_sync` (`node.db`): abgeleitet, nicht im Export,
  `node hub rm` und `config import` räumen mit ab; `Syncer.SyncEntry` schreibt ihn (für
  `serve` und `node sync`), außer bei Abbruch oder entferntem Eintrag.
- **MCP am Node:** `internal/node/mcpnode`. Clients melden sich je Hub mit
  `X-Keph-Account-<alias>` und `X-Keph-Token-<alias>` an; der Node prüft sie je Anfrage gegen
  die Replica, ohne Cache und ohne die Replica offen zu halten — über alle Hub-Einträge in
  einem Schritt (`Authenticate`, `ok`/`invalid`/`missing`, dazu unbekannte Aliase), auf dem
  jedes Werkzeug aufsetzt. Die Antwort von `whoami` baut `mcpnode.Whoami`, dieselbe Funktion
  für das Werkzeug und für `kephalaion node whoami` (dort `AccountLogins` ohne Token).
  `list`, `read` und `changes` lesen nur aus der Replica: lesbar ist eine Collection mit
  gültiger Anmeldung und lebender `SYSTEM:A:`-Zeile des Accounts; alles andere ist dieselbe
  Meldung „nicht lesbar“, eine fehlende Replica „noch nie abgeglichen“. Eine unlesbare Replica
  betrifft nur ihren Hub (feste Meldung ohne Pfad, `unreadable_hubs`; die volle Meldung ins
  Log), ein abgebrochener ctx ist ein Fehler der Anfrage. Nie `SYSTEM:`-Namen, Löschmarken nur
  in `changes`. Gelesen wird ohne Transaktion; `changes` erkennt eine neu angelegte oder
  geleerte Replica an der `generation` in ihrem `db_info`, die es vor allem anderen liest.
- **Keine Migrationen, Schema neu anlegen** — befristet, solange es keine Daten gibt, die
  bleiben müssen. Ändert sich das Schema, wird `SchemaVersion` im Store-Paket erhöht;
  vorhandene Datenbanken werden dann abgelehnt und neu angelegt. Dokumente am Hub gelten
  vorerst als wiederherstellbar per `hub import`; die Replica gleicht sich neu ab.

## Bauen

- Über das `Makefile` im Wurzelverzeichnis, lokal und in CI gleich: `make build` bzw.
  `make dist-host` für diese Plattform, `make dist` für alle vier (linux/darwin ×
  amd64/arm64) plus `SHA256SUMS` nach `dist/` (gitignored), `make dev-install` ersetzt
  `~/.local/bin/kephalaion`.
- Flags stehen nur im Makefile: `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, Version
  und Commit per `-ldflags -X` in `internal/buildinfo`. Ohne `VERSION=` entsteht ein
  dev build.
- Die Toolchain ist die `toolchain`-Zeile in `go.mod`. CI baut mit `GOTOOLCHAIN=local` und
  prüft sie mit `make check-toolchain`. Wer sie anhebt, hebt sie nur dort an.
- Actions in `.github/workflows/` sind auf Commit-SHA gepinnt, mit Versionskommentar;
  Dependabot hält `gomod` und `github-actions` wöchentlich nach, mit PRs gegen `dev`.

## Testen

- Vor jedem Commit mit Code: `make check` (gofmt, `go vet`, `go test`, `sh -n install.sh`).
- `shellcheck` über `install.sh` läuft nur in CI; lokal ist es nicht installiert.
- Von Hand, weder in `make check` noch in CI: `make race` (Race-Detector, braucht cgo und
  einen C-Compiler), `make cover` (Abdeckung je Paket einschließlich der Tests anderer Pakete,
  Bericht nach `coverage/`) und `make mutate` (Mutationstests mit gremlins, per `go run` in
  fester Fassung, in einer Kopie von `go.mod`, `go.sum`, `cmd` und `internal`;
  `MUTATE=./internal/<paket>` für ein Paket).
- Tests gegen GitHub laufen über `httptest`; die Basis-URL der API ist dafür im
  `upgrade.Upgrader` überschreibbar. Sie ist keine Nutzeroption und wird nicht dokumentiert.

## Branches

- **`dev`** ist der Arbeitsbranch: dort wird gearbeitet und gesichert. Auf `dev` darf jeder
  ohne Prüfung und ohne Rückfrage sichern und pushen, auch die KI und `/k-task-run` — `dev`
  ist nur die Sicherung des Arbeitsstands.
- **`main`** trägt nur veröffentlichte Stände und rückt allein über `release` per
  Fast-Forward auf `dev` vor. Nach `main` wird nie direkt committet oder gepusht. Das ist
  Regel, keine Sperre: Das Ruleset „main und dev“ auf GitHub sperrt für beide Branches nur
  Force-Push und Löschen. Weicht `main` doch ab (etwa ein auf GitHub gemergter PR), erkennt
  `release` das in Schritt 6 und bricht ab.
- `main` bleibt Standard-Branch auf GitHub: Ein Clone bekommt den Release-Stand; wer arbeiten
  will, wechselt nach `dev` (`git switch dev`).
- **Dependabot:** Versions-Updates kommen als PR gegen `dev` (`target-branch` in
  `.github/dependabot.yml`; wirkt erst, wenn `main` die Datei enthält). Security-Updates gehen
  nach GitHub-Vorgabe immer gegen `main`. Solche PRs nicht auf GitHub mergen, sondern lokal
  nach `dev` holen (`git fetch origin`, `git merge origin/<branch>`, sichern); enthält `main`
  die Commits nach dem nächsten Release, markiert GitHub den PR als gemergt.

## Release

- Ein Release ist ein Tag `vX.Y.Z` (mit Suffix `-…` eine Vorabversion). Es gibt keine
  `VERSION`-Datei und keine versionierte `SHA256SUMS`.
- Anlegen nur über `make -C k-playbook-local release VERSION=vX.Y.Z`, von `dev` aus. Alle
  Prüfungen laufen vor dem ersten Push: gültige Version, Branch `dev`, sauberer Arbeitsbaum,
  `HEAD == origin/dev` (nach `git fetch`), `origin/main` Vorfahre von `HEAD`, Tag weder auf
  `origin` noch lokal auf einem anderen Commit, CI (`ci.yml`) auf `HEAD` grün — es zählt der
  neueste Lauf auf `dev`, abgefragt mit `gh`. Danach schiebt es `main` per Fast-Forward auf
  `HEAD` (`git push origin HEAD:refs/heads/main`, ohne Force), legt den Tag annotiert an,
  pusht ihn und zieht den lokalen `main` nach.
- Bricht es nach dem Push von `main` ab, wird es einfach wiederholt: `main` steht dann schon
  auf `HEAD`, ein lokal schon angelegter Tag auf `HEAD` wird nur noch gepusht.
- Den Rest macht `.github/workflows/release.yml` (Workflow „Release“): `make check`, `make
  dist`, Release als Entwurf, Assets (vier Binaries, `SHA256SUMS`, `install.sh`), dann
  veröffentlichen. `latest` bekommt nur die höchste Version ohne Suffix.
- Scheitert ein Lauf, wird er über `workflow_dispatch` mit dem Tag wiederholt
  (`gh workflow run release.yml -f tag=vX.Y.Z`); vorhandene Assets werden ersetzt.
- Ein Release wirkt öffentlich: Tag, Push und Veröffentlichung nur nach Rückfrage beim
  Nutzer. Das gilt für `release` (und damit für jeden Push nach `main`), nicht für Pushes auf
  `dev`. Ein v0.x-Release gilt nicht als Veröffentlichung im Sinne der Namensprüfung
  (`docs/konzept.md`); die ist erst vor v1.0 fällig.

## Sichern

- `make -C k-playbook-local sichern [MSG=…]` committet alles und pusht nach `origin dev`,
  ohne Prüfung — für Zwischenstände, nicht für Releases. Es läuft nur auf `dev`; auf einem
  anderen Branch bricht es ab. Ist `dev` auf `origin` weiter, lehnt der Push ab: dann
  `git pull --no-rebase origin dev` und erneut sichern. Da `git add -A` alles nimmt, vorher
  sehen, was im Arbeitsbaum liegt.
