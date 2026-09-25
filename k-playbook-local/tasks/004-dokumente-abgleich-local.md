# Task 004 — Dokumente im Hub und Abgleich zum Node über local

Der Hub nimmt Dokumente auf (CLI), der Node gleicht sie über `transport local` in seine
Replica ab und zeigt sie an. Der Vertrag für den Abgleich ist geschrieben und getestet.

## Intent

Zum ersten Mal fließen Inhalte vom Hub zum Node — über eine harte Schnittstelle, die später
unverändert auch über HTTP läuft.
- Der Vertrag für den Abgleich steht in `docs/vertrag.md` und als Go-Typen in einem neutralen
  Paket; der Node kennt nur die Schnittstelle, nicht den Hub.
- Eine Seite endet an einer Revisionsgrenze: Ein Schreibvorgang mit vielen Zeilen kommt immer
  ganz oder gar nicht an.
- Der Node prüft die `hub_id` und gleicht von vorn ab, wenn sie sich ändert; Collections, die
  der Hub nicht (mehr) erlaubt oder der Node nicht mehr will, verschwinden aus der Replica.
- Der Hub prüft den Node bei jedem Aufruf (Name, Token, Sperre), auch auf dem lokalen Weg.
- `listen` steht in der config; `serve` gibt es noch nicht.

## Referenzen

- `docs/konzept.md` — „Ein Programm, zwei Rollen“ (config mit `listen`, local prüft genauso),
  „Abgleich“ (Revision, Seiten, `hub_id`), „Datenmodell“ (documents, Name als Pfad, auf dem
  Node zählt die `id`, lokale Tabellen), „Speicherung“ (Orte, `replicas/`),
  „Zwei Arten von Eingaben“ (Anlegen scheitert an vorhandenem Namen), „Authentifizierung“.
- `docs/begriffe.md`.
- `k-playbook-local/tasks/done/003-collections-nodes-hubs.md` — Stand davor.
- `internal/hub/store`, `internal/node/store`, `internal/ident`, `internal/sqlitedb`,
  `cmd/kephalaion`.

## Ziel

```text
config:  hub: listen / node: listen; init --listen
Hub:
  kephalaion hub doc put  <collection> <name> [--file pfad]   (sonst stdin)
  kephalaion hub doc get  <collection> <name>
  kephalaion hub doc list <collection> [pfad]
  kephalaion hub doc rm   <collection> <name>                 → Löschmarke
  kephalaion hub import   <collection> <verzeichnis> [--prefix pfad/]
Node:
  kephalaion node hub add … --node <name am hub>              (neu, Pflicht)
  kephalaion node sync [<alias>]
  kephalaion node doc list <hub>:<collection> [pfad]
  kephalaion node doc get  <hub>:<collection> <name>
```

## Kontext

- **`listen` in der config** je Rolle; Standard Node `127.0.0.1:7433`, Hub
  `127.0.0.1:7434`; `init` schreibt ihn, `--listen host:port` weicht ab; `status` zeigt ihn.
  Fehlt er in einer bestehenden config, gilt der Standard. Noch lauscht nichts.
- **Name als Pfad** nach „Datenmodell“: Prüfung in `internal/ident`; der Hub prüft zusätzlich
  „nicht zugleich Datei und Verzeichnis“. `SYSTEM:`-Namen kann die CLI nicht schreiben.
- **Schreiben am Hub:** je Vorgang eine Transaktion, eine Revision (`nextRevision`), neue `id`
  als ULID, `created_by`/`updated_by` = `admin`, Zeile in `actions` mit `document_id` und
  `revision`. `put` legt an oder ersetzt; unveränderter Inhalt erzeugt keine Revision. `rm`
  setzt eine Löschmarke (Inhalt NULL, `deleted = 1`, neue Revision). Nur UTF-8-Text, höchstens
  1 MiB je Dokument. `meta` bleibt NULL; Frontmatter bleibt im Text.
- **`hub import`:** liest das Verzeichnis rekursiv, Namen = relativer Pfad (mit `--prefix`),
  überspringt versteckte Dateien und Verzeichnisse (`.git` …), meldet Nicht-UTF-8 und Zu-große
  und überspringt sie. Anlegen oder Ersetzen wie `put`; im Verzeichnis Fehlendes wird **nicht**
  gelöscht (das ist später `replace_directory`). **Ein Import ist ein Schreibvorgang: eine
  Transaktion, eine Revision für alle Zeilen** — das erzwingt die Revisionsgrenze der Seiten.
  Ausgabe: angelegt, ersetzt, unverändert, übersprungen.
- **Vertrag (`docs/vertrag.md`, Fassung 1)** — nur der Abgleich:
  - Anfrage: Node-Name, Node-Token, Liste (Collection, seit Revision), Seitengröße.
  - Antwort: `hub_id`, Fassung, je angefragter Collection „erlaubt“ oder „nicht erlaubt“
    (unbekannt und nicht erlaubt sind dieselbe Antwort), dazu alle dem Node erlaubten
    Collections; Zeilen (alle Spalten von `documents`, auch Löschmarken und `SYSTEM:`-Zeilen)
    mit `revision > seit` der jeweiligen Collection, sortiert nach Revision, dann `id`; `bis`
    (Revision R, auf die der Node setzt) und `mehr`.
  - **Eine Seite endet an einer Revisionsgrenze:** Sie nimmt ganze Revisionen, bis die
    Seitengröße erreicht ist; eine einzelne Revision, die größer ist, kommt ganz.
  - Fehler: nicht angemeldet (unbekannter Node, falsches Token, gesperrt — dieselbe Antwort),
    ungültige Anfrage, Fassung nicht unterstützt. Token-Vergleich in konstanter Zeit.
  - Go: neutrales Paket (etwa `internal/contract`) mit Typen und Schnittstelle `Hub`; der Hub
    setzt sie um, der Node benutzt sie. Die Verdrahtung für `local` geschieht in
    `cmd/kephalaion` — `internal/node` importiert nichts aus `internal/hub`.
- **Node — Anmeldung:** `hubs` bekommt `node_name` (Schemafassung 3 für den Node, ohne
  Migration); `node hub add` verlangt `--node`, `node hub set` kann es ändern.
- **Replica:** eine SQLite-Datei je Hub-Eintrag unter `replicas/<alias>.db` neben `node.db`,
  Verzeichnis `0700`. Tabellen: `db_info` (Rolle `replica`, `hub_id`, Fassung),
  `documents` wie am Hub, **ohne eindeutigen Index auf den Namen**, `sync_state (collection,
  revision, synced_at)`. Sie wird vom ersten `sync` angelegt — die einzige Datenbank, die
  nicht `init` anlegt, weil sie abgeleitet ist. `node hub rm` löscht sie mit.
- **Abgleich am Node:** fragt je gewünschter Collection ab ihrer Revision (neu: 0); jede Seite
  in einer Transaktion (Zeilen per `id` einfügen oder ersetzen, Revision auf R), dann die
  nächste. `hub_id` beim ersten Kontakt in `hubs` und `db_info` merken; weicht sie ab: Replica
  leeren, von vorn. Nicht mehr gewünschte oder vom Hub nicht erlaubte Collections werden aus
  der Replica entfernt, mit Meldung. Nur `local` geht; andere Transporte melden „noch nicht
  unterstützt“, die übrigen Hubs laufen weiter.
- **Anzeigen am Node:** `node doc list|get` lesen aus der Replica, nie `SYSTEM:`-Zeilen, nie
  Löschmarken. `status` am Node: je Hub `hub_id`, je Collection Revision und letzter Abgleich.
- **Seitengröße:** Standard 500 Zeilen, für Tests einstellbar (nicht als Nutzeroption).
- **Nicht in diesem Task:** `serve`, HTTP, Accounts, Zerlegung und Suche (FTS5),
  `create_numbered`, Schreiben über den Node.

## Zu bauen

### Etappe 1 — listen und node_name

- config: `listen` je Rolle, Standard, `init --listen`, `status`.
- Node-Schema 3: `hubs.node_name`; `node hub add|set --node`; Export/Import nehmen es mit.
- Tests.

### Etappe 2 — Dokumente am Hub

- `internal/ident`: Pfadregeln. Hub-Store: put, get, list (Verzeichnis, sortiert nach Name),
  rm, import in einer Transaktion. CLI `hub doc …`, `hub import`.
- Tests: Pfadregeln, Datei/Verzeichnis-Konflikt, unveränderter Inhalt ohne Revision,
  Löschmarke und Neuanlage desselben Namens, Import mit einer Revision, übersprungene Dateien.

### Etappe 3 — Vertrag und Hub-Seite

- `docs/vertrag.md`, neutrales Paket, Umsetzung am Hub (Anmeldung, erlaubte Collections,
  Seiten an Revisionsgrenzen).
- Tests gegen die Schnittstelle: Seitengrenze mitten in einer großen Revision, mehrere
  Revisionen über mehrere Seiten, falsches Token/gesperrt/unbekannt gleich, nicht erlaubte
  Collection, Fassung.

### Etappe 4 — Replica und Abgleich am Node

- Replica-Store, Abgleichlogik gegen die Schnittstelle (im Test mit einer Attrappe und mit dem
  echten Hub über local).
- Tests: Erstabgleich, Folgeabgleich nur mit Neuem, Abbruch nach Seite n und Fortsetzen,
  `hub_id`-Wechsel, revoke und nicht mehr gewünscht entfernt, Umbenennen mit Namenswechsel
  (A `x`→`y`, B neu `x`, A geändert) scheitert nicht.

### Etappe 5 — CLI und status am Node

- `node sync`, `node doc list|get`, `status`; Verdrahtung von `local` in `cmd/kephalaion`.
- Tests über `run()`: Hub + Node in einer config, import → sync → list/get → rm → sync.

### Etappe 6 — Doku

- `README.md`: Dokumente einspielen und abgleichen; `docs/begriffe.md`: sync, page, replica
  (genauer), vertrag; `k-playbook-local/k-playbook.md`: Vertrag, Replica; `docs/konzept.md`
  nur nachziehen, wo die Umsetzung abweicht.
