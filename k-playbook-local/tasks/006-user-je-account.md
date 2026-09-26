# Task 006 — User je Account

Jeder Account trägt einen User; `created_by`/`updated_by` sind der User, `whoami` und
`rotate` nennen ihn.

## Intent

Account und User nach `docs/konzept.md`, „Account und User“ (entschieden 2026-09-26), sind
gebaut.
- `hub account add|set --user`; ohne Angabe ist der User der Name des Accounts.
- Der User steht in `accounts` (mit Index) und im Inhalt jeder `SYSTEM:A:`-Zeile; ändert der
  Admin ihn, ändern sich alle Zeilen des Accounts in einer Transaktion (eine Revision).
  Vorhandene Dokumente behalten ihren User.
- `created_by`/`updated_by` sind der User des schreibenden Accounts; die CLI am Hub schreibt
  als `admin` (Account und User).
- `whoami` (MCP und `/v1/` mit Account-Teil) und die Antwort von `rotate` nennen den User; der
  Node liest ihn aus der Replica.
- Export und Import nehmen den User mit.

## Referenzen

- `docs/konzept.md` — „Collections, Accounts, Rechte“ (Account und User, Rechte, Urheber),
  „Datenmodell“ (`SYSTEM:A:`-Zeilen mit `user`, `created_by`/`updated_by`, `actions.account`),
  „Authentifizierung“ (Einrichtung mit `--user`).
- `docs/begriffe.md` — account, user, admin, actions.
- `docs/vertrag.md` — `whoami`, `rotate`.
- `docs/fortschritt.md` — Eintrag „nach Abschluss von Task 005 nachziehen“.
- `k-playbook-local/tasks/done/005-kommunikation-http-mcp.md` — Stand davor.
- `k-playbook-local/tasks/007-review-befunde-005.md` — läuft vorher; Tabelle der belegten
  Namen, Sperre der Account-Zeile zuerst, bedingtes `rotate`, null-Teile im Import.

## Kontext

- **Voraussetzung:** Task 005 und Task 007 sind abgeschlossen. 007 zuerst: Es behebt Befunde
  im selben Code (`rotate`, `accounts`, Import) und hebt ebenfalls die Schemafassung des Hubs
  an. 006 setzt auf dem Stand nach 007 auf.
- **Hub-Schemafassung eins über dem Stand nach 007**, ohne Migration: neu anlegen,
  Einstellungen per `config export`/`import`.
- **Neu angelegter Hub:** `hub init` vergibt eine neue `hub_id`; der Node leert beim nächsten
  Kontakt die Replica (`AdoptHubID`, Syncer) und gleicht von vorn ab. Bis dahin liegen in der
  Replica alte `SYSTEM:A:`-Zeilen ohne `user`.
- **Stellen im Code (Stand nach 005):** Tabelle `accounts` in `internal/hub/store/store.go`;
  `contract.AccountContent` (Inhalt der `SYSTEM:A:`-Zeile) und `contract.AccountStatus`
  (Account-Teil von `whoami`) in `internal/contract`; die Antwort von `rotate` trägt die Zeilen,
  der User kommt dort über den Inhalt mit; die Replica liest Accounts in
  `internal/node/replica/accounts.go`; MCP-`whoami` antwortet mit `mcpnode.HubLogin`.
- **Users sind kein Teil der gemeinsamen Namen von Accounts und Nodes** (Tabelle der belegten
  Namen aus Task 007). Ohne Angabe ist der User gerade der Name des Accounts, und ein User darf
  wie ein Node heißen — `created_by` und `actions.account`/`carrier` sind verschiedene Felder.
  Reserviert ist nur `admin`.
- **Gesperrter Account:** `set --user` ändert `accounts.user`; die Zeilen sind Löschmarken und
  bleiben es. `unlock` legt die Zeilen mit dem aktuellen User neu an. Ein Account ohne
  Collection hat den User nur in `accounts`, der erste `grant` schreibt ihn in die Zeile.
- **`rotate`** schreibt die Zeilen weiter bedingt (Task 007) und mit unverändertem User: Hash
  und User liest es in derselben Transaktion nach der Sperre der Account-Zeile.
  `updated_by` der Zeilen ist der User, `actions.account` bleibt der Account. Der Node
  übernimmt den User aus der Antwort in die Replica.
- **Export:** Format eins über dem Stand nach 007 (heute 4, also 5), mit `user` je Account.
  In Format 5 ist `user` Pflicht: fehlt er, ist er null oder leer, bricht der Import ohne
  Änderung ab; geprüft mit derselben Funktion wie die CLI (Namensregel, `admin` reserviert).
  Ein Import älterer Formate setzt den User auf den Namen des Accounts. Die Regeln aus 007
  für null-Teile gelten weiter.
- **Namensregel des Users** wie bei Accounts; `admin` ist reserviert (Account und User der CLI
  am Hub).
- **`hub account list`** zeigt den User; `hub account list --user <user>` filtert über den
  Index. `hub account show` zeigt ihn.
- **Inhalt der `SYSTEM:A:`-Zeile:** `{"hash": …, "user": …, "rights": {…}}`. Der Node liest
  `user` mit. Eine Zeile ohne `user` (alter Hub, siehe oben) ist ein Fehler dieser Zeile:
  nicht angemeldet, kein Absturz.
- **`hub account set --user`** folgt dem Muster aus 007 (Account-Zeile zuerst sperren, erst
  danach lesen) und schreibt alle lebenden `SYSTEM:A:`-Zeilen des Accounts in einer
  Transaktion neu (eine Revision, Zeile in `actions`); Löschmarken bleiben unverändert, auch
  die in entfernten Collections (Task 007). Dokumente behalten ihren `created_by`/`updated_by`.
- **Urheber:** Wo heute ein Account in `created_by`/`updated_by` geschrieben wird oder künftig
  würde, steht der User. Die CLI am Hub schreibt `admin`. `actions.account` bleibt der Account.
- **`whoami`:** `user` nur bei gültiger Anmeldung, sonst leer — für `/v1/whoami`, `local` und
  MCP-`whoami`.
- **`docs/vertrag.md`:** `user` in den Antworten von `whoami` (Account-Teil) und `rotate`. Die
  Fassung bleibt 1: v0.1.0/v0.1.1 liegen vor Task 005, es gibt keinen ausgelieferten Node
  mit Vertrag.
- **Nicht in diesem Task:** persönliche Verzeichnisse, Übergabe an einen anderen User,
  Rotation des Node-Tokens.

## Zu bauen

### Etappe 1 — User am Hub

- Spalte `user` in `accounts` samt Index, `--user` bei `add` und `set`, `SYSTEM:A:`-Zeilen mit
  `user`, `list` (mit `--user`) und `show`, Export (Format 5) und Import.
- Tests: ohne `--user` gleich dem Namen, `set --user` ändert alle Zeilen in einer Revision,
  Dokumente bleiben unberührt, `admin` und ungültige Namen abgewiesen, User gleich einem
  Node-Namen erlaubt, gesperrter Account (`set --user` → `unlock` → Zeilen mit neuem User),
  `rotate` schreibt `updated_by` = User und `actions.account` = Account, Rundlauf
  Export/Import, Import von Format 4 ergibt User = Name, Import von Format 5 mit fehlendem,
  null, leerem, ungültigem User oder `admin` → Abbruch ohne Änderung.

### Etappe 2 — Vertrag und Node

- `user` in `contract.AccountContent` und `contract.AccountStatus`; die Replica liest ihn;
  `mcpnode.HubLogin` zeigt ihn je Hub.
- Tests über `local` und HTTP; MCP-`whoami` nennt den User, nie ein Token oder einen Hash;
  mit falschem Token ist `user` leer (`/v1/whoami`, `local`, MCP); Replica-Zeile ohne `user`
  → nicht angemeldet, kein Absturz.

### Etappe 3 — Doku

- `README.md` (Beispiel mit `--user`), `docs/vertrag.md`, `docs/begriffe.md` nur wo nötig,
  `docs/fortschritt.md` abhaken.

---
## Review-Log (2026-09-26)

**Pfad:** k-playbook-local/tasks (gemeinsam mit 007)
**Intent:** inline, dazu `docs/konzept.md` „Account und User“
**Runden:** 2

### Diskussion
- **N2 (verwaiste Löschmarken):** Nach der Änderung in 007 (Marken bleiben beim Entfernen
  einer Collection) hätte „alle Zeilen neu schreiben“ bei `set --user` auch Marken in
  entfernten Collections berührt. Präzisiert auf lebende Zeilen.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| 1 | FEHLER | 007, 006 | Kontext `set --user`/`rotate` | `set --user` kann unter READ COMMITTED mit `rotate` verschränkt den alten Hash zurückschreiben | `set --user` folgt dem Sperrmuster aus 007 |
| 7 | FEHLEND | 006 | Kontext Export | `user` in Format 5 fehlend/null/leer offen, Prüfung beim Import nicht genannt | Pflicht, Abbruch, gleiche Prüffunktion wie CLI |
| 8 | FEHLEND | 006 | `whoami` | `user` könnte bei falschem Token ausgeliefert werden | Nur bei gültiger Anmeldung |
| 9 | WARNUNG | 006 | Vertragsfassung | Bedingung „kein ausgelieferter Node“ nicht bewertbar | Entscheidung festschreiben |
| 10 | WARNUNG | 006 | Urheber bei `rotate` | Woher `rotate` den User für `updated_by` nimmt | Nach der Sperre in derselben Transaktion lesen |
| 11 | FEHLEND | 006 | Schemafassung/Node | Neue `hub_id`, Replica-Reset und Zeilen ohne `user` nicht erwähnt | Ergänzen, Zeile ohne `user` = nicht angemeldet |
| N2 | WARNUNG | 006 | `set --user` | „alle Zeilen“ würde verwaiste Marken neu schreiben | „alle lebenden Zeilen“ |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| 1 | pass | Aussperrung, Kern von 007 | gefixt |
| 7 | pass | Import-Verhalten sonst geraten | gefixt |
| 8 | pass | Sicherheitsrelevant | gefixt |
| 9 | decide | v0.1.0/v0.1.1 liegen vor Task 005 → Fassung bleibt 1 | gefixt |
| 10 | pass | Zusammen mit 1 | gefixt |
| 11 | pass | Am Code geprüft (`hub init`, `AdoptHubID`) | gefixt |
| N2 | decide | Kleine Präzisierung, vom Moderator direkt eingearbeitet | gefixt |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| 1 | fixed | `set --user` sperrt zuerst die Account-Zeile, liest danach |
| 7 | fixed | Format 5: `user` Pflicht, gleiche Prüfung wie CLI; Tests in Etappe 1 |
| 8 | fixed | `user` nur bei gültiger Anmeldung (`/v1/`, `local`, MCP); Test in Etappe 2 |
| 9 | fixed | Fassung bleibt 1, begründet |
| 10 | fixed | Hash und User nach der Sperre lesen; `updated_by` = User, `actions.account` = Account; Test |
| 11 | fixed | Kontext zu neuer `hub_id`; Zeile ohne `user` = nicht angemeldet, kein Absturz; Test |

### Moderator-Entscheidungen
- 9 vom Moderator entschieden (Tags am Repo geprüft).
- N2 ohne weitere Editor-Runde direkt eingearbeitet.

### Intent-Alignment
Ja — alle Punkte des Intents und die Kernaussagen aus „Account und User“ sind abgedeckt; der
Anschluss an 007 (Sperre zuerst, bedingtes `rotate`, Namenstabelle ohne Users, Schemafassung)
ist ausdrücklich geregelt. Die Rechteprüfung „Eigenes = gleicher User“ wird bewusst nicht
gebaut, da Dokumente noch nicht geschrieben werden.

### Geänderte Dateien
- 006-user-je-account.md: Referenz auf 007 ergänzt, Kontext (neuer Hub, `rotate`, Export
  Format 5, Zeile ohne `user`, `set --user` mit Sperre und nur lebenden Zeilen, `whoami`,
  Vertragsfassung), Tests in Etappe 1 und 2 (1, 7, 8, 9, 10, 11, N2)

### Offen (nicht gefixt)
- —
