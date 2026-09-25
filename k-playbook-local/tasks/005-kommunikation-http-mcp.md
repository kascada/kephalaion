# Task 005 — Kommunikation: Accounts, serve, Hub über HTTP, Node als MCP-Server, rotate

Alle Beteiligten reden miteinander: Clients per MCP mit dem Node, der Node per HTTP (oder
`local`) mit dem Hub. Bewiesen wird es mit `whoami` auf jeder Strecke und mit `rotate`, das
einmal ganz durchläuft.

## Intent

Nach diesem Task steht die Kommunikation, auf die Suche, Schreiben und Abgleich im
Hintergrund später nur noch aufsetzen.
- `kephalaion serve` startet je eingerichteter Rolle einen HTTP-Listener auf ihrem `listen`;
  Hub und Node teilen sich den Unterbau, nicht das Protokoll.
- Node ↔ Hub: HTTP mit JSON unter `/v1/` (`whoami`, `rotate`, `sync`), als zweite Umsetzung
  derselben Schnittstelle wie `local`; die Tests des Vertrags laufen gegen beide.
- Client ↔ Node: MCP (Streamable HTTP) mit dem Werkzeug `whoami`; Account und Token kommen
  als Header und werden gegen die Replica geprüft, ohne Cache.
- Jeder Weg prüft: der Hub den Node (Name, Token, Sperre) und bei `rotate` den Account, der
  Node den Account des Clients. Auch `local` prüft genauso.
- Kein Token steht je in einem Werkzeugaufruf, einer MCP-Antwort oder einem Log.

## Referenzen

- `docs/konzept.md` — „Kommunikation“ (MCP über HTTP, zustandslos, Header, Host/Origin),
  „Ein Programm, zwei Rollen“ (config mit `listen`, local prüft genauso, Testaufbau),
  „Collections, Accounts, Rechte“, „Authentifizierung“ (Account-Zeilen, zwei Identitäten je
  Anfrage, `rotate` Schritt für Schritt, Sperren), „Datenmodell“ (`SYSTEM:A:`-Zeilen).
- `docs/vertrag.md` — aus Task 004; wird hier um `whoami` und `rotate` erweitert.
- `docs/begriffe.md`.
- `k-playbook-local/tasks/done/004-dokumente-abgleich-local.md` — Vertrag, Replica, `sync`
  über `local`.
- MCP-Go-SDK `github.com/modelcontextprotocol/go-sdk` (wie in k-playbook).

## Ziel

```text
Hub:
  kephalaion hub account add <name> [--description …]        → Einrichtungstoken, einmal
  kephalaion hub account list | show <name> | rm <name>
  kephalaion hub account set <name> --description …
  kephalaion hub account lock <name> | unlock <name>
  kephalaion hub account grant <name> <collection> [--write] [--supersede]
  kephalaion hub account revoke <name> <collection>
  kephalaion hub account token <name>                          → neues Einrichtungstoken
Node:
  kephalaion node hub add <alias> --transport local --node <name> --create
  kephalaion node hub check <alias>                            → whoami am Hub
  kephalaion node account rotate <hub> <account> (--token-file pfad | --token-stdin)
  kephalaion node sync [<alias>]                               → jetzt auch über http
Dienst:
  kephalaion serve
MCP (Node, /mcp):
  whoami
```

## Kontext

- **Accounts am Hub.** Was sich abgleichen muss, steht in `documents`: je Account und
  Collection eine Zeile `SYSTEM:A:<name>`, Inhalt JSON mit `hash` (sha256 hex) und
  `rights` (`write`, `supersede`; `read` ergibt sich aus der Zeile). Was nur der Hub braucht —
  Beschreibung, gesperrt, angelegt — steht in einer lokalen Tabelle `accounts`. Jede Änderung
  an den Zeilen ist ein Schreibvorgang mit Revision und Zeile in `actions` (`account` =
  `admin`, `subject` = Account bzw. `<account>:<collection>`). Namen gemeinsam mit den Nodes
  eindeutig, in beide Richtungen geprüft; Namensregel wie bei Nodes.
- **Sperren und Entziehen** über Löschmarken: `lock` setzt alle `SYSTEM:A:`-Zeilen des
  Accounts als Löschmarke und merkt die Rechte in `accounts`; `unlock` legt sie neu an.
  `revoke` setzt die eine Zeile als Löschmarke. `rm` entfernt den Account samt Zeilen
  (Löschmarken). `grant` auf eine vorhandene Zeile ändert die Rechte.
- **Einrichtungstoken:** Format und Anzeige wie beim Node (`keph_…`, einmal, nur Hash
  gespeichert). `hub account token` ersetzt den Hash in allen Zeilen. Ein Account ohne
  Collection hat keine Zeilen — sein Hash steht dann nur in `accounts` und wandert beim
  ersten `grant` in die Zeilen.
- **`serve`:** liest die config, öffnet je Rolle ihre Datenbank und lauscht auf `listen`.
  Beide Rollen in einem Prozess; ein Hub-Eintrag des Nodes mit `local` ruft den Hub dieses
  Prozesses als Funktion auf. Der Node lauscht nur auf Loopback (`127.0.0.1`, `::1`,
  `localhost`), sonst Abbruch beim Start; der Hub darf nach außen, wenn es in der config
  steht. Eine Sperre (`flock`) auf einer Datei neben jeder Datenbank verhindert einen
  zweiten `serve` auf derselben Rolle; `status` zeigt, ob `serve` läuft. Logs auf stderr,
  eine Zeile je Anfrage (Methode, Pfad, Status, Dauer, Node- bzw. Account-Name) — nie ein
  Token. SIGINT/SIGTERM beenden sauber mit Frist. Kein Abgleich im Hintergrund.
- **Hub `/v1/`:** POST mit JSON, Antwort gzip, wenn erbeten. Der Node meldet sich bei jedem
  Aufruf an: `X-Keph-Node: <name>` und `Authorization: Bearer <node-token>`. Fehler als
  HTTP-Status plus JSON `{"code": …, "message": …}`; Codes mindestens `unauthenticated`
  (Node unbekannt, Token falsch, gesperrt — dieselbe Antwort), `account_unauthenticated`,
  `invalid`, `unsupported_version`, `no_shared_collection`. Größenbegrenzung des Bodys,
  Zeitlimits. Die Fassung im Pfad (`v1`) ist die Fassung des Vertrags.
  - `whoami`: bestätigt den Node, liefert `hub_id`, Node-Name, erlaubte Collections; mit
    optionalem Account-Teil (Name, Token) zusätzlich, ob der Account gilt und welche seiner
    Collections dieser Node hält — für die Prüfung nach einem unklaren `rotate`.
  - `rotate`: Account-Name, altes Token, sha256 des neuen (64 hex). Prüft Node und altes
    Token (konstante Zeit), ersetzt den Hash in allen Zeilen des Accounts in **einer**
    Transaktion und liefert `hub_id` und die Zeilen, beschränkt auf die Collections dieses
    Nodes. Hat der Account keine davon: `no_shared_collection`, **vor** jeder Änderung. Ein
    gesperrter Account kann nicht rotieren.
  - `sync`: der Abgleich aus Task 004 über HTTP, unverändert im Vertrag.
- **Transport `http` am Node:** zweite Umsetzung der Schnittstelle aus Task 004; nur
  `http://localhost|127.0.0.1|[::1]`. `whoami` und `sync` dürfen bei Netzfehlern mit
  wachsendem Abstand wiederholen, `rotate` **nie** (nicht wiederholbar: danach gilt das alte
  Token nicht mehr). `https` und `ssh` melden weiter „noch nicht unterstützt“.
- **`node hub add --create`:** nur mit `--transport local`; legt am Hub dieser config den
  Node an (Fehler, wenn es ihn gibt), erzeugt das Token und schreibt es direkt in `node.db`,
  ohne Anzeige. Die Prüfung beim Aufruf bleibt dieselbe.
- **`node hub check`:** ruft `whoami` am Hub, merkt beim ersten Kontakt die `hub_id` (Regel
  aus Task 004 bei Abweichung) und zeigt Node-Name, erlaubte Collections, Erreichbarkeit.
- **`node account rotate`** — ein CLI-Kommando, **kein MCP-Werkzeug**: Das Token stünde sonst
  im Kontext der KI.
  - `--token-file pfad`: liest das alte Token, schreibt das neue **vor** dem Aufruf nach
    `pfad.pending` (`0600`), ersetzt nach Erfolg die Datei und löscht `.pending`. Scheitert
    der Aufruf eindeutig, bleibt die alte Datei und `.pending` wird gelöscht; ist der Ausgang
    unklar (Zeitüberschreitung), bleiben beide, mit Hinweis auf `node account check`.
  - `--token-stdin`: liest das alte Token, gibt das neue nach Erfolg einmal aus; bei unklarem
    Ausgang gibt es das neue ebenfalls aus, deutlich als „unklar — prüfen“ markiert.
  - Nach Erfolg schreibt der Node die gelieferten Zeilen in die Replica (legt sie an, wenn es
    sie noch nicht gibt) — der Account ist sofort bekannt, ohne Abgleich.
  - Dazu `kephalaion node account check <hub> <account> (--token-file | --token-stdin)`:
    `whoami` mit Account-Teil.
- **Node als MCP-Server** unter `/mcp` auf dem Node-`listen`, Streamable HTTP über das
  go-sdk; zustandslos, soweit das SDK es erlaubt. `Host` muss Loopback mit dem eigenen Port
  sein, `Origin` fehlt oder ist `http://localhost…`/`http://127.0.0.1…` — sonst 403
  (DNS-Rebinding).
  - **Header je Hub:** `X-Keph-Account-<alias>` und `X-Keph-Token-<alias>`; ein Client kann
    für mehrere Hubs je ein Paar schicken. Der Client trägt sie aus seiner MCP-Konfiguration
    ein, die KI sieht sie nicht.
  - **Prüfung je Anfrage** gegen die `SYSTEM:A:`-Zeilen der Replica des Hubs: Zeilen des
    Accounts über den Index, sha256 des vorgelegten Tokens, Vergleich in konstanter Zeit.
    Kein Cache. Unbekannt und falsch: dieselbe Antwort.
  - **Werkzeug `whoami`:** ohne Argumente; je Hub mit Header-Paar: angemeldet ja/nein, bei ja
    Account, Collections und Rechte. Nie ein Token, nie ein Hash.
- **Testaccounts:** Collections `team-x` und `privat`; `alice` (read team-x), `bob` (read +
  write team-x, read privat), `carol` (supersede team-x), Node `laptop` mit beiden
  Collections. Einmal über `local`, einmal über `http` auf `localhost`.
- **Nicht in diesem Task:** Abgleich im Hintergrund und Ereignisstrom, Suche, weitere
  MCP-Werkzeuge, Schreiben über den Node, `https`/`ssh`, Einrichtung als Dienst, Lauschen auf
  der Docker-Bridge, Verwaltung über MCP.

## Zu bauen

### Etappe 1 — Accounts am Hub

- Tabelle `accounts`, `SYSTEM:A:`-Zeilen, Store-Methoden und CLI `hub account …`; Export und
  Import nehmen `accounts` mit (die Zeilen liegen in `documents` und gehören nicht in den
  Export der Einstellungen — festhalten, wie ein Import mit ihnen umgeht).
- Tests: gemeinsame Eindeutigkeit mit Nodes (beide Richtungen), grant/revoke/lock/unlock/rm
  als Revisionen mit Löschmarken, Token-Ersatz in allen Zeilen, Account ohne Collection.

### Etappe 2 — Vertrag erweitert, Hub über HTTP

- `docs/vertrag.md` und das neutrale Paket um `whoami` und `rotate`; Umsetzung am Hub.
- HTTP-Handler `/v1/whoami|rotate|sync` über der Hub-Umsetzung; Anmeldung per Header,
  Fehlercodes, gzip, Grenzen.
- Tests des Vertrags gegen `local` und gegen HTTP (`httptest`): jeder Fehlercode, `rotate`
  ändert alle Zeilen in einer Revision, `no_shared_collection` ändert nichts, gesperrter
  Node/Account.

### Etappe 3 — Node: Transport http, check, --create, rotate

- HTTP-Umsetzung der Schnittstelle am Node; `node sync` über `http`.
- `node hub check`, `node hub add --create`, `node account rotate|check`.
- Tests: rotate über beide Transporte, Zeilen landen in der Replica, `.pending` in allen drei
  Ausgängen, keine Wiederholung bei `rotate`.

### Etappe 4 — serve

- Listener je Rolle, gemeinsamer Unterbau, Verdrahtung von `local`, Sperre, Logs, Beenden,
  Anzeige in `status`.
- Tests: Start mit Port 0, zweiter `serve` scheitert, Node auf Nicht-Loopback scheitert,
  Beenden per Signal, kein Token im Log.

### Etappe 5 — Node als MCP-Server

- `/mcp` mit dem go-sdk, Host/Origin-Prüfung, Header je Hub, Prüfung gegen die Replica,
  Werkzeug `whoami`.
- Tests mit dem MCP-Client des SDK: ohne Header, falsches Token, unbekannter Account, zwei
  Hubs, gesperrter Account nach `lock` + `sync`, falscher Host, fremder Origin; keine
  Tokens/Hashes in Antworten.

### Etappe 6 — Durchlauf und Doku

- Durchlauf mit den Testaccounts: `serve` starten; `rotate` für alice über `local` und für bob
  über `http`; `whoami` per MCP mit beiden; `lock carol` → `sync` → `whoami` scheitert.
- `README.md`: Einrichten mit Accounts, `serve`, `rotate`, Beispiel für `.mcp.json` mit
  Headern aus Umgebungsvariablen. `docs/vertrag.md` erweitert. `docs/begriffe.md`: neue
  Begriffe (Header, `check`, `--create`, Sperrdatei). `docs/konzept.md`: `rotate` ist ein
  CLI-Kommando des Nodes, nicht MCP; Header-Namen; nachziehen, wo die Umsetzung abweicht.
