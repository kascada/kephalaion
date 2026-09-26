# Task 014 — Schreiben über MCP: create, write, delete, rename

Clients schreiben über den Node beim Hub — Dokumente anlegen, ersetzen, löschen, umbenennen,
Verzeichnisse als Ganzes —, und die Erweiterung für VS Code speichert damit.

## Intent

Ein Client schreibt über MCP in jede Collection, in der sein Account schreiben darf; der Hub
bleibt der einzige Schreiber und prüft Anmeldung, Recht, Form und Vorbedingung, und in VS Code
gehen Speichern, neue Dateien, Löschen, Umbenennen und Drag & Drop.
- Der Hub prüft jeden Schreibvorgang: `write` für Neues und Eigenes, `supersede` für Fremdes;
  `created_by`/`updated_by` ist der User, `actions` nennt Account und Node.
- Nichts wird still überschrieben: Name vergeben und veraltete Revision sind eigene,
  endgültige Fehler.
- Die eigene Änderung steht in der Replica, bevor der Client die Antwort bekommt —
  zweimal hintereinander speichern geht ohne Konflikt.
- Kein Schreibvorgang wird nach dem Abschicken wiederholt; „nicht erreichbar“, „Ausgang
  unklar“ und „abgelehnt“ sind drei unterscheidbare Meldungen. Lesen läuft offline weiter.
- `delete` und `rename` behandeln ein Verzeichnis als Ganzes: alles oder nichts, eine Revision.

## Referenzen

- `docs/konzept.md` — „Die tragende Entscheidung“, „Der Weg eines Eintrags“, „Zwei Arten von
  Eingaben“ (Name vergeben, Revision als Vorbedingung), „Löschen“, „Collections, Accounts,
  Rechte“ (Rechte, Urheber), „Transport, Token und Fehlschläge“, „Authentifizierung“ (Wer wann
  prüft), „Werkzeuge“ → „Allgemein — schreiben“ (Festlegungen dieses Tasks), „Stufen“.
- `docs/vertrag.md` — Aufbau eines Vorgangs, Anmeldung von Node und Account, Fehler, HTTP;
  `rotate` als Vorbild für „nie wiederholen“ und „Ausgang unklar“.
- `docs/vscode.md` — Tabelle „Die Vorgänge und ihre Entsprechung“, Konflikte, Offline, Rechte.
- `docs/begriffe.md` — die Begriffe dieses Tasks stehen dort schon (als geplant markiert);
  weitere vor Benutzung eintragen.
- `k-playbook-local/k-playbook.md` — Regeln: Hub und Node getrennt, Vertrag zweimal gleich,
  Umsetzung nur in `cmd/kephalaion`, Hub-SQL PostgreSQL-tauglich, Account-Zeile zuerst
  sperren, kein Token in MCP.
- Code:
  - `internal/hub/store/documents.go` — `docTx`, `put`/`create`/`replace`, `DeleteDocument`,
    `checkPathFree`, `dirRange`, `CheckContent`, `MaxDocumentBytes`; Urheber heute fest `Admin`;
  - `internal/hub/store/accounts.go` — Account prüfen, Sperre der Account-Zeile;
  - `internal/hub/replication/replication.go` — `Whoami`/`Rotate` als Vorbild;
  - `internal/contract/contract.go` — `Hub`, `Codes`, `ErrOutcomeUnknown`;
  - `internal/contract/httpapi/` — `MaxBodyBytes`, Wiederholung im Client;
  - `internal/node/replica/accounts.go` — `WriteAccountRows` (Zeilen aus einer Antwort in die
    Replica);
  - `internal/node/mcpnode/` — `access.go`, `read.go`, `NewHandler`;
  - `cmd/kephalaion/synccmd.go` (`connector`, `localHub`), `bgsync.go`, `serve.go`;
  - `vscode/extension.js`, `vscode/package.json` (Stand 0.0.4: Account je Hub wählbar).

## Ziel

Stufe 2 des Konzepts, dazu `rename` (vorgezogen aus Stufe 3): der Vertrag Node → Hub um
`create`, `write`, `delete` und `rename`, dieselben vier als MCP-Werkzeuge am Node, und die
Erweiterung schreibt über sie.

## Kontext

- **Voraussetzung:** Tasks 001–012 abgeschlossen.
- **Nicht in diesem Task:** `create_numbered`, `append`, `replace_section`, `supersede`
  (Ablösen), `replace_directory`, `meta`; Überschreiben beim `rename`, Verschieben über
  Collections oder Hubs; Idempotenzschlüssel und Wiederholung; Ereignisstrom; `https`/`ssh`;
  persönliche Verzeichnisse; die Erweiterung im Release. `hub doc put|rm` und `hub import`
  bleiben Admin-Vorgänge wie bisher.
- **Fassung bleibt 1** — die vier Vorgänge kommen nur hinzu, es gibt keinen ausgelieferten
  Node mit Vertrag (wie bei `user` in Task 006). Kennt ein Hub den Vorgang nicht (404,
  `invalid`), meldet der Node „Hub kann noch nicht schreiben“.
- **Anmeldung:** der Node wie bisher (Header); der Account im Body `{"account", "token"}` wie
  bei `whoami`, geprüft gegen `accounts` (maßgeblich, gesperrt = ungültig):
  `account_unauthenticated`. Der Node prüft den Client vorher wie beim Lesen gegen die Replica
  (`Authenticate`) — nicht angemeldet oder Collection nicht lesbar: dieselbe Meldung „nicht
  lesbar“, ohne den Hub zu fragen. Ob geschrieben werden darf, entscheidet allein der Hub; so
  wirkt eine Sperre beim Schreiben sofort.
- **Prüfung am Hub**, in einer Transaktion: zuerst die Zeile des Accounts in `accounts`
  sperren (Regel in `k-playbook.md`), dann Hash, gesperrt und User lesen. Die Collection muss
  es geben, der Node muss sie abgleichen dürfen (`node_collections`), der Account muss eine
  lebende `SYSTEM:A:`-Zeile in ihr haben — sonst eine Antwort: `not_readable`. `write` aus der
  Zeile für `create` und Eigenes (`created_by` = User des Accounts, auch wenn ein anderer
  Account desselben Users es angelegt hat), zusätzlich `supersede` für Fremdes — bei
  Verzeichnissen für jedes Dokument darunter. Sonst `forbidden` mit dem Grund in der Meldung
  („gehört admin, `supersede` fehlt“).
- **Urheber:** Der Schreibvorgang am Hub trägt User, Account und Node. `created_by`/
  `updated_by` = User; `actions` je Dokument eine Zeile mit `account`, `carrier` = Node und
  `action` (`create`, `update`, `delete`, `rename`) unter der Revision des Vorgangs. Die CLI am
  Hub bleibt `admin` ohne Träger.
- **Vorgänge** — je eine Transaktion, höchstens eine Revision:
  - `create` (`collection`, `name`, `content`): lebendes Dokument mit dem Namen →
    `name_taken` (endgültig); eine Löschmarke unter dem Namen hindert nicht, neue `id`. Datei
    und Verzeichnis zugleich → `path_conflict`.
  - `write` (`collection`, `name`, `content`, wahlweise `base_revision`): kein lebendes
    Dokument → `not_found`. `base_revision` weicht von der Revision des lebenden Dokuments ab
    → `stale_revision` (Meldung nennt die aktuelle), geprüft vor dem Vergleich des Inhalts.
    Unveränderter Inhalt: keine neue Revision, Antwort mit der bestehenden Zeile.
  - `delete` (`collection`, `name`, wahlweise `base_revision`, `recursive`): `name` ist ein
    lebendes Dokument → Löschmarke. Ist es ein Verzeichnis (lebende Dokumente darunter), nur
    mit `recursive: true`, sonst `invalid` („ist ein Verzeichnis“); dann alle darunter
    Löschmarken unter einer Revision. `base_revision` nur für Dokumente. Weder noch →
    `not_found`. Die Wurzel einer Collection wird nie gelöscht.
  - `rename` (`collection`, `name`, `new_name`, wahlweise `base_revision`): `id` bleibt. Ein
    Verzeichnis: alle Dokumente darunter bekommen den neuen Präfix, alles oder nichts, eine
    Revision; jeder neue Name wird geprüft (`CheckDocName`). Ziel lebend belegt →
    `name_taken` (kein Überschreiben); `path_conflict`; Ziel in der Quelle (`x` → `x/y`) oder
    gleicher Name → `invalid`. Nur innerhalb einer Collection.
  - Name und Inhalt prüft der Hub wie bisher (`CheckDocName`, kein `SYSTEM:`; `CheckContent`:
    UTF-8, keine NUL, höchstens 1 MiB). Leere Dokumente sind erlaubt — „Neue Datei“ in
    VS Code schreibt zuerst leer.
- **Antwort** jedes Vorgangs: `hub_id`, `version`, `revision` (die neue; bei unverändertem
  Inhalt die bestehende) und `rows` — die geschriebenen Zeilen in der Form von `sync`, bei
  `delete` die Löschmarken. Alles, was die Antwort braucht, liest der Hub vor dem Commit. Die
  Antwort ist die Wahrheit, nicht die Anfrage.
- **Fehlercodes neu:** `name_taken` (409), `path_conflict` (409), `stale_revision` (409),
  `not_found` (404), `forbidden` (403), `not_readable` (403). Über HTTP unterscheidet der Client
  nach dem Code im Body, nicht nach dem Status.
- **Transport:** `create`, `write`, `delete`, `rename` werden nie wiederholt. Kam keine
  Verbindung zustande oder eine Weiterleitung: nichts geschehen („Hub nicht erreichbar, nichts
  gespeichert“). Jeder andere Fehler nach dem Abschicken (Zeitüberschreitung, abgebrochene
  Verbindung, unlesbare Antwort, 5xx) ist `contract.ErrOutcomeUnknown`; über `local` jeder
  Fehler, der kein Fehler des Vertrags ist (wie `localHub` bei `rotate`). Die Grenze des Bodys
  für Schreibvorgänge muss jedes Dokument tragen, das der Store annimmt, auch bei größter
  Aufblähung durch JSON (`\u00XX` = 6 Byte) — mindestens 6 × `MaxDocumentBytes` plus Rand;
  ebenso am MCP-Eingang des Nodes. Log: Node- und Account-Name, nie Token, nie Inhalt.
- **Eigene Änderung sofort sichtbar:** Nach Erfolg schreibt der Node die `rows` der Antwort in
  die Replica des Hub-Eintrags, bevor er dem Client antwortet — `WriteAccountRows`
  verallgemeinert: gebunden an `entry_id`/`hub_id`, Zeilen nur vorwärts, nur gewünschte
  Collections, `sync_state` bleibt. Danach stößt er den Abgleich dieses Hubs an, ohne darauf zu
  warten (unter `serve` über `backgroundSync`). Scheitert das Schreiben in die Replica, bleibt
  es ein Erfolg mit Hinweis; der Abgleich holt nach. `changes` meldet die eigene Änderung erst
  nach dem nächsten Abgleich (liest bis `sync_state`) — gewollt.
- **MCP am Node:** Werkzeuge `create`, `write`, `delete`, `rename` mit der Adresse wie beim
  Lesen (`<hub>:<collection>`, Hub-Teil darf fehlen). Den Weg zum Hub bekommt `mcpnode` von
  `cmd/kephalaion` als Funktion über den `connector`, dazu den Anstoß des Abgleichs;
  `internal/node` kennt weiter nur `contract.Hub`. `https`/`ssh`: „noch nicht unterstützt“.
  Antwort: Adresse, Name, `id`, Revision, geändert (wann, von wem), Größe; bei Verzeichnissen
  Zahl der Dokumente und Revision. Fehler tragen neben der Meldung einen Code (`name_taken`,
  `stale_revision`, `forbidden`, `not_found`, `path_conflict`, `not_readable`, `invalid`,
  `unreachable`, `outcome_unknown`), den die Erweiterung auswertet, nicht die Meldung. Die
  Meldung bei `outcome_unknown` sagt, dass es gespeichert sein kann und wie man nachsieht.
  Beschreibungen kurz und deutsch.
- **Erweiterung:** baut auf 0.0.4 auf (Account je Hub wählbar).
  - `stat` schreibgeschützt nur ohne `writable`; `isReadonly` wird `false`.
  - `writeFile`: `read` mit `content: false`; `none` → `create` (ohne `options.create`:
    `FileNotFound`); `document` → `write` mit dessen Revision (ohne `options.overwrite`:
    `FileExists`); `directory` → `FileIsADirectory`. Inhalt streng als UTF-8; kein Text oder zu
    groß → klare Meldung vor dem Aufruf.
  - `delete` → `delete` mit `recursive` aus den Optionen.
  - `rename` innerhalb einer Collection → `rename`; über Collections oder Hubs, oder mit
    `overwrite` bei belegtem Ziel → Meldung.
  - `createDirectory` nur in der Erweiterung gemerkt, bis darin etwas angelegt oder es
    gelöscht wird; `stat` und `readDirectory` zeigen es.
  - Nach Erfolg die Ereignisse selbst feuern. Codes auf `FileSystemError`: `name_taken` →
    `FileExists`, `not_found` → `FileNotFound`, `forbidden`/`not_readable` → `NoPermissions`,
    `unreachable`/`outcome_unknown` → `Unavailable` mit Meldung, sonst ein Fehler mit der
    Meldung des Nodes.
  - Nächste Version (0.0.5).
- **Testdaten im echten Store** nur unter `test/` in `home:eins`. Dokumente, die dort mit
  `hub doc put` angelegt wurden, gehören `admin` — ein Account ohne `supersede` darf sie nicht
  ändern; das ist zugleich der Test für `forbidden`.

## Zu bauen

### Etappe 1 — Hub-Store: Urheber, Rechte, create, write, delete

- Wer schreibt (User, Account, Node) als Teil des Schreibvorgangs statt fest `Admin`; die CLI
  bleibt `admin`.
- `create`, `write` (mit `base_revision`), `delete` für Dokumente, mit Rechteprüfung und Sperre
  der Account-Zeile; Abfragen in `queries`, `sqlq.Check`.
- Tests: `created_by`/`updated_by` = User, `actions` mit Account und Node; Eigenes mit
  `write`, Eigenes eines anderen Accounts desselben Users, Fremdes ohne/mit `supersede`, ohne
  `write`, gesperrter Account; `name_taken`, Löschmarke neu anlegen (neue `id`),
  `path_conflict`, `stale_revision`, unveränderter Inhalt ohne Revision; leeres Dokument;
  `SYSTEM:`-Name abgelehnt.

### Etappe 2 — Vertrag: create, write, delete über local und HTTP

- `docs/vertrag.md` und `internal/contract` im selben Commit: Vorgänge, Felder, Antwort, Codes
  mit HTTP-Status, „nie wiederholen“, Grenze des Bodys.
- `hub/replication`: Anmeldung, `not_readable`; `httpapi`: Pfade, keine Wiederholung, unklarer
  Ausgang; `localHub`: Nicht-Vertragsfehler unklar.
- Tests gegen `local` und HTTP: jeder Code; Node ohne `replicate`; Account unbekannt oder
  gesperrt; Dokument an der Größengrenze aus Zeichen, die JSON aufbläht, über HTTP; kein
  Wiederholen nach 5xx und Zeitüberschreitung (unklar), Verbindung verweigert (nichts
  gespeichert); Hub ohne den Vorgang.

### Etappe 3 — Node: Werkzeuge create, write, delete

- Werkzeuge in `mcpnode` über `access.go`; Weg zum Hub und Anstoß des Abgleichs aus
  `cmd/kephalaion`; Antwortzeilen in die Replica; Fehlercodes.
- Tests mit dem MCP-Client des go-sdk: schreiben → sofort `read` mit neuer Revision, ohne
  Abgleich; zweimal hintereinander `write` mit der Revision aus `read`; `changes` nach dem
  Abgleich; Hub über `http` gestoppt → `unreachable`, Lesen geht weiter; `connectHTTP` mit
  `ErrOutcomeUnknown` → `outcome_unknown`; „nicht lesbar“ ohne Anfrage an den Hub; kein Token
  in Antwort und Log.

### Etappe 4 — rename und Verzeichnisse

- `rename` durch alle Schichten (Store, Vertrag, Werkzeug); `delete` mit `recursive` für
  Verzeichnisse.
- Tests: `id` bleibt; Verzeichnis umbenennen und löschen: eine Revision, alles oder nichts,
  auch mit einem fremden Dokument ohne `supersede` mittendrin; `name_taken` am Ziel,
  `path_conflict`, `x` → `x/y`, gleicher Name; Verzeichnis ohne `recursive`; Replica nach
  `rename` sofort richtig (alter Name weg, neuer da).

### Etappe 5 — Erweiterung

- `vscode/extension.js` nach „Kontext“, Version 0.0.5, `.vsix` bauen.
- Geprüft mit dem Ersatz für `vscode` gegen den laufenden Node (`home:eins`, nur unter
  `test/`): neue Datei (leer, dann Inhalt), zweimal speichern, Konflikt nach `hub doc put` auf
  dasselbe Dokument, Löschen von Datei und Verzeichnis, Umbenennen von Datei und Verzeichnis,
  leeres Verzeichnis anlegen und befüllen, Binärdatei abgelehnt, fremdes Dokument nicht
  änderbar, Node bzw. Hub nicht erreichbar.

### Etappe 6 — Durchlauf und Doku

- `serve` mit Hub und Node, ein zweiter Node über `http`: Schreiben von beiden, Konflikt
  zwischen beiden, Hub gestoppt.
- Liste der Handgriffe für den echten VS Code, die der Nutzer durchgeht: Speichern, neue
  Datei, Ordner per Drag & Drop, Umbenennen und Löschen im Explorer, „Datei ist neuer“.
- Doku: `README.md` (Schreiben über MCP), `docs/begriffe.md` (Markierung „geplant“ entfernen,
  nachziehen), `docs/konzept.md` (Stand, Werkzeuge gebaut, Stufen), `docs/vscode.md` (Stand,
  Umsetzung Schreiben), `k-playbook-local/k-playbook.md` (`contract.Hub`, `mcpnode`,
  Urheber), `docs/fortschritt.md`.
