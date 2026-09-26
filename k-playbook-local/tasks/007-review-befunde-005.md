# Task 007 — Review-Befunde aus Task 005 beheben

Die Befunde aus dem Code-Review von Task 005 werden behoben: `rotate` kann keinen Account
mehr aussperren, der Hub-Store bleibt unter PostgreSQL korrekt, der HTTP-Weg ist gehärtet,
der Import leert nicht versehentlich.

## Intent

Der Unterbau aus Task 005 trägt verlässlich, bevor Suche, Schreiben und Abgleich im
Hintergrund darauf aufsetzen.
- Ein `rotate` endet nie als „eindeutig gescheitert“, wenn der Hub den neuen Hash schon
  gespeichert haben kann — über `local` genauso wie über `http`.
- `rotate`, die übrigen Änderungen an einem Account und die gemeinsame Eindeutigkeit von
  Node- und Account-Namen sind auch ohne die Schreibsperre von SQLite korrekt (PostgreSQL,
  READ COMMITTED).
- Kein Token verlässt den Node über eine Weiterleitung; Hub und Node prüfen `Host` gleich.
- Ein Import leert Accounts nur bei einem ausdrücklich leeren Teil, nie bei `accounts:` ohne Wert.
- Jeder Befund hat einen Test, der vorher scheitert — außer wo SQLite den Fehler nicht
  zeigen kann oder der Code ihn schon abfängt (jeweils bei den Tests vermerkt).

## Referenzen

- `k-playbook-local/tasks/done/005-kommunikation-http-mcp.md` — Abschnitt „Ausführung“,
  Code-Review (Befunde und Vorschläge).
- `docs/vertrag.md` — `rotate`, `ErrOutcomeUnknown`, Fehlercodes.
- `docs/konzept.md` — „Authentifizierung“ (`rotate` Schritt für Schritt), „Lokale Tabellen“/Export,
  Abgleich (Revision als gesperrte Zeile, `LockRevision`).
- `k-playbook-local/k-playbook.md` — Hub-SQL PostgreSQL-tauglich, Import prüft wie die CLI.

## Kontext

- **Aussperrung (Befund 1):** `hub/replication.Rotate` liest `h.st.Info(ctx)` erst nach dem
  Commit von `RotateAccount`. Scheitert das, kommt ein gewöhnlicher Fehler zurück. Über HTTP
  wird daraus 500 → `ErrOutcomeUnknown` (richtig), über `local` behandelt
  `cmd/kephalaion/nodeaccountcmd.go` ihn als eindeutiges Scheitern: `.pending` gelöscht bzw.
  „altes Token gilt weiter“ — beides falsch.
- **Race bei `rotate` (Befund 2):** `RotateAccount` prüft per SELECT und schreibt per
  `UPDATE accounts SET token_hash = $2 WHERE name = $1` ohne Bedingung.
- **Race zwischen Account-Änderungen:** Alle laufen über `writeAccount`
  (`internal/hub/store/accounts.go`). Sie lesen die Zeile in `accounts` ungesperrt; die
  Revision (`LockRevision`) sperrt erst beim ersten Schreiben einer Zeile. Unter READ
  COMMITTED kann so `rotate` zwischen `grant`/`revoke`/`lock`/`unlock` fallen, und die
  schreiben die Zeilen danach mit dem alten Hash — der Node lehnt das neue Token ab.
  `SELECT … FOR UPDATE` versteht SQLite nicht (`sqlq.Check` verbietet es nicht, es scheitert
  am Dialekt).
- **Namenseindeutigkeit (Befund 3):** Node und Account prüfen den Namen nur per Zählabfrage
  über die jeweils andere Tabelle, ohne Constraint.
- **Weiterleitungen:** Der `http.Client` in `internal/contract/httpapi/client.go` folgt Redirects;
  Go streicht bei fremdem Host den `Authorization`-Header, nicht aber den Body mit dem
  Account-Token von `rotate`.
- **Host am Hub:** Der Node prüft `Host` (Loopback, eigener Port) vor `/mcp`, der Hub-Listener
  unter `/v1/` nicht.
- **`accounts:` ohne Wert:** In Format 4 lehnt `requirePart` (`cmd/kephalaion/exportfile.go`)
  null schon ab („tables.hub.accounts ist null“) — die Aussage im Review trifft nicht zu, es
  fehlt nur der Test. In Format 3 wird ein Accounts-Teil nur als Liste erkannt: `[]` wird
  abgelehnt, null decodiert zu nil und wird still angenommen (die Accounts bleiben stehen).
- **Collections entfernen:** `collectionRemovable` zählt auch Löschmarken; `SYSTEM:A:`-Zeilen
  werden nie entfernt. Eine Collection, in der je ein Account Rechte hatte, lässt sich deshalb
  nie mehr entfernen, auch nach `revoke` oder `rm`. Die Löschmarken physisch zu entfernen geht
  nicht: Ein Node verwirft eine Collection nur, wenn ein Abgleich sie als nicht erlaubt meldet.
  War er offline, während sie entfernt, gleichnamig neu angelegt und wieder erlaubt wurde,
  gleicht er mit altem `seit` weiter ab — ohne Löschmarke bliebe seine lebende
  `SYSTEM:A:`-Zeile mit altem Hash und alten Rechten stehen. `documents` hat keinen
  Fremdschlüssel auf `collections`; die Marken können stehen bleiben.
- **Nicht in diesem Task:** die übrigen Vorschläge aus dem Review — Laufzeitunterschied
  bekannt/unbekannt, 4xx ohne Vertragsform bei `rotate` als unklar, Body-Limit/ReadTimeout
  für `/mcp`, Origin mit fremdem Port, Einrichtungstoken vor dem ersten `rotate`, Hilfetexte
  zu `lock`/`rotate`, Windows-Build. Ebenso das PostgreSQL-Backend selbst.

## Zu bauen

### Etappe 1 — rotate: kein eindeutiges Scheitern nach dem Commit

- `Rotate` am Hub liest alles, was die Antwort braucht (`hub_id`), **vor** `RotateAccount`;
  nach dem Commit wird nur noch aus dem Speicher zusammengestellt.
- Der `local`-Connector meldet jeden Fehler von `Rotate`, der kein Vertragsfehler
  (`contract`-Code) ist, als `ErrOutcomeUnknown`.
- Tests: Fehler nach dem Commit über `local` → `.pending` bleibt, Hinweis auf
  `node account check`; mit `--token-stdin` wird das neue Token als „unklar — prüfen“
  ausgegeben; `node account check` löst den Fall danach auf.

### Etappe 2 — Hub-Store ohne Race

- Grundsatz: Jede Transaktion, die einen Account ändert, sperrt zuerst seine Zeile in
  `accounts` und liest erst danach — als erste Anweisung in `writeAccount` ein `UPDATE` auf
  die Zeile (wie `LockRevision`, z. B. `UPDATE accounts SET name = name WHERE name = $1`),
  kein `SELECT … FOR UPDATE`. Der Import sperrt vor dem Lesen alle Zeilen von `accounts`.
- `RotateAccount` beginnt mit dem bedingten Schreiben
  (`… WHERE name = $1 AND token_hash = $3 AND locked = 0`) — das ist zugleich die Sperre —,
  prüft die Zahl der geänderten Zeilen und liefert bei 0 `account_unauthenticated` ohne
  weitere Änderung. Zeilen und Rechte liest es erst danach.
- Gemeinsame Eindeutigkeit von Node- und Account-Namen per Datenbank absichern, PostgreSQL-
  tauglich (z. B. eine gemeinsame Tabelle der belegten Namen mit Primärschlüssel, in derselben
  Transaktion wie Anlegen/Entfernen). Schema-Fassung des Hubs anheben.
- Die Tabelle der belegten Namen folgt aus `accounts` und `nodes`: nicht im Export (Format
  bleibt 4), `config import` baut sie in derselben Transaktion neu auf.
- Die Vorprüfung per Zählabfrage bleibt für die lesbare Meldung; die Constraint ist die
  letzte Wache, ihre Verletzung ergibt denselben Fehler (`ErrExists`), keine 500.
- Tests: zweites `rotate` mit demselben alten Token scheitert auch, wenn die Vorprüfung
  umgangen wird; `rotate` auf einen gesperrten Account scheitert am bedingten Schreiben;
  `rotate` verschränkt mit `grant` bzw. `lock` — unter SQLite reiht `BEGIN IMMEDIATE` die
  Transaktionen, der Test belegt nur, dass danach Hash in `accounts` und Zeilen
  übereinstimmen und die Sperre die erste Anweisung in `writeAccount` ist; vorher scheitert
  er unter SQLite nicht. Anlegen eines Namens, der in der anderen Tabelle belegt ist,
  scheitert an der Datenbank (Vorprüfung umgangen) mit derselben Meldung wie mit
  Vorprüfung; Name nach `rm` wieder frei; Import mit Namenskonflikt (Node und Account
  gleichnamig) → Abbruch ohne Änderung.

### Etappe 3 — HTTP-Weg härten

- Der Client folgt keinen Weiterleitungen (`CheckRedirect` → `http.ErrUseLastResponse`);
  eine 3xx-Antwort ist ein Fehler, bei `rotate` ein eindeutiger (der Hub hat nicht ausgeführt).
- Der Hub-Listener prüft `Host` wie der Node (Loopback mit eigenem Port), sonst 403;
  gemeinsame Prüfung, nicht doppelt. Ein Tunnel geht damit nur mit gleichem Port
  (`ssh -L 8080:localhost:8080`), bis `ssh`/`https` kommen.
- Tests: Redirect-Ziel bekommt keinen Body, `rotate` hinter 307 scheitert eindeutig; fremder
  `Host` am Hub → 403.

### Etappe 4 — Import und Collections

- Import: Format 3 lehnt einen Accounts-Teil in jeder Form ab (null wie `[]`), geprüft am
  YAML-Knoten, nicht an der decodierten Liste; Format 4 lehnt null ab (schon heute), nur
  `accounts: []` leert.
- Collections entfernen: Löschmarken von `SYSTEM:A:`-Zeilen blockieren das Entfernen nicht
  und bleiben stehen (kein Fremdschlüssel); wird die Collection gleichnamig neu angelegt,
  belebt ein `grant` sie wieder. Lebende Zeilen und gemerkte Rechte eines gesperrten
  Accounts blockieren weiter. Dasselbe gilt für `config import` (`collectionRemovable` im
  Import): Entfällt eine Collection, bleiben ihre Marken stehen.
- Tests: null-Teil in beiden Formaten → Abbruch ohne Änderung (Format 4 scheitert vorher
  nicht, sichert ab); Collection nach `revoke` bzw. `rm` entfernbar, ihre Löschmarken
  bleiben; mit lebender Zeile oder gemerkten Rechten nicht; Node offline während
  `revoke` → entfernen → neu anlegen → `grant`: der nächste Abgleich macht seine alte Zeile
  zur Löschmarke bzw. ersetzt sie; Import ohne eine Collection, in der nur Löschmarken
  stehen → Collection entfernt, Marken bleiben.

### Etappe 5 — Doku

- `docs/vertrag.md` (Weiterleitungen, `rotate` bei Fehler nach dem Commit, 403 am Hub,
  Tunnel nur mit gleichem Port), `docs/konzept.md` (Sperre der Account-Zeile zuerst,
  Eindeutigkeit per Datenbank, Collections mit Löschmarken entfernen — Marken bleiben,
  Import mit null), `k-playbook-local/k-playbook.md` (Namenseindeutigkeit; „Accounts am
  Hub“: Zeile in `accounts` zuerst sperren, Löschmarken bleiben auch beim Entfernen der
  Collection), `docs/begriffe.md` falls neue Begriffe.

---
## Review-Log (2026-09-26)

**Pfad:** k-playbook-local/tasks (gemeinsam mit 006)
**Intent:** inline
**Runden:** 2

### Diskussion
- **4 (Löschmarken beim Entfernen einer Collection):** Critic sah im physischen Entfernen einen
  Widerspruch zur Regel „Zeilen werden nie entfernt“ und verlangte eine Begründung. Der Editor
  zeigte, dass die vorgeschlagene Begründung nicht trägt: Ein Node, der offline war, während die
  Collection entfernt, gleichnamig neu angelegt und wieder erlaubt wurde, behielte seine alte
  lebende `SYSTEM:A:`-Zeile. Lösung: Marken blockieren das Entfernen nicht und bleiben stehen;
  die Regel bleibt. Critic hat in Runde 2 zugestimmt und am Code bestätigt.
- **6 (null im Import):** Der Editor stellte am Code fest, dass Format 4 null schon ablehnt
  (`requirePart`); der Befund aus dem Review von 005 trifft nur Format 3. Kontext und Intent
  (Test, der vorher scheitert) wurden entsprechend angepasst; Critic einverstanden.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| 1 | FEHLER | 007, 006 | Etappe 2 | Bedingtes UPDATE schützt nur `rotate` gegen `rotate`; unter READ COMMITTED überholt `rotate` `grant`/`revoke`/`lock`/`unlock` (ab 006 `set --user`) → Zeilen mit altem Hash, Aussperrung | Account-Zeile zuerst sperren, dann lesen; `rotate` prüft auch „nicht gesperrt“; Test |
| 2 | FEHLEND | 007 | Etappe 2, Tabelle der belegten Namen | Import ersetzt `accounts`/`nodes` vollständig; Export der Namenstabelle offen, Format-Abhängigkeit zu 006 | Abgeleitet, nicht exportiert, Import baut neu auf; Format bleibt 4 |
| 3 | WARNUNG | 007 | Etappe 2 | Unique-Verstoß treiberabhängig, lesbare Meldung könnte verloren gehen | Vorprüfung bleibt, Constraint als letzte Wache mit demselben Fehler |
| 4 | WARNUNG | 007 | Etappe 4/5 | Physisches Entfernen von Löschmarken widerspricht Regel und Konzept | Begründen, Regeln anpassen |
| 5 | WARNUNG | 007 | Etappe 3 | `Host` mit eigenem Port lehnt Tunnel mit anderem Port ab | Entscheiden und dokumentieren |
| 6 | WARNUNG | 007 | Etappe 4 | „nur `[]` leert“ für Format 3 missverständlich | Format 3 lehnt jede Form ab, Format 4 nur null |
| N1 | FEHLEND | 007 | Etappe 4 | `collectionRemovable` läuft auch im Import; Regel dort offen | Gleiche Regel für den Import, Test |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| 1 | pass | Trifft genau das Intent (PostgreSQL, keine Aussperrung) | gefixt |
| 2 | pass | Blockiert: Import und Folgetask 006 hängen davon ab | gefixt |
| 3 | decide | Vorprüfung bleibt, Constraint bildet auf denselben Fehler ab | gefixt |
| 4 | pass | Widerspruch zu Projektregel | anders gelöst (Marken bleiben) |
| 5 | decide | Wie Node prüfen; Tunnel nur mit gleichem Port bis `ssh`/`https` | gefixt |
| 6 | pass | Billige Klarstellung, am Code prüfen | gefixt, Kontext korrigiert |
| N1 | decide | Kleine Ergänzung, vom Moderator direkt eingearbeitet | gefixt |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| 1 | fixed | Grundsatz in Etappe 2: erste Anweisung in `writeAccount` sperrt per `UPDATE` (wie `LockRevision`, kein `FOR UPDATE` — SQLite); `rotate` bedingt inkl. `locked = 0`; Import sperrt alle Zeilen; Test mit Vermerk, dass SQLite die Race nicht zeigt |
| 2 | fixed | Namenstabelle abgeleitet, nicht im Export, Import baut neu auf; Test Namenskonflikt |
| 3 | fixed | Verletzung ergibt `ErrExists`, keine 500 |
| 4 | teilweise anders | Begründung des Moderators widerlegt (Offline-Node); Marken bleiben stehen, `grant` belebt sie wieder, Regel bleibt |
| 5 | fixed | Tunnel nur mit gleichem Port, in `vertrag.md` |
| 6 | fixed | Format 4 lehnt null schon ab (nur Test fehlt), Format 3 prüft am YAML-Knoten |

### Moderator-Entscheidungen
- 3 und 5 vom Moderator entschieden (siehe Routing); 5 kann der Nutzer überstimmen, falls
  Tunnel mit anderem Port vor `ssh`/`https` gebraucht werden.
- Gegenvorschlag des Editors zu 4 übernommen, vom Critic in Runde 2 bestätigt.
- N1 ohne weitere Editor-Runde direkt eingearbeitet (eindeutig, minimal).

### Intent-Alignment
Ja — jeder Punkt des Intents hat eine Etappe mit Tests; Ausnahmen bei „Test scheitert
vorher“ sind je Test benannt. Das Behandeln der Löschmarken geht über das Intent hinaus,
widerspricht ihm aber nicht.

### Geänderte Dateien
- 007-review-befunde-005.md: Intent (Account-Änderungen allgemein, Test-Ausnahmen), Kontext
  (Race zwischen Account-Änderungen, null im Import korrigiert, Löschmarken), Etappe 2
  (Sperre zuerst, bedingtes `rotate`, Namenstabelle, Fehlerabbildung, Tests), Etappe 3
  (Tunnel), Etappe 4 (Format 3/4, Marken bleiben, Import), Etappe 5 (Doku) (1, 2, 3, 4, 5, 6, N1)

### Offen (nicht gefixt)
- —
