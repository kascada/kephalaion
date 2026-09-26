# Projektregeln

Diese Datei gilt nur für dieses Projekt. Sie wird nach der mitgelieferten
Ebene gelesen und kann deren Aussagen ergänzen oder überstimmen.

Was hier hineingehört: Aufbau und Besonderheiten des Projekts, Konventionen,
wiederkehrende Abläufe, alles was ein Assistent in jeder Sitzung wissen sollte.

Was nicht: allgemeine k-playbook-Regeln — die stehen in der mitgelieferten
Ebene und werden bei jedem Update aktualisiert.

## Aufbau

Ein Go-Modul `github.com/kascada/kephalaion`, ein Binary `kephalaion`
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
  (`CheckDocName`, `SYSTEM:` abgelehnt).
- `sqlq` — Hilfe für PostgreSQL-taugliche Abfragen: Platzhalter `$n`, `Bind` je Dialekt,
  `Check` auf verbotene Konstrukte.
- `hub/store`, `node/store` — die gekapselten Datenbanken von Hub und Node, je eine
  Schnittstelle `Store` mit SQLite-Umsetzung und DDL.
- `contract` — neutral, der Vertrag zwischen Node und Hub als Go-Typen: Anfragen, Antworten,
  Fehlercodes, Schnittstelle `Hub`, Fassung (`contract.Version`).
- `hub/replication` — die Seite des Hubs im Vertrag: setzt `contract.Hub` über dem Hub-Store
  um (Anmeldung, erlaubte Collections, Seitenschnitt); der Store liefert nur Zeilen.
- `node/replica` — die Replica des Nodes (je Hub-Eintrag eine SQLite-Datei) und der Abgleich
  (`Syncer`), der sie über `contract.Hub` füllt.

Regeln dazu:

- **Hub und Node bleiben getrennt.** Kein Paket unter `internal/hub` importiert eines unter
  `internal/node` und umgekehrt, auch nicht über Umwege oder in Tests; Gemeinsames gehört in
  neutrale Pakete. `internal/separation_test.go` prüft das, ebenso, dass `internal/contract`
  weder Hub noch Node importiert.
- **Der Vertrag steht zweimal, gleich.** `docs/vertrag.md` ist verbindlich, `internal/contract`
  folgt ihm; wer den einen ändert, zieht den anderen im selben Commit nach. Er trägt eine
  Fassung (`contract.Version`, derzeit 1), und der Hub soll auch ältere Nodes bedienen.
- **Welche Umsetzung des Vertrags ein Node bekommt, entscheidet nur `cmd/kephalaion`.** Dort
  ist `local` verdrahtet (`localConnector` in `synccmd.go`: der Hub der eigenen config,
  `hub/replication` darüber); `internal/node` kennt nur `contract.Hub`. Auch `local` prüft
  die Anmeldung wie jeder Transport.
- **Die Replica ist abgeleitet.** Sie enthält nur, was der Hub geliefert hat, und darf wie
  der Node-Store SQLite-Eigenes benutzen. Angelegt wird sie nur vom Abgleich, nie von `init`;
  `node hub rm` und `config import` (für weggefallene Aliase) entfernen sie mit, innerhalb der
  Transaktion, die den Eintrag entfernt. Passt ihre Schemafassung nicht, verwirft der
  Abgleich sie.
- **Hub-SQL bleibt PostgreSQL-tauglich.** Die Abfragen (DML) des Hubs und des Unterbaus
  stehen zentral in einer Struktur (`queries` bzw. `sqlitedb.Queries`), mit Platzhaltern
  `$n` über `sqlq.Bind` — kein `INSERT OR`, kein `PRAGMA`, kein `AUTOINCREMENT`, kein rohes
  `?`. Ein Test je Paket prüft sie mit `sqlq.Check`. `PRAGMA` gibt es nur beim Öffnen der
  Verbindung. Das DDL steht je Dialekt. Zähler wie die Revision werden im Code
  hochgezählt, nicht per Umwandlung in SQL. Der Node darf SQLite-Eigenes benutzen.
- **Token nie als Argument.** Ein Token kommt über `--token-stdin` (eine Zeile) herein, nie
  über ein Argument — Shell-Verlauf und Prozessliste. Angezeigt wird es nur gekürzt
  (`ident.MaskToken`); ein am Hub erzeugtes Token genau einmal, gespeichert nur als Hash.
- **Namensregeln.** Collections, Nodes und Hub-Aliase: `[a-z0-9][a-z0-9._-]{0,62}`, kein `:`,
  kein Präfix `system` in beliebiger Schreibweise — geprüft mit `ident.CheckName` bzw.
  `ident.ParseAddress`. Die Pfadregeln für Dokumentnamen stehen nur in `ident`
  (`CheckDocName`, `DocDirPrefix`); Hub-Store und Replica benutzen sie. Node-Namen sind
  gemeinsam mit Account-Namen eindeutig (jede Zeile `SYSTEM:A:<name>` in `documents` belegt
  den Namen).
- **Import prüft wie die CLI.** `config import` benutzt dieselben Prüffunktionen
  (`CheckTables` je Store) und prüft alles, bevor geschrieben wird; erst der Hub, dann der
  Node.
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
  Dependabot hält `gomod` und `github-actions` wöchentlich nach.

## Testen

- Vor jedem Commit mit Code: `make check` (gofmt, `go vet`, `go test`, `sh -n install.sh`).
- `shellcheck` über `install.sh` läuft nur in CI; lokal ist es nicht installiert.
- Tests gegen GitHub laufen über `httptest`; die Basis-URL der API ist dafür im
  `upgrade.Upgrader` überschreibbar. Sie ist keine Nutzeroption und wird nicht dokumentiert.

## Release

- Ein Release ist ein Tag `vX.Y.Z` (mit Suffix `-…` eine Vorabversion). Es gibt keine
  `VERSION`-Datei und keine versionierte `SHA256SUMS`.
- Anlegen nur über `make -C k-playbook-local release VERSION=vX.Y.Z`: verlangt sauberen
  Arbeitsbaum, Branch `main` und `HEAD == origin/main`, legt den Tag an und pusht ihn.
- Den Rest macht `.github/workflows/release.yml` (Workflow „Release“): `make check`, `make
  dist`, Release als Entwurf, Assets (vier Binaries, `SHA256SUMS`, `install.sh`), dann
  veröffentlichen. `latest` bekommt nur die höchste Version ohne Suffix.
- Scheitert ein Lauf, wird er über `workflow_dispatch` mit dem Tag wiederholt
  (`gh workflow run release.yml -f tag=vX.Y.Z`); vorhandene Assets werden ersetzt.
- Ein Release wirkt öffentlich: Tag, Push und Veröffentlichung nur nach Rückfrage beim
  Nutzer. Ein v0.x-Release gilt nicht als Veröffentlichung im Sinne der Namensprüfung
  (`docs/konzept.md`); die ist erst vor v1.0 fällig.

## Sichern

- `make -C k-playbook-local sichern [MSG=…]` committet alles und pusht ohne Prüfung —
  für Zwischenstände, nicht für Releases. Da `git add -A` alles nimmt, vorher sehen, was
  im Arbeitsbaum liegt.
