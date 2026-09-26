# Task 012 — Nachbesserung Task 008: whoami robust, Replica-Anlage, Beenden von serve

Behebt die drei Befunde aus Review und Intent-Alignment von Task 008, die vor dem nächsten
Release erledigt sein sollen.

## Intent

`whoami` bleibt die eine Stelle, an der Clients und Menschen sehen, wie der Node steht —
auch wenn eine einzelne Replica kaputt ist —, und der Abgleich im Hintergrund schreibt nie in
einen fremden Hub-Eintrag und hält das Beenden von `serve` nicht auf.
- Eine unlesbare Replica (alte Schemafassung, ohne `entry_id`, beschädigt) betrifft in
  `whoami`, `node whoami` und `status` nur ihren eigenen Hub; die übrigen Hubs erscheinen
  vollständig.
- Die Antwort von `whoami` bleibt bei den Feldern und den drei `login`-Werten aus
  `docs/konzept.md`; kein Token, Hash, Adresse, Transport, `hub_id`, `entry_id`; auch der
  Fehlertext nennt keinen Pfad.
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
  `internal/node/replica/sync.go` (`openForSync`), `cmd/kephalaion/serve.go` (`bgDone`,
  `shutdownGrace`), `cmd/kephalaion/bgsync.go`, `cmd/kephalaion/nodewhoamicmd.go`.

## Ziel

1. `whoami` (MCP) und `kephalaion node whoami` liefern auch dann eine Antwort, wenn eine oder
   mehrere Replicas sich nicht öffnen lassen; betroffen ist nur der jeweilige Hub.
2. `replica.Create` gibt nur eine Replica mit der erwarteten `entry_id` zurück, sonst
   `ErrChanged`.
3. `serve` wartet beim Beenden höchstens `shutdownGrace` auf den Abgleich im Hintergrund.

## Kontext

- **Befund 1:** `openReplica` behandelt nur `sqlitedb.ErrNotFound` als „keine Replica“; jeder
  andere Fehler bricht `Authenticate`/`Whoami`/`KnownAccounts` für alle Hubs ab. Weil
  `whoami` über alle Hub-Einträge läuft, legt eine einzige solche Replica das Werkzeug lahm —
  bis ein Abgleich sie verwirft; bei `sync_interval 0` oder einem Fehler beim Verbinden nie.
- **Darstellung einer unlesbaren Replica — entschieden am 2026-09-26:** `login` ist
  `missing`, auch wenn ein Header-Paar kam — ohne lesbare Replica gibt es nichts, wogegen
  geprüft werden könnte. In `sync` keine Revision und als letzter Fehler ein fester Satz
  „Replica nicht lesbar“ (ohne Pfad, ohne Meldung), sofern `hub_sync` keinen jüngeren Fehler
  trägt. Die volle Meldung geht ins Log von `serve` bzw. nach stderr der CLI. `node whoami`
  (Liste) zeigt den Hub mit diesem Hinweis und ohne Accounts. Ein Fehler von `node.db` selbst
  bleibt ein Fehler der ganzen Anfrage. Die Regel aus Task 008 für eine **fehlende** Replica
  (Header-Paar → `invalid`, „noch nie abgeglichen“) bleibt unverändert.
- **Vorschlag 1:** Zwischen `os.Link` und `Open` in `Create` kann ein anderer Prozess
  (`node hub rm` + `add` + Abgleich des neuen Eintrags) die Datei ersetzen. `checkOwner`
  vergleicht gegen `r.entryID` der geöffneten Datei, nicht gegen die gewünschte; ist beim alten
  Eintrag `hub_id` unverändert, entfällt `SetHubID` als Sperre. Nach `Open`: `EntryID()` gegen
  die übergebene `entryID` prüfen, bei Abweichung schließen und `ErrChanged` — `SyncHub` setzt
  dann wie bisher neu auf.
- **Vorschlag 2:** `serve` wartet mit `<-bgDone` ohne Frist, bevor `shutdownGrace` beginnt.
  Mit `select` auf `bgDone` und einem Timer begrenzen; läuft die Frist ab, eine Logzeile und
  weiter beenden. Die Stores werden erst danach geschlossen — ein noch laufender Abgleich
  bekommt dann einen Fehler, der nicht als Fehler in `hub_sync` landen darf (ctx ist
  abgebrochen) und kein Panic auslöst.
- Nebenbei, weil dieselben Stellen: `DescribeSync` prüft `Revision` auf nil (Review-Vorschlag
  4).
- **Nicht in dieser Task:** die übrigen Review-Vorschläge 3, 5–12 und Review-Punkt 9 (stehen
  in `docs/fortschritt.md`).

## Zu bauen

### Etappe 1 — whoami robust gegen eine unlesbare Replica

- `openReplica` bzw. die Aufrufer unterscheiden „fehlt“, „nicht lesbar“ und Fehler von
  `node.db`; Darstellung wie im Kontext, eine Stelle für MCP und CLI.
- `DescribeSync` ohne Dereferenz von nil.
- Tests über MCP und `node whoami` (Liste, Einzelansicht, `--json`): zwei Hubs, einer mit
  Replica alter Schemafassung, einer mit beschädigter Datei, dazu ein gesunder → der gesunde
  vollständig, die kaputten mit `login` `missing` (auch mit Header-Paar) und Hinweis; in der
  rohen Antwort kein Pfad, Token, Hash, Adresse, `hub_id`, `entry_id`. `status` mit derselben
  Lage stürzt nicht ab.

### Etappe 2 — Replica-Anlage und Beenden von serve

- `Create` prüft nach `Open` die `entry_id`; Test: Datei zwischen Link und Open ersetzt
  (über einen Test-Haken oder Ersetzen vor `Open`) → `ErrChanged`, nichts geschrieben.
- `serve`: Warten auf `bgDone` mit `shutdownGrace` begrenzt; Test mit einem Hub, der den ctx
  nicht beachtet → `serve` endet innerhalb der Frist, `hub_sync` ohne Fehler aus dem Abbruch.
- `make check` grün; `make race` über `cmd/kephalaion` und `internal/node/...`.

### Etappe 3 — Doku

- `docs/konzept.md` (`whoami`: Darstellung einer unlesbaren Replica), `docs/fortschritt.md`
  (die drei Punkte nach „Erledigt“).
