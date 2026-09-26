# Task 012 — Nachbesserung Task 008: whoami robust, Replica-Anlage, Beenden von serve

Behebt die drei Befunde aus Review und Intent-Alignment von Task 008, die vor dem nächsten
Release erledigt sein sollen, und dazu das Verwerfen einer beschädigten Replica.

## Intent

`whoami` bleibt die eine Stelle, an der Clients und Menschen sehen, wie der Node steht —
auch wenn eine einzelne Replica kaputt ist —, und der Abgleich im Hintergrund schreibt nie in
einen fremden Hub-Eintrag und hält das Beenden von `serve` nicht auf.
- Eine unlesbare Replica (alte Schemafassung, ohne `entry_id`, beschädigt) betrifft in
  `whoami` und `node whoami` nur ihren eigenen Hub; die übrigen Hubs erscheinen
  vollständig. `status` zeigt sie wie bisher je Hub.
- Die Antwort von `whoami` bleibt bei den Feldern und den drei `login`-Werten aus
  `docs/konzept.md`; kein Token, Hash, Adresse, Transport, `hub_id`, `entry_id`; auch der
  Fehlertext in der Antwort von `whoami` und in der Ausgabe von `node whoami` (stdout,
  `--json`) nennt keinen Pfad. Die volle Meldung auf stderr der CLI ist Diagnose wie in
  `status` und darf ihn nennen.
- Eine beschädigte Replica bleibt nicht liegen: Der nächste Abgleich verwirft sie und legt sie
  neu an, wie eine alter Schemafassung.
- Nach `replica.Create` schreibt ein Abgleich nur in eine Replica, deren `entry_id` die
  seines Eintrags ist.
- `serve` beendet sich auch dann in begrenzter Zeit, wenn ein Hub den Abbruch nicht beachtet.

## Referenzen

- `k-playbook-local/tasks/done/008-abgleich-hintergrund-whoami.md` — „Ausführung“:
  Code-Review (Befund 1, Vorschläge 1 und 2), Intent-Alignment.
- `docs/fortschritt.md` — „Zu tun“, Nachbesserung Task 008 und kleinere Punkte (nicht Teil
  dieser Task, außer wo unten genannt).
- `docs/konzept.md` — „`whoami` — festgelegt am 2026-09-26“, „Im Hintergrund“.
- `k-playbook-local/k-playbook.md` — Aufbau, `serve`, MCP am Node, keine Migrationen.
- `internal/node/mcpnode/login.go` (`openReplica`, `Authenticate`, `AccountLogins`),
  `internal/node/mcpnode/whoami.go` (`Whoami`, `syncInfo`, `KnownAccounts`, `DescribeSync`),
  `internal/node/replica/replica.go` (`Create`, `Open`, `checkOwner`),
  `internal/node/replica/sync.go` (`openForSync`), `internal/sqlitedb/sqlitedb.go` (`Open`,
  `CheckInfo`), `cmd/kephalaion/serve.go` (`bgDone`, `shutdownGrace`),
  `cmd/kephalaion/bgsync.go`, `cmd/kephalaion/nodewhoamicmd.go`,
  `cmd/kephalaion/roles.go` (`printReplicaStatus`).

## Ziel

1. `whoami` (MCP) und `kephalaion node whoami` liefern auch dann eine Antwort, wenn eine oder
   mehrere Replicas sich nicht öffnen lassen; betroffen ist nur der jeweilige Hub.
2. `openForSync` verwirft eine eindeutig unlesbare Replica und legt sie neu an, wie bei alter
   Schemafassung; vorübergehende Fehler verwerfen nichts.
3. `replica.Create` gibt nur eine Replica mit der erwarteten `entry_id` zurück, sonst
   `ErrChanged`.
4. `serve` wartet beim Beenden höchstens `shutdownGrace` auf den Abgleich im Hintergrund.

## Kontext

- **Befund 1:** `openReplica` behandelt nur `sqlitedb.ErrNotFound` als „keine Replica“; jeder
  andere Fehler bricht `Authenticate`/`Whoami`/`KnownAccounts` für alle Hubs ab. Weil
  `whoami` über alle Hub-Einträge läuft, legt eine einzige solche Replica das Werkzeug lahm.
  Ein Abgleich verwirft heute nur eine Replica alter Schemafassung oder fremder `entry_id`
  (`openForSync`, vor dem Verbinden mit dem Hub). Eine beschädigte Datei oder eine ohne
  `hub_id`/`entry_id` liefert aus `Open` einen gewöhnlichen Fehler: Sie wird nie verworfen,
  jeder Abgleich scheitert daran mit `KindReplica`. Bei `sync_interval 0` bleibt auch eine
  alter Schemafassung liegen.
- **Beschädigte Replica verwerfen — entschieden am 2026-09-26:** `openForSync` verwirft auch
  eine eindeutig unlesbare Replica und legt sie neu an; `reason` sagt es, wie bei alter
  Schemafassung. Eindeutig unlesbar: SQLite meldet `NOTADB` oder `CORRUPT` (beim Öffnen oder
  beim Lesen von `db_info`), `db_info` fehlt oder hat keine Rolle, oder `hub_id`/`entry_id`
  fehlen. Nicht verworfen wird bei `BUSY`/`LOCKED`, abgebrochenem ctx, einem Fehler von
  `stat` oder Zugriffsrechten und bei einer Datei fremder Rolle — das bleibt ein Fehler des
  Abgleichs. Der Text in `status` („der nächste Abgleich legt sie neu an“) stimmt damit auch
  für eine beschädigte Replica.
- **Darstellung einer unlesbaren Replica — entschieden am 2026-09-26:** `login` ist
  `missing`, auch wenn ein Header-Paar kam — ohne lesbare Replica gibt es nichts, wogegen
  geprüft werden könnte. In `sync` keine Revision und als letzter Fehler ein fester Satz
  „Replica nicht lesbar“ (ohne Pfad, ohne Meldung); er ersetzt einen Fehler aus `hub_sync`,
  `last_error_at` bleibt leer — ein „jüngerer Fehler“ lässt sich nicht bestimmen, weil die
  Replica keinen Zeitstempel hat (Moderator-Entscheidung im Review). `last_success` kommt
  weiter aus `hub_sync`; der Fehler dort bleibt in `status` und im Log sichtbar.
  `never_synced` nur, wenn `hub_sync` keinen erfolgreichen Abgleich kennt; sonst nicht
  gesetzt. Die volle Meldung geht ins Log von `serve` bzw. nach stderr der CLI. `node whoami`
  (Liste) zeigt den Hub mit diesem Hinweis und ohne Accounts. Die Regel aus Task 008 für eine
  **fehlende** Replica (Header-Paar → `invalid`, „noch nie abgeglichen“) bleibt unverändert.
- **Was als „nicht lesbar“ gilt (nur Anzeige in `whoami`/`node whoami`):** jeder Fehler beim
  Öffnen der Replica-Datei außer `sqlitedb.ErrNotFound` (→ fehlt) und abgebrochenem ctx (→
  Fehler der Anfrage). Ein Fehler der Anfrage an `node.db` (Store `nodes`) bleibt ein Fehler
  der ganzen Anfrage. Diese Regel ist weiter als die für das Verwerfen oben; beide nicht
  vermischen.
- **`status`:** `printReplicaStatus` behandelt eine unlesbare Replica schon je Hub und gibt
  die volle Meldung aus, auch mit Pfad. Von Befund 1 nicht betroffen; keine Änderung, nur ein
  Regressionstest. Die Regel „kein Pfad“ gilt für `whoami` (MCP) und die Ausgabe von
  `node whoami` auf stdout, nicht für `status` und nicht für stderr.
- **Vorschlag 1:** Zwischen `os.Link` und `Open` in `Create` kann ein anderer Prozess
  (`node hub rm` + `add` + Abgleich des neuen Eintrags) die Datei ersetzen. `checkOwner`
  vergleicht gegen `r.entryID` der geöffneten Datei, nicht gegen die gewünschte; ist beim alten
  Eintrag `hub_id` unverändert, entfällt `SetHubID` als Sperre. Nach `Open`: `EntryID()` gegen
  die übergebene `entryID` prüfen, bei Abweichung schließen und `ErrChanged` — `SyncHub` setzt
  dann wie bisher neu auf.
- **Vorschlag 2:** `serve` wartet mit `<-bgDone` ohne Frist, bevor `shutdownGrace` beginnt.
  Mit `select` auf `bgDone` und einem Timer begrenzen; läuft die Frist ab, eine Logzeile und
  weiter beenden. Die Stores werden erst danach geschlossen. Ein weiterlaufender Abgleich wird
  hingenommen — der Prozess endet ohnehin —; er bekommt dann einen Fehler, darf aber nicht
  panicen und nichts in `hub_sync` schreiben (ctx ist abgebrochen).
- Nebenbei, weil dieselben Stellen: `DescribeSync` prüft `Revision` auf nil (Review-Vorschlag
  4).
- **Nicht in dieser Task:** die übrigen Review-Vorschläge 3, 5–12 und Review-Punkt 9 (stehen
  in `docs/fortschritt.md`).

## Zu bauen

### Etappe 1 — whoami robust gegen eine unlesbare Replica

- `openReplica` bzw. die Aufrufer unterscheiden „fehlt“, „nicht lesbar“ und Fehler von
  `node.db`; Darstellung wie im Kontext, eine Stelle für MCP und CLI.
- `DescribeSync` ohne Dereferenz von nil und mit sauberem Text ohne `LastErrorAt`.
- Tests über MCP und `node whoami` (Liste, Einzelansicht, `--json`): zwei Hubs, einer mit
  Replica alter Schemafassung, einer mit beschädigter Datei, dazu ein gesunder → der gesunde
  vollständig, die kaputten mit `login` `missing` (auch mit Header-Paar) und Hinweis;
  `never_synced` nur ohne Erfolg in `hub_sync`; `last_error` „Replica nicht lesbar“ auch
  dann, wenn `hub_sync` einen alten Fehler trägt; in der rohen Antwort kein Pfad, Token, Hash,
  Adresse, `hub_id`, `entry_id`. Abgebrochener ctx → Fehler der Anfrage.
- `status` mit derselben Lage: Regressionstest, dass es je Hub ausgibt und nicht abbricht;
  keine Änderung am Code.

### Etappe 2 — Verwerfen, Replica-Anlage und Beenden von serve

- `openForSync` verwirft eine eindeutig unlesbare Replica (Kontext); Tests: Datei mit
  Müll-Bytes und Replica ohne `entry_id` → Abgleich legt neu an, `Reset` nennt den Grund;
  abgebrochener ctx → Datei bleibt, kein Verwerfen.
- `Create` prüft nach `Open` die `entry_id`; Test: Datei zwischen Link und Open ersetzt
  (über einen Test-Haken oder Ersetzen vor `Open`) → `ErrChanged`, nichts geschrieben.
- `serve`: Warten auf `bgDone` mit `shutdownGrace` begrenzt; Test mit einem Hub, der den ctx
  nicht beachtet → `serve` endet innerhalb der Frist, `hub_sync` ohne Fehler aus dem Abbruch.
  Am Testende den hängenden Hub freigeben und auf die Goroutine warten, damit `make race`
  weder Leak noch Race meldet.
- `make check` grün; `make race` über `cmd/kephalaion` und `internal/node/...`.

### Etappe 3 — Doku

- `docs/konzept.md`, `whoami`: in der Tabelle die Definition von `missing` nachziehen
  (nichts geschickt oder Replica nicht lesbar); im Fließtext die Darstellung einer unlesbaren
  Replica samt `last_error`, `never_synced` und den Unterschied zur fehlenden (Header-Paar → `invalid`).
- `docs/konzept.md`, „Im Hintergrund“: Der Abgleich verwirft eine Replica alter Schemafassung,
  fremder `entry_id` oder eindeutig beschädigt und legt sie neu an; vorübergehende Fehler
  verwerfen nichts.
- `docs/fortschritt.md`: die drei Punkte und das Verwerfen einer beschädigten Replica nach
  „Erledigt“.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — whoami robust gegen eine unlesbare Replica | erledigt | 2026-09-26 | `mcpnode.UnreadableError`; `openReplica`/Lesefehler je Hub eingeordnet (ctx → Fehler der Anfrage); `Whoami` liefert die Meldungen zurück (Log per `reqlog.NoteError`, stderr in `node whoami`); `DescribeSync` ohne nil/Zeit; Tests MCP, CLI, `status` |
| 2 — Verwerfen, Replica-Anlage und Beenden von serve | erledigt | 2026-09-26 | `openForSync` verwirft eindeutig unlesbare Replicas (`sqlitedb.IsCorrupt`, `sqlitedb.ErrNoInfo`, fehlende IDs); `Create` prüft `entry_id` nach `Open` (Test-Haken `afterLink`); `serve` wartet höchstens `shutdownGrace` (jetzt `var`) auf den Abgleich; `make check` und `make race` (cmd, internal/node) grün |
| 3 — Doku | offen | | |

---
## Review-Log (2026-09-26)

**Pfad:** k-playbook-local/tasks (nur Dateien ohne Review-Log)
**Intent:** inline (`## Intent`)
**Runden:** 2

### Diskussion
- **`login` bei unlesbarer Replica (1):** Der Critic sah einen Widerspruch zur Tabelle in
  konzept.md (`missing` = „nichts geschickt“) und zur fehlenden Replica (→ `invalid`). Die
  Nutzerentscheidung `missing` bleibt; Etappe 3 zieht die Definition in der Tabelle nach und
  benennt den Unterschied zur fehlenden Replica.
- **Beschädigte Replica heilt nie (2):** Der Critic wies nach, dass `openForSync` nur bei alter
  Schemafassung oder fremder `entry_id` verwirft; eine beschädigte Datei bleibt, jeder Abgleich
  scheitert. Auf Rückfrage entschied der Nutzer, das in Task 012 mitzuerledigen.
- **„jüngerer Fehler“ (U1):** Vom Editor selbst aufgeworfen, vom Critic bestätigt: `hub_sync`
  hält nur einen Fehler, die Replica keinen Zeitstempel — nichts zu vergleichen. Aufgelöst per
  Moderator-Entscheidung (siehe unten).

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| 1 | WARNUNG | 012 | Kontext, Darstellung | `missing` trotz Header-Paar widerspricht der Definition in konzept.md | Tabelle in konzept.md ausdrücklich ändern oder `invalid` |
| 2 | WARNUNG | 012 | Kontext, Befund 1 | „bis ein Abgleich sie verwirft“ stimmt für beschädigte Dateien nicht; Hinweis „Replica nicht lesbar“ praktisch nie sichtbar | Verwerfen festlegen oder ausklammern |
| 3 | FEHLEND | 012 | Kontext, Darstellung | `never_synced` bei unlesbarer Replica offen | nur ohne Erfolg in `hub_sync` |
| 4 | WARNUNG | 012 | Intent / Etappe 1 | `status` ist schon je Hub robust, gibt Pfad aus | nur Regressionstest; „kein Pfad“ nur für whoami |
| 5 | FEHLEND | 012 | Vorschlag 2 / Etappe 2 | weiterlaufende Abgleich-Goroutine nach Frist; Leak/Race im Test | hinnehmen; Test gibt Hub frei |
| 6 | WARNUNG | 012 | Kontext / Etappe 1 | Erkennung „nicht lesbar“ offen; BUSY/ctx könnten als unlesbar gelten | Regel festlegen |
| U1 | WARNUNG | 012 | Kontext, Darstellung | „sofern `hub_sync` keinen jüngeren Fehler trägt“ nicht bestimmbar | fester Satz ersetzt Fehler aus `hub_sync`, `last_error_at` leer |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| 1 | decide + pass | Nutzerentscheidung bleibt; Doku muss Definition nachziehen | behoben |
| 2 | ask-user + pass | Produktfrage; Nutzer: verwerfen | behoben |
| 3 | decide + pass | `never_synced` nur ohne Erfolg in `hub_sync` | behoben |
| 4 | pass | verhindert unnötigen Umbau von `status` | behoben |
| 5 | pass | Testbarkeit mit `make race` | behoben |
| 6 | pass | zwei getrennte Regeln: Anzeige vs. Verwerfen | behoben |
| U1 | decide | Regel nicht bestimmbar; kleinste eindeutige Fassung | vom Moderator eingearbeitet |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| 1 | behoben | Etappe 3: Tabelle `missing` nachziehen, Unterschied zur fehlenden Replica |
| 2 | behoben | Prämisse korrigiert; Entscheidung im Kontext; Intent, Ziel 2, Etappe 2 mit Tests, Etappe 3 |
| 3 | behoben | im Kontext und als Testpunkt |
| 4 | behoben | Intent präzisiert, Kontextpunkt `status`, Etappe 1 Regressionstest |
| 5 | behoben | Kontext Vorschlag 2 und Test in Etappe 2 |
| 6 | behoben | Absatz „Was als ‚nicht lesbar‘ gilt“, Verwerfen-Regel getrennt |

### Moderator-Entscheidungen
- U1: Bei unlesbarer Replica ist `last_error` immer „Replica nicht lesbar“, ersetzt einen
  Fehler aus `hub_sync`, `last_error_at` bleibt leer; `DescribeSync` formatiert ohne Zeit
  sauber. Präzisiert die Nutzerentscheidung vom 2026-09-26, deren Nebensatz nicht umsetzbar war.
- Intent-Alignment ergab einen Widerspruch: Intent „kein Pfad“ gegen „volle Meldung nach
  stderr der CLI“. Aufgelöst, indem „kein Pfad“ auf die Antwort von `whoami` und die
  stdout-/`--json`-Ausgabe von `node whoami` bezogen wird; stderr ist Diagnose wie `status`.
- Editor-Festlegungen übernommen: Datei fremder Rolle wird nicht verworfen; kein Test für
  Rechtefehler (unter root unzuverlässig).

### Intent-Alignment
Nach Runde 2 „Nein“ wegen des stderr-Widerspruchs, sonst alle Intent-Punkte abgedeckt. Nach der
Präzisierung im Intent (Moderator) ist der einzige Befund behoben: Ja.

### Geänderte Dateien
- 012-nachbesserung-abgleich-whoami.md: Verwerfen beschädigter Replicas aufgenommen (2),
  Prämisse Befund 1 korrigiert (2), `never_synced` und `last_error` festgelegt (3, U1), Regeln
  „nicht lesbar“ vs. „eindeutig unlesbar“ (6), `status` nur Regressionstest (4), `serve`-Frist
  mit weiterlaufender Goroutine und Testaufräumen (5), konzept.md-Tabelle in Etappe 3 (1),
  Intent zu Pfad präzisiert (Alignment)

### Offen (nicht gefixt)
- —
