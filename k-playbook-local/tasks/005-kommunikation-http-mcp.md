# Task 005 — Kommunikation: Accounts, serve, Hub über HTTP, Node als MCP-Server, rotate

Alle Beteiligten reden miteinander: Clients per MCP mit dem Node, der Node per HTTP (oder
`local`) mit dem Hub. Bewiesen wird es mit `whoami` auf jeder Strecke und mit `rotate`, das
einmal ganz durchläuft.

**Vorbedingung:** Task 004 liegt in `k-playbook-local/tasks/done/`. Sonst nicht beginnen,
sondern abbrechen.

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
  kephalaion node account check <hub> <account> (--token-file pfad | --token-stdin)
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
  Beschreibung, gesperrt, angelegt — steht in einer lokalen Tabelle `accounts`. **Den Hash
  führt `accounts` maßgeblich und immer** (auch gesperrt, auch ohne Collection); die Zeilen
  tragen eine Kopie für den Abgleich. Jede Änderung an den Zeilen ist ein Schreibvorgang mit
  Revision und Zeile in `actions` (`account` = `admin`, `subject` = Account bzw.
  `<account>:<collection>`; außer `rotate`, siehe dort). Namen gemeinsam mit den Nodes eindeutig, geprüft über die
  Tabellen `accounts` ↔ `nodes` in beide Richtungen; der bestehende `checkNodeNameFree`
  (heute über `SYSTEM:A:`-Zeilen samt Löschmarken) wird darauf umgestellt. Nach
  `hub account rm` ist der Name wieder frei; die Löschmarken bleiben, ein neuer Account
  gleichen Namens schreibt neue Revisionen darüber. Namensregel wie bei Nodes; `admin` ist
  als Account- und Node-Name reserviert, er steht im Protokoll für den Verwalter — eine eigene
  Prüfung in `ident` für diese beiden Namensarten, `ident.CheckName` bleibt unverändert
  (Collections und Hub-Aliase dürfen `admin` heißen).
- **Sperren und Entziehen** über Löschmarken: `lock` setzt alle `SYSTEM:A:`-Zeilen des
  Accounts als Löschmarke und merkt die Rechte in `accounts`; `unlock` legt sie neu an.
  `revoke` setzt die eine Zeile als Löschmarke. `rm` entfernt den Account samt Zeilen
  (Löschmarken). `grant` setzt die Rechte der Zeile vollständig (legt sie an oder ändert
  sie): ohne `--write` wird `write` entzogen, ohne `--supersede` ebenso `supersede`. Beide
  sind unabhängig (`write` = Eigenes, `supersede` = Fremdes).
- **Einrichtungstoken:** Format und Anzeige wie beim Node (`keph_…`, einmal, nur Hash
  gespeichert). `grant`, `unlock`, `hub account token` und `rotate` schreiben den Hash in
  `accounts` und in die Zeilen in **derselben** Transaktion; neue Zeilen übernehmen ihn aus
  `accounts`.
- **`serve`:** liest die config, öffnet je Rolle ihre Datenbank und lauscht auf `listen`.
  Beide Rollen in einem Prozess. In diesem Task lauschen Node **und** Hub nur auf Loopback
  (`127.0.0.1`, `::1`, `localhost`), sonst Abbruch beim Start — Klartext-HTTP verlässt den
  Rechner nicht, bis `https`/`ssh` kommen. Eine Sperre (`flock`) auf einer Datei neben jeder
  Datenbank verhindert einen zweiten `serve` auf derselben Rolle; sie betrifft nur `serve`,
  CLI-Kommandos laufen daneben über SQLite. `status` prüft die Sperre nicht blockierend und
  zeigt, ob `serve` läuft. Logs auf stderr, eine Zeile je Anfrage (Methode, Pfad, Status,
  Dauer, Node- bzw. Account-Name — nur Namen, die `ident.CheckName` bestehen, sonst
  maskiert) — nie ein Token. SIGINT/SIGTERM beenden sauber mit Frist. Kein Abgleich im
  Hintergrund.
- **Hub `/v1/`:** POST mit JSON, Antwort gzip, wenn erbeten. Der Node meldet sich bei jedem
  Aufruf an: `X-Keph-Node: <name>` und `Authorization: Bearer <node-token>`. Fehler als
  HTTP-Status plus JSON `{"code": …, "message": …}`; Codes mindestens `unauthenticated`
  (Node unbekannt, Token falsch, gesperrt — dieselbe Antwort), `account_unauthenticated`,
  `invalid`, `unsupported_version`, `no_shared_collection`. Bei unbekanntem Node- oder
  Account-Namen wird gegen einen Dummy-Hash verglichen (gleiche Laufzeit). Größenbegrenzung
  nur für Anfrage-Bodys; Zeitlimits so, dass eine große `sync`-Seite (Task 004) durchgeht.
  Die Fassung im Pfad (`v1`) ist die Fassung des Vertrags. `docs/vertrag.md` legt fest: die
  Abbildung Code → HTTP-Status, die Antwort auf eine fremde Fassung (`/v2/…`), die
  bezifferten Grenzen.
  - `whoami`: bestätigt den Node, liefert `hub_id`, Node-Name, erlaubte Collections; mit
    optionalem Account-Teil (Name, Token) zusätzlich, ob der Account gilt (geprüft gegen
    `accounts`, gesperrt gilt nicht) und welche seiner Collections dieser Node hält — für
    die Prüfung nach einem unklaren `rotate`.
  - `rotate`: Account-Name, altes Token, sha256 des neuen (64 hex). Prüft Node und altes
    Token gegen `accounts` (konstante Zeit), ersetzt den Hash in `accounts` und allen Zeilen
    des Accounts in **einer** Transaktion — eine Revision mit **einer** Zeile in `actions`
    (`account` = der Account, `carrier` = Node, `subject` = Account, Aktion `rotate`) — und liefert `hub_id` und die
    Zeilen, beschränkt auf die Collections dieses Nodes. Hat der Account keine davon:
    `no_shared_collection`, **vor** jeder Änderung. Ein gesperrter Account kann nicht
    rotieren. Fehlversuche nur ins Log (ohne Token), nicht in `actions`.
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
    unklar (Zeitüberschreitung), bleiben beide, mit Hinweis auf `node account check`. Ein
    vorhandenes `pfad.pending` blockiert einen neuen `rotate`, bis `check` gelaufen ist.
  - `--token-stdin`: liest das alte Token, gibt das neue nach Erfolg einmal aus; bei unklarem
    Ausgang gibt es das neue ebenfalls aus, deutlich als „unklar — prüfen“ markiert.
  - Nach Erfolg zuerst Token-Datei ersetzen bzw. Token ausgeben, **danach** die gelieferten
    Zeilen in die Replica schreiben (legt sie an, wenn es sie noch nicht gibt) — der Account
    ist sofort bekannt, ohne Abgleich. Nur Zeilen der gewünschten Collections; bleibt keine
    übrig, meldet der Node es. Bei abweichender `hub_id` gilt die Regel aus Task 004. Ein
    Fehler beim Schreiben der Replica wird gemeldet („`node sync` nachholen“) und kippt den
    Erfolg nicht.
  - Dazu `kephalaion node account check <hub> <account> (--token-file | --token-stdin)`:
    `whoami` mit Account-Teil. Existiert `pfad.pending`, prüft es beide Tokens: gilt das
    neue, ersetzt es die Datei und löscht `.pending`; gilt das alte, löscht es `.pending`;
    gilt keines, bleiben beide, mit Meldung.
- **Node als MCP-Server** unter `/mcp` auf dem Node-`listen`, Streamable HTTP über das
  go-sdk; zustandslos, soweit das SDK es erlaubt. `Host` muss Loopback mit dem eigenen Port
  sein, `Origin` fehlt oder ist `http://localhost…`/`http://127.0.0.1…` — sonst 403
  (DNS-Rebinding).
  - **Header je Hub:** `X-Keph-Account-<alias>` und `X-Keph-Token-<alias>`; ein Client kann
    für mehrere Hubs je ein Paar schicken. Der Client trägt sie aus seiner MCP-Konfiguration
    ein, die KI sieht sie nicht.
  - **Prüfung je Anfrage** gegen die `SYSTEM:A:`-Zeilen der Replica des Hubs: Zeilen des
    Accounts über den Index, sha256 des vorgelegten Tokens, Vergleich in konstanter Zeit;
    bei unbekanntem Account gegen einen Dummy-Hash. Kein Cache. Unbekannt und falsch:
    dieselbe Antwort.
  - **Werkzeug `whoami`:** ohne Argumente; je Hub mit Header-Paar: angemeldet ja/nein, bei ja
    Account, Collections und Rechte. Nie ein Token, nie ein Hash. Transport und `initialize`
    gehen ohne Anmeldung; ein Header-Paar für einen unbekannten Alias ergibt „nein“ für
    diesen Alias, keine Fehlerantwort. Spätere Werkzeuge verlangen eine gültige Anmeldung.
- **Testaccounts:** Collections `team-x` und `privat`; `alice` (read team-x), `bob` (read +
  write team-x, read privat), `carol` (supersede team-x), Node `laptop` mit beiden
  Collections. Einmal über `local`, einmal über `http` auf `localhost`.
- **Nicht in diesem Task:** Abgleich im Hintergrund und Ereignisstrom, Suche, weitere
  MCP-Werkzeuge, Schreiben über den Node, `https`/`ssh`, Hub außerhalb von Loopback,
  Verdrahtung von `local` innerhalb von `serve` (kein Aufrufer ohne Hintergrundabgleich),
  Begrenzung von Fehlversuchen, Einrichtung als Dienst, Lauschen auf der Docker-Bridge,
  Verwaltung über MCP.

## Zu bauen

### Etappe 1 — Accounts am Hub

- Tabelle `accounts`, `SYSTEM:A:`-Zeilen, Store-Methoden und CLI `hub account …`;
  `checkNodeNameFree` auf `accounts` umstellen, `admin` für Account- und Node-Namen
  reservieren. Export nimmt je Account Beschreibung, gesperrt, Hash und die Rechte je
  Collection mit (Exportformat anheben, wie in Task 004). Import gleicht die
  `SYSTEM:A:`-Zeilen an den Export an — vorhandene ändern, fehlende als Löschmarke, neue
  anlegen —, in einer Revision und derselben Transaktion wie der übrige Import, mit denselben
  Prüfungen wie die CLI. Kein stilles Leeren (Regel aus Task 003): fehlt der Accounts-Teil,
  bricht der Import ab; nur ein ausdrücklich leerer Teil leert.
- Tests: gemeinsame Eindeutigkeit mit Nodes (beide Richtungen), Name nach `rm` wieder frei,
  `admin` als Account- und Node-Name abgelehnt, grant/revoke/lock/unlock/rm als Revisionen
  mit Löschmarken, `grant` ohne `--write` entzieht `write`, Hash in `accounts` und allen
  Zeilen gleich nach `grant`/`unlock`/`token`, Account ohne Collection, Export/Import mit
  Rechten.

### Etappe 2 — Vertrag erweitert, Hub über HTTP

- `docs/vertrag.md` und das neutrale Paket um `whoami` und `rotate`; Umsetzung am Hub.
- HTTP-Handler `/v1/whoami|rotate|sync` über der Hub-Umsetzung; Anmeldung per Header,
  Fehlercodes, gzip, Grenzen.
- Tests des Vertrags gegen `local` und gegen HTTP (`httptest`): jeder Fehlercode mit seinem
  HTTP-Status, fremde Fassung, zu großer Body, `rotate` ändert `accounts` und alle Zeilen in
  einer Revision mit `actions`-Zeile, `no_shared_collection` ändert nichts, gesperrter
  Node/Account.

### Etappe 3 — Node: Transport http, check, --create, rotate

- HTTP-Umsetzung der Schnittstelle am Node; `node sync` über `http`.
- `node hub check`, `node hub add --create`, `node account rotate|check`.
- Tests: rotate über beide Transporte, Zeilen landen in der Replica (nur gewünschte
  Collections), `.pending` in allen drei Ausgängen, `.pending` blockiert `rotate`, `check`
  mit `.pending` (neues gilt, altes gilt), Replica-Fehler nach Erfolg kippt ihn nicht (Datei
  ist ersetzt), keine Wiederholung bei `rotate`.

### Etappe 4 — serve

- Listener je Rolle, gemeinsamer Unterbau, Sperre, Logs, Beenden, Anzeige in `status`.
- Tests: Start mit Port 0, zweiter `serve` scheitert, CLI-Kommando neben laufendem `serve`
  geht, Node oder Hub auf Nicht-Loopback scheitert, Beenden per Signal, kein Token im Log,
  ungültiger Name im Log maskiert.

### Etappe 5 — Node als MCP-Server

- `/mcp` mit dem go-sdk, Host/Origin-Prüfung, Header je Hub, Prüfung gegen die Replica,
  Werkzeug `whoami`.
- Tests mit dem MCP-Client des SDK: ohne Header (`initialize` geht, `whoami`: nein),
  falsches Token und unbekannter Account (nein, gleiche Antwort), Header für unbekannten
  Alias (nein für diesen Alias, kein Fehler), zwei Hubs (je Hub getrennt), gesperrter
  Account nach `lock` + `sync` (nein), falscher Host und fremder Origin (403); keine
  Tokens/Hashes in Antworten.

### Etappe 6 — Durchlauf und Doku

- Durchlauf mit den Testaccounts: `serve` starten; `rotate` für alice über `local` und für bob
  über `http` (für `http` ein eigener Node mit angezeigtem Token aus `hub node add`, weil
  `--create` das Token von `laptop` nicht anzeigt und ein neues es für `local` ungültig machte); `whoami` per MCP mit beiden; `lock carol` → `sync` → `whoami` scheitert.
- `README.md`: Einrichten mit Accounts, `serve`, `rotate`, Beispiel für `.mcp.json` mit
  Headern aus Umgebungsvariablen. `docs/vertrag.md` erweitert. `docs/begriffe.md`: neue
  Begriffe (Header, `check`, `--create`, Sperrdatei). `docs/konzept.md`: `rotate` ist ein
  CLI-Kommando des Nodes, nicht MCP; Header-Namen; eigene Tabelle `accounts` (statt „keine
  eigene Tabelle“); `grant` statt `account add --scope`; `lock` merkt die Rechte;
  Klartext-`http` nur auf Loopback; „Lokale Tabellen“/Export (Accounts samt Rechten im
  Export, Import gleicht `SYSTEM:A:`-Zeilen an); nachziehen, wo die Umsetzung sonst abweicht.
  `k-playbook-local/k-playbook.md`: Accounts, Namenseindeutigkeit (über `accounts` ↔
  `nodes`, nicht mehr über `SYSTEM:A:`-Zeilen), Transport `http`, `serve` und Sperrdatei.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — Accounts am Hub | erledigt | 2026-09-26 | Tabelle `accounts` (Schema 3), `hub account …`, Export-Format 4, `admin` reserviert; make check grün |
| 2 — Vertrag erweitert, Hub über HTTP | erledigt | 2026-09-26 | `whoami`/`rotate` in Vertrag und `replication`, `internal/contract/httpapi` (Handler, Client), Vertragstests gegen local und HTTP; make check grün |
| 3 — Node: Transport http, check, --create, rotate | offen | | |
| 4 — serve | offen | | |
| 5 — Node als MCP-Server | offen | | |
| 6 — Durchlauf und Doku | offen | | |

---
## Review-Log (2026-09-25)

**Pfad:** k-playbook-local/tasks/005-kommunikation-http-mcp.md
**Intent:** inline (`## Intent`)
**Runden:** 2

### Diskussion
- **Hub nach außen (FEHLER-02):** Der Task erlaubte dem Hub, nach außen zu lauschen. Das Konzept verlangt für Node → Hub aber TLS oder SSH, und der Node-Transport `http` nimmt ohnehin nur Loopback an. Entscheidung: In diesem Task lauscht auch der Hub nur auf Loopback.
- **Quelle des Token-Hashes (FEHLER-01):** Der Hash stand mal in `accounts`, mal in den Zeilen, und `lock` löscht die Zeilen. Entscheidung: `accounts` führt den Hash maßgeblich, die Zeilen tragen eine Kopie. Alle Änderungen schreiben beides in einer Transaktion.
- **Split des Tasks (WARNUNG-01):** Der Critic riet zu 005a/b. Der Moderator bleibt bei einem Task, weil die Etappen Haltepunkte sind und `/k-task-run` einen Teillauf stehen lässt.
- **Import (R2-1):** Der Editor-Text „legt Zeilen neu an“ kollidierte mit der Ersetz-Semantik aus 003 und dem Unique-Index. Entscheidung: Der Import gleicht die Zeilen an. Ein fehlender Teil bricht ab (kein stilles Leeren).

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| FEHLER-01 | FEHLER | 005 | Accounts / Einrichtungstoken / Sperren | Token-Hash ohne eindeutige Quelle | `accounts` maßgeblich, Zeilen Kopie |
| FEHLER-02 | FEHLER | 005 | serve | Hub nach außen per Klartext-HTTP, widerspricht Konzept | Hub nur Loopback |
| FEHLER-03 | FEHLER | 005 | node account rotate | Reihenfolge Token/Replica nach Erfolg offen | erst Token, dann Replica |
| WARNUNG-01 | WARNUNG | 005 | Umfang | 6 große Etappen | in 005a/b teilen |
| WARNUNG-02 | WARNUNG | 005 | Referenzen | 004 noch offen | Vorbedingung |
| WARNUNG-03 | WARNUNG | 005 | Etappe 1 Export/Import | Designentscheidung offen gelassen | Rechte exportieren, Import legt Zeilen an |
| WARNUNG-04 | WARNUNG | 005 | rotate am Node | erlaubt vs. gewünscht; `hub_id` | nur gewünschte, Regel aus 004 |
| WARNUNG-05 | WARNUNG | 005 | --token-file / check | `.pending` bleibt Handarbeit | `check` löst `.pending` auf |
| WARNUNG-06 | WARNUNG | 005 | MCP | Soll ohne Header unklar | festlegen, Soll je Test |
| WARNUNG-07 | WARNUNG | 005 | /v1/ Grenzen | kollidiert mit großen sync-Seiten | nur Anfrage-Body begrenzen |
| WARNUNG-08 | WARNUNG | 005 | Etappe 6 | Doku-Nachzug unvollständig | Punkte aufzählen, k-playbook.md |
| WARNUNG-09 | WARNUNG | 005 | Eindeutigkeit | `checkNodeNameFree` über Zeilen | über `accounts` ↔ `nodes` |
| WARNUNG-10 | WARNUNG | 005 | Header je Hub | Header-Kanonisierung, `_` | Zuordnung festlegen |
| WARNUNG-11 | WARNUNG | 005 | grant | setzt oder ergänzt? supersede/write | festlegen |
| FEHLEND-01 | FEHLEND | 005 | Namensregel | `admin` im Protokoll mehrdeutig | reservieren |
| FEHLEND-02 | FEHLEND | 005 | rotate | `actions`-Eintrag fehlt | ergänzen |
| FEHLEND-03 | FEHLEND | 005 | Prüfung | Timing bei unbekanntem Namen, Fehlversuche | Dummy-Hash, Begrenzung zurückstellen |
| FEHLEND-04 | FEHLEND | 005 | Replica | Index `documents_system` | festhalten |
| FEHLEND-05 | FEHLEND | 005 | serve | flock vs. CLI, local-Verdrahtung ohne Aufrufer | klarstellen, zurückstellen |
| FEHLEND-06 | FEHLEND | 005 | Fehlercodes | Code → Status, fremde Fassung, Log-Injection | in vertrag.md; Namen prüfen |
| R2-1 | Widerspruch | 005 | Etappe 1 Import | „neu anlegen“ vs. Ersetzen/kein stilles Leeren aus 003 | angleichen, Format anheben, Doku |
| R2-2 | Unklarheit | 005 | admin in ident | `CheckName` gilt auch für Collections/Aliase | eigene Prüfung |
| R2-3 | Unklarheit | 005 | actions bei rotate | liest sich wie Widerspruch zur allgemeinen Regel | Ausnahme benennen |
| R2-H | Hinweis | 005 | Etappe 6 | `laptop` hat für `http` kein Token (`--create` zeigt es nicht) | zweiter Node |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| FEHLER-01 | decide + pass | Sicherheitskern, eindeutig lösbar | gefixt |
| FEHLER-02 | decide + pass | Konzept verlangt TLS/SSH | gefixt |
| FEHLER-03 | pass | Aussperrgefahr | gefixt |
| WARNUNG-01 | skip | Etappen sind Haltepunkte | offen gelassen |
| WARNUNG-02 | pass | blockiert Ausführung | gefixt |
| WARNUNG-03 | decide + pass | Datenverlust bei Schema-Neuanlage | gefixt |
| WARNUNG-04 | pass | Replica sonst inkonsistent | gefixt |
| WARNUNG-05 | pass | rotate-Sicherheit | gefixt |
| WARNUNG-06 | decide + pass | Tests ohne Soll | gefixt |
| WARNUNG-07 | pass | sync scheiterte sonst | gefixt |
| WARNUNG-08 | pass | Doku-Drift | gefixt |
| WARNUNG-09 | decide + pass | bestehender Code greift nicht | gefixt |
| WARNUNG-10 | skip | vom Ausführenden lösbar | offen gelassen |
| WARNUNG-11 | decide + pass | Semantik nötig für Tests | gefixt |
| FEHLEND-01 | pass | billig, Protokoll eindeutig | gefixt |
| FEHLEND-02 | pass | Audit-Lücke | gefixt |
| FEHLEND-03 | decide + pass | Timing-Leck | gefixt |
| FEHLEND-04 | skip | Replica „wie am Hub“, Ausführender prüft | offen gelassen |
| FEHLEND-05 | decide + pass | nicht testbar | gefixt |
| FEHLEND-06 | pass | Vertrag unvollständig | gefixt |
| R2-1 | decide | klar lösbar, Regel aus 003 | Moderator-Edit |
| R2-2 | decide | klar lösbar | Moderator-Edit |
| R2-3 | decide | Klarstellung | Moderator-Edit |
| R2-H | decide | Durchlauf sonst nicht ausführbar | Moderator-Edit |
| Alignment-Hinweis | decide | `node account check` fehlte in „Ziel“ | Moderator-Edit |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| FEHLER-01 | fixed | `accounts` maßgeblich; grant/unlock/token/rotate schreiben beides; Prüfung gegen `accounts` |
| FEHLER-02 | fixed | Node und Hub nur Loopback; in „Nicht in diesem Task“ |
| FEHLER-03 | fixed | erst Token, dann Replica; Fehler kippt Erfolg nicht; Test |
| WARNUNG-02 | fixed | Vorbedingung vor Intent |
| WARNUNG-03 | fixed | Export-/Importregel in Etappe 1 |
| WARNUNG-04 | fixed | nur gewünschte Collections, `hub_id`-Regel |
| WARNUNG-05 | fixed | `check` löst `.pending` auf, ergänzt „gilt keines, bleiben beide“ |
| WARNUNG-06 | fixed | Soll festgelegt, Soll je Test in Etappe 5 |
| WARNUNG-07 | fixed | nur Anfrage-Bodys, Grenzen in vertrag.md |
| WARNUNG-08 | fixed | Etappe 6 erweitert |
| WARNUNG-09 | fixed | Eindeutigkeit über Tabellen, Name nach rm frei |
| WARNUNG-11 | fixed | grant setzt vollständig, write/supersede unabhängig |
| FEHLEND-01 | fixed | `admin` reserviert |
| FEHLEND-02 | fixed | `actions`-Zeile für rotate |
| FEHLEND-03 | fixed | Dummy-Hash; Begrenzung zurückgestellt |
| FEHLEND-05 | fixed | Sperre nur serve; local-Verdrahtung zurückgestellt |
| FEHLEND-06 | fixed | Code → Status, fremde Fassung, Log-Namen |

### Moderator-Entscheidungen
- WARNUNG-01 (Split) übersprungen: Die Etappen tragen, und ein Teillauf bleibt über `/k-task-run` sichtbar.
- WARNUNG-10 übersprungen: Header-Kanonisierung ist Umsetzungsdetail.
- FEHLEND-04 übersprungen: Die Replica aus 004 entsteht wie am Hub, den Index prüft der Ausführende.
- R2-1: Der Import gleicht die `SYSTEM:A:`-Zeilen an. Fehlt der Accounts-Teil, bricht er ab (Regel aus 003, abweichend vom Critic-Vorschlag „unberührt lassen“). Das Exportformat wird angehoben.
- R2-2: eigene Prüfung für Account- und Node-Namen, `ident.CheckName` bleibt.
- R2-3: Ausnahme für `rotate` im allgemeinen Satz benannt, `carrier` = Node, eine `actions`-Zeile.
- R2-H: Der Durchlauf über `http` nutzt einen eigenen Node mit angezeigtem Token.
- Alignment-Hinweis: `node account check` im Block „Ziel“ ergänzt.

### Intent-Alignment
Ja: Jeder Intent-Punkt hat eine Etappe mit Tests (serve, `/v1/` gegen local und HTTP, MCP `whoami` gegen die Replica, Prüfung auf jedem Weg, keine Tokens in Antworten und Logs). Der Durchlauf in Etappe 6 beweist alle Strecken.

### Geänderte Dateien
- 005-kommunikation-http-mcp.md: Vorbedingung, Hash-Quelle, Loopback-Hub, Reihenfolge und `.pending` bei rotate, Import/Export der Accounts, Eindeutigkeit, grant-Semantik, `admin`, actions, Dummy-Hash, MCP-Soll, Grenzen, serve-Sperre, Doku-Nachzug, Durchlauf mit zweitem Node (FEHLER-01–03, WARNUNG-02–09/11, FEHLEND-01–03/05/06, R2-1–3, R2-H)

### Offen (nicht gefixt)
- WARNUNG-01: kein Split, bewusst
- WARNUNG-10: Header-Kanonisierung dem Ausführenden überlassen
- FEHLEND-04: Replica-Index prüft der Ausführende
