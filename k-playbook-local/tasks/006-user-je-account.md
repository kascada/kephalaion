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

## Kontext

- **Voraussetzung:** Task 005 ist abgeschlossen.
- **Hub-Schemafassung +1**, ohne Migration: neu anlegen, Einstellungen per `config
  export`/`import`.
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
  Dokumente bleiben unberührt, `admin` und ungültige Namen abgewiesen, Rundlauf Export/Import.

### Etappe 2 — Vertrag und Node

- `user` in `whoami` (Account-Teil) und `rotate`; die Replica liest ihn; MCP-`whoami` zeigt
  ihn je Hub.
- Tests über `local` und HTTP; MCP-`whoami` nennt den User, nie ein Token oder einen Hash.

### Etappe 3 — Doku

- `README.md` (Beispiel mit `--user`), `docs/vertrag.md`, `docs/begriffe.md` nur wo nötig,
  `docs/fortschritt.md` abhaken.
