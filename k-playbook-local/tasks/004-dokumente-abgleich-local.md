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
