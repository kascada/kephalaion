# Task 007 — Review-Befunde aus Task 005 beheben

Die Befunde aus dem Code-Review von Task 005 werden behoben: `rotate` kann keinen Account
mehr aussperren, der Hub-Store bleibt unter PostgreSQL korrekt, der HTTP-Weg ist gehärtet,
der Import leert nicht versehentlich.

## Intent

Der Unterbau aus Task 005 trägt verlässlich, bevor Suche, Schreiben und Abgleich im
Hintergrund darauf aufsetzen.
- Ein `rotate` endet nie als „eindeutig gescheitert“, wenn der Hub den neuen Hash schon
  gespeichert haben kann — über `local` genauso wie über `http`.
- `rotate` und die gemeinsame Eindeutigkeit von Node- und Account-Namen sind auch ohne die
  Schreibsperre von SQLite korrekt (PostgreSQL, READ COMMITTED).
- Kein Token verlässt den Node über eine Weiterleitung; Hub und Node prüfen `Host` gleich.
- Ein Import leert Accounts nur bei einem ausdrücklich leeren Teil, nie bei `accounts:` ohne Wert.
- Jeder Befund hat einen Test, der vorher scheitert.

## Referenzen

- `k-playbook-local/tasks/done/005-kommunikation-http-mcp.md` — Abschnitt „Ausführung“,
  Code-Review (Befunde und Vorschläge).
- `docs/vertrag.md` — `rotate`, `ErrOutcomeUnknown`, Fehlercodes.
- `docs/konzept.md` — „Authentifizierung“ (`rotate` Schritt für Schritt), „Lokale Tabellen“/Export.
- `k-playbook-local/k-playbook.md` — Hub-SQL PostgreSQL-tauglich, Import prüft wie die CLI.

## Kontext

- **Aussperrung (Befund 1):** `hub/replication.Rotate` liest `h.st.Info(ctx)` erst nach dem
  Commit von `RotateAccount`. Scheitert das, kommt ein gewöhnlicher Fehler zurück. Über HTTP
  wird daraus 500 → `ErrOutcomeUnknown` (richtig), über `local` behandelt
  `cmd/kephalaion/nodeaccountcmd.go` ihn als eindeutiges Scheitern: `.pending` gelöscht bzw.
  „altes Token gilt weiter“ — beides falsch.
- **Race bei `rotate` (Befund 2):** `RotateAccount` prüft per SELECT und schreibt per
  `UPDATE accounts SET token_hash = $2 WHERE name = $1` ohne Bedingung.
- **Namenseindeutigkeit (Befund 3):** Node und Account prüfen den Namen nur per Zählabfrage
  über die jeweils andere Tabelle, ohne Constraint.
- **Weiterleitungen:** Der `http.Client` in `internal/contract/httpapi/client.go` folgt Redirects;
  Go streicht bei fremdem Host den `Authorization`-Header, nicht aber den Body mit dem
  Account-Token von `rotate`.
- **Host am Hub:** Der Node prüft `Host` (Loopback, eigener Port) vor `/mcp`, der Hub-Listener
  unter `/v1/` nicht.
- **`accounts:` ohne Wert:** In Format 4 besteht ein YAML-null den Prüfschritt für
  vorhandene Teile, ergibt eine leere Liste und macht alle Accounts zu Löschmarken. In
  Format 3 wird null ebenso nicht erkannt.
- **Collections entfernen:** `collectionRemovable` zählt auch Löschmarken; `SYSTEM:A:`-Zeilen
  werden nie entfernt. Eine Collection, in der je ein Account Rechte hatte, lässt sich deshalb
  nie mehr entfernen, auch nach `revoke` oder `rm`.
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

- `RotateAccount` schreibt bedingt (`… WHERE name = $1 AND token_hash = $3`), prüft die Zahl
  der geänderten Zeilen und liefert bei 0 `account_unauthenticated` ohne weitere Änderung.
- Gemeinsame Eindeutigkeit von Node- und Account-Namen per Datenbank absichern, PostgreSQL-
  tauglich (z. B. eine gemeinsame Tabelle der belegten Namen mit Primärschlüssel, in derselben
  Transaktion wie Anlegen/Entfernen). Schema-Fassung des Hubs anheben.
- Tests: zweites `rotate` mit demselben alten Token scheitert auch, wenn die Vorprüfung
  umgangen wird; Anlegen eines Namens, der in der anderen Tabelle belegt ist, scheitert an der
  Datenbank; Name nach `rm` wieder frei.

### Etappe 3 — HTTP-Weg härten

- Der Client folgt keinen Weiterleitungen (`CheckRedirect` → `http.ErrUseLastResponse`);
  eine 3xx-Antwort ist ein Fehler, bei `rotate` ein eindeutiger (der Hub hat nicht ausgeführt).
- Der Hub-Listener prüft `Host` wie der Node (Loopback mit eigenem Port), sonst 403;
  gemeinsame Prüfung, nicht doppelt.
- Tests: Redirect-Ziel bekommt keinen Body, `rotate` hinter 307 scheitert eindeutig; fremder
  `Host` am Hub → 403.

### Etappe 4 — Import und Collections

- Import: `accounts:` mit null wird abgelehnt (Format 3 und 4); nur `accounts: []` leert.
- Collections entfernen: Löschmarken von `SYSTEM:A:`-Zeilen blockieren das Entfernen nicht
  und werden mit der Collection entfernt; lebende Zeilen und gemerkte Rechte eines
  gesperrten Accounts blockieren weiter.
- Tests: null-Teil in beiden Formaten → Abbruch ohne Änderung; Collection nach `revoke` bzw.
  `rm` entfernbar; mit lebender Zeile oder gemerkten Rechten nicht.

### Etappe 5 — Doku

- `docs/vertrag.md` (Weiterleitungen, `rotate` bei Fehler nach dem Commit, 403 am Hub),
  `docs/konzept.md` (Eindeutigkeit per Datenbank, Collections mit Löschmarken entfernen,
  Import mit null), `k-playbook-local/k-playbook.md` (Namenseindeutigkeit),
  `docs/begriffe.md` falls neue Begriffe.
