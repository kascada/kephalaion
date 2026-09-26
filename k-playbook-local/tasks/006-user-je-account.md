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
  Namen, bedingtes `rotate`, null-Teile im Import.

## Kontext

- **Voraussetzung:** Task 005 und Task 007 sind abgeschlossen. 007 zuerst: Es behebt Befunde
  im selben Code (`rotate`, `accounts`, Import) und hebt ebenfalls die Schemafassung des Hubs
  an. 006 setzt auf dem Stand nach 007 auf.
- **Hub-Schemafassung eins über dem Stand nach 007**, ohne Migration: neu anlegen,
  Einstellungen per `config export`/`import`.
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
- **`rotate`** schreibt die Zeilen weiter bedingt (Task 007) und mit unverändertem User; der
  Node übernimmt ihn aus der Antwort in die Replica.
- **Export:** Format eins über dem Stand nach 007 (heute 4), mit `user` je Account. Ein Import
  älterer Formate setzt den User auf den Namen des Accounts. Die Regeln aus 007 für null-Teile
  gelten weiter.
- **Namensregel des Users** wie bei Accounts; `admin` ist reserviert (Account und User der CLI
  am Hub).
- **`hub account list`** zeigt den User; `hub account list --user <user>` filtert über den
  Index. `hub account show` zeigt ihn.
- **Inhalt der `SYSTEM:A:`-Zeile:** `{"hash": …, "user": …, "rights": {…}}`. Der Node liest
  `user` mit; Zeilen ohne `user` gibt es nicht (Schema neu).
- **`hub account set --user`** schreibt alle `SYSTEM:A:`-Zeilen des Accounts in einer
  Transaktion neu (eine Revision, Zeile in `actions`); Dokumente behalten ihren
  `created_by`/`updated_by`.
- **Urheber:** Wo heute ein Account in `created_by`/`updated_by` geschrieben wird oder künftig
  würde, steht der User. Die CLI am Hub schreibt `admin`. `actions.account` bleibt der Account.
- **`docs/vertrag.md`:** `user` in den Antworten von `whoami` (Account-Teil) und `rotate`. Die
  Fassung bleibt 1, solange es keinen ausgelieferten Node mit dem alten Stand gibt — sonst
  hochzählen.
- **Nicht in diesem Task:** persönliche Verzeichnisse, Übergabe an einen anderen User,
  Rotation des Node-Tokens.

## Zu bauen

### Etappe 1 — User am Hub

- Spalte `user` in `accounts` samt Index, `--user` bei `add` und `set`, `SYSTEM:A:`-Zeilen mit
  `user`, `list` (mit `--user`) und `show`, Export und Import.
- Tests: ohne `--user` gleich dem Namen, `set --user` ändert alle Zeilen in einer Revision,
  Dokumente bleiben unberührt, `admin` und ungültige Namen abgewiesen, User gleich einem
  Node-Namen erlaubt, gesperrter Account (`set --user` → `unlock` → Zeilen mit neuem User),
  Rundlauf Export/Import, Import von Format 4 ergibt User = Name.

### Etappe 2 — Vertrag und Node

- `user` in `contract.AccountContent` und `contract.AccountStatus`; die Replica liest ihn;
  `mcpnode.HubLogin` zeigt ihn je Hub.
- Tests über `local` und HTTP; MCP-`whoami` nennt den User, nie ein Token oder einen Hash.

### Etappe 3 — Doku

- `README.md` (Beispiel mit `--user`), `docs/vertrag.md`, `docs/begriffe.md` nur wo nötig,
  `docs/fortschritt.md` abhaken.
