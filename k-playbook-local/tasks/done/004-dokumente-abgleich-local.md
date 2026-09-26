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
  `revision`. `put` legt an oder ersetzt — bewusst ein Admin-Upsert; der Store ist so
  geschnitten, dass „anlegen, scheitert an vorhandenem Namen“ später daneben passt.
  Unveränderter Inhalt erzeugt keine Revision. `rm`
  setzt eine Löschmarke (Inhalt NULL, `deleted = 1`, neue Revision). Nur UTF-8-Text, höchstens
  1 MiB je Dokument. `meta` bleibt NULL; Frontmatter bleibt im Text.
- **`hub import`:** liest das Verzeichnis rekursiv, Namen = relativer Pfad (mit `--prefix`),
  überspringt versteckte Dateien und Verzeichnisse (`.git` …), meldet Nicht-UTF-8 und Zu-große
  und überspringt sie. Anlegen oder Ersetzen wie `put`; im Verzeichnis Fehlendes wird **nicht**
  gelöscht (das ist später `replace_directory`). **Ein Import ist ein Schreibvorgang: eine
  Transaktion, eine Revision für alle Zeilen** — das erzwingt die Revisionsgrenze der Seiten.
  Ausgabe: angelegt, ersetzt, unverändert, übersprungen.
- **Vertrag (`docs/vertrag.md`, Fassung 1)** — nur der Abgleich:
  - Anfrage: Fassung des Nodes (in Go ein Feld, über HTTP später im Pfad; Fassung 1 ist die
    einzige), Node-Name, Node-Token, Liste (Collection, seit Revision), Seitengröße.
  - Antwort: `hub_id`, Fassung, je angefragter Collection „erlaubt“ oder „nicht erlaubt“
    (unbekannt und nicht erlaubt sind dieselbe Antwort), dazu alle dem Node erlaubten
    Collections; Zeilen (alle Spalten von `documents`, auch Löschmarken und `SYSTEM:`-Zeilen)
    mit `revision > seit` der jeweiligen Collection, sortiert nach Revision, dann `id`; die
    aktuelle Revision H des Hubs; `bis` (Revision R der Seite, bei leerer Seite = H) und
    `mehr`. Der Hub liest H zuerst und liefert nur Zeilen mit `revision ≤ H` — so passen Seite
    und H zusammen, ohne Lese-Transaktion (die wäre IMMEDIATE). Die Revision ist global: Der
    Node setzt je Collection max(seit, R), nie zurück (steht so in `vertrag.md`).
  - **Eine Seite endet an einer Revisionsgrenze:** Sie nimmt ganze Revisionen, bis die
    Seitengröße erreicht ist; eine einzelne Revision, die größer ist, kommt ganz. Bekannte
    Grenze, in `vertrag.md` benannt: Ein großer Import ist eine unbegrenzte Seite (über HTTP
    später Datenstrom oder Obergrenze je Schreibvorgang); kein Limit in diesem Task.
  - Fehler: nicht angemeldet (unbekannter Node, falsches Token, gesperrt — dieselbe Antwort),
    ungültige Anfrage, Fassung nicht unterstützt (unbekannte oder nicht unterstützte Fassung
    des Nodes). Token-Vergleich in konstanter Zeit.
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
  in einer Transaktion (Zeilen per `id` einfügen oder ersetzen, Revision je Collection auf
  max(seit, R)), dann die nächste. `hub_id` beim ersten Kontakt merken: maßgeblich ist
  `db_info.hub_id` der Replica; `hubs.hub_id` ist Kopie für Anzeige und Export und wird danach
  geschrieben. Weicht sie ab oder liegt ein `seit` über der Hub-Revision (Hub aus Sicherung mit
  gleicher `hub_id`): Replica leeren, von vorn. Die zweite Prüfung greift nur, bis der Hub wieder
  darüber hinaus geschrieben hat; `vertrag.md` benennt die Grenze: Ein aus einer Sicherung
  zurückgespielter Hub braucht eine neue `hub_id`, bis dahin verwirft der Node die Replica
  selbst (`node hub rm` + `add`). Nicht
  mehr gewünschte oder vom Hub nicht erlaubte Collections werden aus der Replica entfernt, mit
  Meldung. Nur `local` geht; andere Transporte melden „noch nicht unterstützt“, die übrigen Hubs
  laufen weiter. `local` meint den Hub der eigenen config; fehlt dort der `hub:`-Abschnitt, gibt
  es eine Fehlermeldung für diesen Hub-Eintrag, die übrigen laufen weiter.
- **Anzeigen am Node:** `node doc list|get` lesen aus der Replica, nie `SYSTEM:`-Zeilen, nie
  Löschmarken. `status` am Node: je Hub `hub_id`, je Collection Revision und letzter Abgleich.
- **Seitengröße:** Standard 500 Zeilen, für Tests einstellbar (nicht als Nutzeroption). Der Hub
  prüft sie: ≤ 0 ist „ungültige Anfrage“, nach oben begrenzt er auf eine eigene Obergrenze.
- **Keine Migrationen** bleibt: Hub-Dokumente gelten vorerst als wiederherstellbar per
  `hub import`.
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
  Collection, nicht unterstützte Fassung, Seitengröße ≤ 0 und über der Obergrenze, leere Seite
  (`bis` = Hub-Revision).

### Etappe 4 — Replica und Abgleich am Node

- Replica-Store, Abgleichlogik gegen die Schnittstelle (im Test mit einer Attrappe und mit dem
  echten Hub über local).
- Tests: Erstabgleich, Folgeabgleich nur mit Neuem, Abbruch nach Seite n und Fortsetzen,
  `hub_id`-Wechsel, revoke und nicht mehr gewünscht entfernt, Collections mit verschiedenen
  Ständen über mehrere Seiten (keine fällt zurück), `seit` über der Hub-Revision gleicht die
  ganze Replica von vorn ab, Umbenennen mit Namenswechsel (A `x`→`y`, B neu `x`, A geändert)
  scheitert nicht — nur gegen die Attrappe, der Hub kann noch nicht umbenennen.

### Etappe 5 — CLI und status am Node

- `node sync`, `node doc list|get`, `status`; Verdrahtung von `local` in `cmd/kephalaion`.
- Tests über `run()`: Hub + Node in einer config, import → sync → list/get → rm → sync.

### Etappe 6 — Doku

- `README.md`: Dokumente einspielen und abgleichen; `docs/begriffe.md`: sync, page, replica
  (genauer), vertrag; `k-playbook-local/k-playbook.md`: Vertrag, Replica; `docs/konzept.md`
  nur nachziehen, wo die Umsetzung abweicht, ausdrücklich: Rolle `replica` in `db_info`,
  Replica als Ausnahme von „nur `init` legt eine Datenbank an“, `hubs.node_name`, die
  Abgleich-Verfeinerungen aus `vertrag.md` (max(seit, R), `bis` bei leerer Seite, Grenze bei
  Wiederherstellung aus einer Sicherung).

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — listen und node_name | erledigt | 2026-09-25 | `config.Section.Listen` + `Config.Listen(r)` (Standard, wenn leer), `CheckListen` verlangt Host; Node-Schema 3 mit `hubs.node_name NOT NULL`, `--node` bei add/set; Exportformat 3 (Format 2 ohne `node_name` scheitert an der Prüfung); README-Beispiele nachgezogen |
| 2 — Dokumente am Hub | erledigt | 2026-09-26 | `ident.CheckDocName`/`DocDirPrefix`/`DocChild`; Store `PutDocument`, `Document`, `Documents`, `DeleteDocument`, `ImportDocuments` über `docTx` (eine Transaktion, `lazyRevision` = höchstens eine Revision, nur bei Änderung); actions `create`/`update`/`delete` je Dokument; kein NUL im Inhalt (PostgreSQL); Import bricht bei Namens- oder Pfadkonflikt ganz ab, übergeht versteckte Dateien still, meldet Nicht-UTF-8, >1 MiB und Nicht-Reguläre (Symlinks); Hub-Schema unverändert (2) |
| 3 — Vertrag und Hub-Seite | erledigt | 2026-09-26 | `docs/vertrag.md` (Fassung 1, nur sync); `internal/contract`: `Hub.Sync`, `SyncRequest` (Fassung und `NodeAuth` `json:"-"`, über HTTP Pfad/Header), `SyncResponse`, `Row` mit `*string` für `content`/`meta`, `Error{Code,Message}` mit `unauthenticated`/`invalid`/`unsupported_version` (Codes wie Task 005), `DefaultPageSize` 500; Hub-Seite in `internal/hub/replication` (Anmeldung mit Ersatz-Hash, H zuerst, Seitenschnitt, `MaxPageSize` 5000), Store nur `SyncRows` (eine OR-Abfrage, NULL als nil); `until` = H auf jeder letzten Seite, nicht nur der leeren; Test: `internal/contract` importiert weder Hub noch Node |
| 4 — Replica und Abgleich am Node | erledigt | 2026-09-26 | `internal/node/replica`: Replica-Store (`Open` legt nie an, `Create` samt `replicas/` 0700; `documents` ohne eindeutigen Namensindex, `sync_state`; Lesen `Document`/`Documents` ohne Löschmarken und `SYSTEM:`, bei doppeltem lebendem Namen gilt die jüngste Zeile) und `Syncer.Sync(ctx, alias, Connect)` → `[]HubResult` je Eintrag (Status je Collection `Synced`/`NotAllowed`/`NotWanted`, `Reset`-Grund, `Err`); Reset bei anderer `hub_id` oder Stand > `hub_revision` höchstens einmal je Aufruf; Replica mit fremder Schemafassung wird verworfen und neu angelegt; Schutz gegen `more` ohne Fortschritt. Node-Store: `ReplicaPath`, `SetHubID`, `RemoveHub` löscht die Replica mit. Echter Hub über local im Test in `cmd/kephalaion`; Begriff `sync_state` |
| 5 — CLI und status am Node | erledigt | 2026-09-26 | `node sync [<alias>]` (`synccmd.go`): `localConnector` öffnet den Hub der eigenen config höchstens einmal, andere Transporte „noch nicht unterstützt“, local ohne `hub:` Fehler je Eintrag; Ausgabe je Hub und Collection auf stdout, Fehler je Eintrag auf stderr, Exit 1 wenn einer scheiterte (übrige laufen weiter); `node doc list|get` aus der Replica, `printListing` auf `listedDoc` verallgemeinert; `status` am Node zeigt `hub_id` aus der Replica und je Collection Revision und letzten Abgleich (unlesbare Replica ist kein Rollenfehler); `config import` entfernt die Replicas weggefallener Aliase in der Transaktion (wie `RemoveHub`); `begriffe.md`: `node doc`, `node sync` |
| 6 — Doku | erledigt | 2026-09-26 | README: Stand, Abschnitt „Dokumente einspielen und abgleichen“ (durchgespielter Ablauf mit echten Ausgaben, nur `local`, Exit 1 bei gescheitertem Eintrag), Exportformat 3, Replica als Ausnahme von „nur `init` legt an“; `begriffe.md`: replica, sync, page, db_info (Rolle `replica`), init, hub entry, hub_id nachgeschärft; `k-playbook.md`: Pakete `contract`, `hub/replication`, `node/replica`, Regeln Vertrag, Verdrahtung in `cmd/kephalaion`, Replica abgeleitet, Pfadregeln in `ident`; `konzept.md`: Stand, Ausnahme Replica, `node_name` (Aufzählungen, DDL), `hub_id` maßgeblich in der Replica, Sicherung/Grenze, max(seit, R), `bis` = H, Rolle `replica` in `db_info`, Exportformat 3, `replicas/` im Verzeichnis von `node.db` |

---
## Review-Log (2026-09-25)

**Pfad:** k-playbook-local/tasks/004-dokumente-abgleich-local.md
**Intent:** inline (`## Intent`)
**Runden:** 2

### Diskussion
- **K8 (Hub aus Sicherung):** Runde 1 setzte „`seit` über der Hub-Revision → nur diese Collection
  von vorn“. Der Moderator wandte ein, dass die Prüfung nur greift, bis der Hub wieder über `seit`
  hinaus geschrieben hat. Der Critic bestätigte das in Runde 2: Wegen der globalen Revision stehen
  praktisch alle Collections gleich, das Rücksetzen je Collection ist also zu fein. Ergebnis: Die
  Prüfung wird wie ein `hub_id`-Wechsel behandelt (ganze Replica), die eigentliche Grenze nennt
  `vertrag.md` (Wiederherstellung braucht eine neue `hub_id`).
- **N1 (Seite und Hub-Revision aus einem Lesestand):** Der Critic schlug eine Lese-Transaktion vor.
  Der Moderator verwarf das, weil Transaktionen in `sqlitedb` IMMEDIATE sind und reine Lesezugriffe
  laut Projektregeln ohne Transaktion laufen. Stattdessen liest der Hub H zuerst und liefert nur
  Zeilen mit `revision ≤ H`; das ist gleichwertig konsistent.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| K1 | FEHLER | 004 | Vertrag, Anfrage | Keine Fassung in der Anfrage, trotzdem Fehler/Test „Fassung“ | Fassung des Nodes in die Anfrage |
| K2 | FEHLER | 004 | Abgleich am Node, „Revision auf R“ | Globale Revision: Collection mit seit > R fällt zurück | je Collection max(seit, R), Test |
| K3 | WARNUNG | 004 | Schreiben am Hub | `put` als Upsert vs. „Anlegen scheitert an vorhandenem Namen“ | als Admin-Upsert kennzeichnen |
| K4 | WARNUNG | 004 | hub import / Seiten | Großer Import = unbegrenzte Seite, über HTTP problematisch | Grenze bewusst entscheiden, dokumentieren |
| K5 | WARNUNG | 004 | Regel „Keine Migrationen“ | Erstmals Inhalte; unklar, ob Regel bleibt | ausdrücklich festhalten |
| K6 | WARNUNG | 004 | `hub_id` in hubs und db_info | Zwei Orte, keine gemeinsame Transaktion | maßgebliche Quelle, Reihenfolge |
| K7 | FEHLEND | 004 | Seitengröße | Keine Prüfung am Hub | ≤ 0 ungültig, Obergrenze |
| K8 | FEHLEND | 004 | Antwort `bis`/`mehr` | `bis` bei leerer Seite offen; Hub-Rückspielung unbemerkt | Hub-Revision mitliefern, Fall festlegen |
| K9 | FEHLEND | 004 | Etappe 4, Umbenennen | Kein Rename am Hub | nur gegen Attrappe |
| K10 | FEHLEND | 004 | Nur local | `local` ohne `hub:`-Abschnitt unklar | Fehler für diesen Eintrag |
| K11 | WARNUNG | 004 | Etappe 6 | Konzept-Abweichungen (Rolle `replica`, nur init legt an, node_name) leicht übersehen | ausdrücklich nennen |
| N1 | Korrektheit | 004 | Vertrag, Antwort | Hub-Revision und Seite aus verschiedenem Lesestand → Zeilen übersprungen | aus einem Lesestand |
| N2 | Überspezifikation | 004 | Abgleich am Node, Etappe 4 | Rücksetzen je Collection passt nicht zur globalen Revision | wie `hub_id`-Wechsel, Grenze benennen |
| N3 | Konsistenz | 004 | Etappe 6 | Abgleich-Verfeinerungen fehlen in der Abweichungsliste | aufnehmen |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| K1 | pass | Widerspruch zum Konzept (Fassung, ältere Nodes) | gefixt |
| K2 | pass | echter Fehler, Revision am Hub global | gefixt |
| K3 | decide | bewusst Admin-Upsert, Store offen für Anlegen | gefixt |
| K4 | decide | Risiko akzeptiert, in `vertrag.md` benannt | gefixt |
| K5 | decide | Regel bleibt, Wiederherstellung per `hub import` | gefixt |
| K6 | decide | `db_info.hub_id` maßgeblich, `hubs.hub_id` Kopie, danach geschrieben | gefixt |
| K7 | pass | billig, betrifft HTTP-Tauglichkeit | gefixt |
| K8 | decide | Hub-Revision in der Antwort, `bis` bei leerer Seite | gefixt, in Runde 2 korrigiert |
| K9 | pass | Test sonst nicht ausführbar | gefixt |
| K10 | pass | Verhalten sonst offen | gefixt |
| K11 | pass | Abweichungen sonst übersehen | gefixt |
| N1 | decide | ohne Lese-Transaktion: H zuerst, Zeilen ≤ H | vom Moderator gefixt |
| N2 | pass | Moderator-Einwand bestätigt | vom Moderator gefixt |
| N3 | pass | Folge von K2/K8 | vom Moderator gefixt |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| K1 | fixed | Fassung in der Anfrage (Go-Feld, HTTP im Pfad), Fehler präzisiert, Test |
| K2 | fixed | max(seit, R) je Collection, in `vertrag.md`, Test in Etappe 4 |
| K3 | fixed | ein Satz: Admin-Upsert, Store offen |
| K4 | fixed | bekannte Grenze, kein Limit |
| K5 | fixed | Vermerk im Kontext |
| K6 | fixed | maßgebliche Quelle und Reihenfolge |
| K7 | fixed | Prüfung am Hub, Test in Etappe 3 |
| K8 | fixed | Hub-Revision, `bis` bei leerer Seite, Rücksetzen, Tests |
| K9 | fixed | nur gegen Attrappe |
| K10 | fixed | Fehlermeldung je Hub-Eintrag |
| K11 | fixed | drei Abweichungen in Etappe 6 |

### Moderator-Entscheidungen
- K3–K6, K8: Designfragen vom Moderator entschieden (siehe Routing), keine Nutzerfrage nötig.
- N1–N3: Runde 2 ohne erneuten Editor umgesetzt, weil die Änderungen klein und eindeutig waren.
  N1 abweichend vom Critic-Vorschlag gelöst (keine Lese-Transaktion, siehe Diskussion).
- Keine Editor-Vorschläge verworfen.

### Intent-Alignment
Ja — jeder Punkt des Intents ist im Kontext festgelegt und hat eine Etappe mit Tests (Vertrag im
neutralen Paket ohne Hub-Import im Node, Seiten an Revisionsgrenzen, `hub_id` und Entfernen nicht
erlaubter Collections, Anmeldeprüfung auch über local, `listen` ohne `serve`).

### Geänderte Dateien
- 004-dokumente-abgleich-local.md: Fassung in der Anfrage (K1); max(seit, R) je Collection (K2);
  Admin-Upsert (K3); Grenze großer Import (K4); Keine Migrationen bleibt (K5); maßgebliche `hub_id`
  (K6); Prüfung der Seitengröße (K7); Hub-Revision H, `bis` bei leerer Seite, Zeilen ≤ H (K8, N1);
  Rücksetzen der ganzen Replica bei `seit` > H, Grenze bei Wiederherstellung (K8, N2); Rename-Test
  nur gegen Attrappe (K9); local ohne `hub:` (K10); Abweichungsliste in Etappe 6 (K11, N3);
  Tests in Etappe 3 und 4 ergänzt.

### Offen (nicht gefixt)
- —

## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** 2026-09-26  
**Zusammenfassung:** Der Hub nimmt Dokumente auf (`hub doc put|get|list|rm`, `hub import`, eine Transaktion und höchstens eine Revision je Schreibvorgang), `listen` steht in der config, der Node meldet sich mit `hubs.node_name` an. Der Abgleich-Vertrag (Fassung 1) steht in `docs/vertrag.md` und im neutralen `internal/contract`, umgesetzt am Hub in `internal/hub/replication` (Seiten an Revisionsgrenzen, H zuerst, Anmeldung in konstanter Zeit); der Node gleicht über `transport local` in `replicas/<alias>.db` ab (`internal/node/replica`, `node sync`, `node doc list|get`, `status`). Doku (README, begriffe, konzept, k-playbook.md) nachgezogen; Commits 7490df8, d469012, 442bacc, ae1b4b7, b30fc00, cb0f731, ffbac30 (Hinweis von `hub node add` nennt `--node`).

**Hinweis:** Während Etappe 1 hat eine parallele Sitzung (Task 005) mit Commit f3ba8c4 einen Zwischenstand von Etappe 1 mitgenommen; erst 7490df8 macht den Stand wieder grün. Der Diff unten ist ohne die Dateien von Task 005 und `docs/fortschritt.md`.

**Geänderte Dateien:**
```
 README.md                                    | 139 +++++-
 cmd/kephalaion/admin_test.go                 |  35 +-
 cmd/kephalaion/command.go                    |  42 +-
 cmd/kephalaion/configcmd.go                  |   3 +-
 cmd/kephalaion/configcmd_test.go             |  14 +-
 cmd/kephalaion/configimport_test.go          |  44 +-
 cmd/kephalaion/doccmd.go                     | 305 +++++++++++++
 cmd/kephalaion/doccmd_test.go                | 122 +++++
 cmd/kephalaion/exportfile.go                 |  11 +-
 cmd/kephalaion/hubcmd.go                     |   4 +-
 cmd/kephalaion/main.go                       |  12 +-
 cmd/kephalaion/nodecmd.go                    |  35 +-
 cmd/kephalaion/replica_local_test.go         | 148 +++++++
 cmd/kephalaion/roles.go                      | 123 +++++-
 cmd/kephalaion/roles_test.go                 |  35 ++
 cmd/kephalaion/synccmd.go                    | 279 ++++++++++++
 cmd/kephalaion/synccmd_test.go               | 161 +++++++
 docs/begriffe.md                             |  57 ++-
 docs/konzept.md                              |  64 ++-
 docs/vertrag.md                              | 127 ++++++
 internal/config/config.go                    |  72 ++-
 internal/config/config_test.go               |  59 ++-
 internal/contract/contract.go                | 154 +++++++
 internal/contract/contract_test.go           |  77 ++++
 internal/hub/replication/replication.go      | 176 ++++++++
 internal/hub/replication/replication_test.go | 405 +++++++++++++++++
 internal/hub/store/documents.go              | 416 +++++++++++++++++
 internal/hub/store/documents_test.go         | 366 +++++++++++++++
 internal/hub/store/store.go                  |  89 +++-
 internal/hub/store/store_test.go             |   4 +-
 internal/hub/store/sync.go                   |  77 ++++
 internal/hub/store/sync_test.go              |  94 ++++
 internal/ident/ident.go                      | 105 ++++-
 internal/ident/ident_test.go                 |  80 ++++
 internal/node/replica/fake_test.go           | 150 +++++++
 internal/node/replica/replica.go             | 396 +++++++++++++++++
 internal/node/replica/sync.go                | 398 +++++++++++++++++
 internal/node/replica/sync_test.go           | 639 +++++++++++++++++++++++++++
 internal/node/store/hubs.go                  |  92 +++-
 internal/node/store/hubs_test.go             | 183 +++++++-
 internal/node/store/store.go                 |  42 +-
 internal/node/store/store_test.go            |   6 +-
 internal/separation_test.go                  |  16 +
 k-playbook-local/k-playbook.md               |  33 +-
 44 files changed, 5704 insertions(+), 185 deletions(-)
```

**Code-Änderungen:** (Diff ~6500 Zeilen, hier zusammengefasst)
- `internal/config`: `Section.Listen`, `DefaultListen`, `Config.Listen(r)`, `CheckListen` (Host verlangt); `init --listen`, `status` zeigt ihn.
- `internal/node/store`: Schema 3 mit `hubs.node_name NOT NULL`, `ReplicaPath`, `SetHubID`; `RemoveHub` und `config import` löschen Replicas in der Transaktion. Exportformat 3.
- `internal/ident`: `CheckDocName` (Pfadregeln, `SYSTEM:` gesperrt), `DocDirPrefix`, `DocAncestors`, `DocChild`.
- `internal/hub/store/documents.go`: `PutDocument`, `Document`, `Documents`, `DeleteDocument`, `ImportDocuments` über `docTx`/`lazyRevision`; `sync.go`: `SyncRows` (eine OR-Abfrage je Collection, ≤ H, NULL als nil).
- `internal/contract`: `Hub.Sync`, `SyncRequest`/`SyncResponse`/`Row`, Fehlercodes; `internal/separation_test.go` prüft seine Neutralität.
- `internal/hub/replication`: Anmeldung, erlaubte Collections, Seitenschnitt. Kern:

```go
	rows, err = h.st.SyncRows(ctx, since, hubRev, pageSize+1)
	...
	if len(rows) <= pageSize {
		return rows, hubRev, false, nil
	}
	last := rows[pageSize-1].Revision
	if rows[pageSize].Revision != last {
		return rows[:pageSize], last, true, nil
	}
	cut := pageSize
	for cut > 0 && rows[cut-1].Revision == last {
		cut--
	}
	if cut > 0 {
		return rows[:cut], rows[cut-1].Revision, true, nil
	}
	// Die ganze Seite ist eine Revision, größer als die Seite: sie kommt ganz.
	rows, err = h.st.SyncRows(ctx, raise(since, last-1), last, 0)
```
- `internal/node/replica`: Replica-Store (`Open` legt nie an, `Create` mit `replicas/` 0700, kein eindeutiger Namensindex, `sync_state`) und `Syncer.Sync` (Seite je Transaktion, max(seit, until), Reset bei anderer `hub_id` oder seit > H, Entfernen nicht erlaubter/nicht gewünschter Collections).
- `cmd/kephalaion`: `doccmd.go` (hub doc, hub import), `synccmd.go` (`node sync`, `node doc`, `localConnector`), `roles.go` (status), Tests über `run()` und mit dem echten Hub über local.

**Code-Review:** (engineering:code-review, nur auf dem Diff) — Urteil: Approve. Nichts blockiert den heutigen SQLite-Stand.
- Mittel 1: Verzeichnisbereich `name >= 'x/' AND name < 'x0'` setzt Byte-Sortierung voraus; unter PostgreSQL mit Locale-Collation falsch → `COLLATE "C"` oder Präfixvergleich, vor PostgreSQL.
- Mittel 2: `liveDocument`/`checkPathFree` lesen vor der Sperre der Revision; unter PostgreSQL (READ COMMITTED) können `put a` und `put a/b` parallel durchkommen → `LockRevision` am Anfang von `writeDocs`.
- Mittel 3: Zwei gleichzeitige `node sync` desselben Eintrags: B kann nach einem Reset von A einen hohen Stand in die leere Replica schreiben → Sperre je Replica oder hub_id/Stand in `apply` neu lesen, vor Cron/`serve`.
- Vorschläge: Obergrenze je Import (Seite und Speicher unbegrenzt, bekannte Grenze); Symlink als Import-Wurzel meldet still „0 angelegt“; `ident` lässt Cf-Zeichen (Bidi, U+200B) durch, keine NFC-Normalisierung (macOS-Import); `checkResponse` am Node könnte `Until ≤ HubRevision` und Zeilengrenzen prüfen; `invalid` vor `unauthenticated` vor HTTP entscheiden; Tests: große Revision mit anderer Collection `since > last` am echten Hub, `a` und `a/b` im selben Import, zwei Syncer auf einer Replica.


**Intent-Alignment:** Ja - Vertrag in `docs/vertrag.md` und neutralem `internal/contract` (Neutralität per Test), Seiten an Revisionsgrenzen, `hub_id`-Prüfung mit Neuabgleich, Entfernen nicht erlaubter bzw. nicht gewünschter Collections, Anmeldeprüfung (Name, Token, Sperre — gesperrt getestet in `replica_local_test.go` und `TestNodeSyncPerEntryErrors`) auch über local, `listen` in der config ohne `serve`.
