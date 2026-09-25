# Begriffe

Verbindliche Namen. Begriffe sind englisch — in Code, Konfiguration, Befehlen und Werkzeugen;
die Dokumentation ist vorerst deutsch. Wer einen neuen Begriff braucht, trägt ihn hier ein,
bevor er ihn benutzt.

Ausführlich: [`konzept.md`](konzept.md).

## Programm und Rollen

- **kephalaion** — Produkt, Paket, Binary. Kurzform im Gespräch: Keph.
- **serve** — `kephalaion serve`, der Dienst. Trägt die Rollen, die in der Konfiguration
  stehen: `hub:`, `node:` oder beide in einem Prozess.
- **client** (MCP-Client) — was per MCP mit dem Node redet: Claude Code, Cursor, OpenCode,
  k-playbook. Für ihn ist der Node der MCP-Server. Die KI im Client sieht Name und Token nicht.
- **node** (Knoten) — Rolle, Abschnitt `node:`. Einmal je Rechner. MCP-Server über HTTP
  für Clients, Client eines oder mehrerer Hubs. Hält Replica, Index, Suche,
  Revisionen.
- **hub** (Zentrale) — Rolle, Abschnitt `hub:`. Einmal je Installation. Hält Store, Journal,
  Accounts, Token. Einziger Schreiber. Kein MCP, sucht nicht. Eigene Datenbank, getrennt von
  der Replica eines Nodes im selben Prozess.
- **transport** — wie ein Node einen Hub erreicht: `https`, `ssh` (dasselbe HTTP, getunnelt)
  oder `local` (Funktionsaufruf im selben Prozess, mit denselben Prüfungen).
- **bridge** (Brücke) — *zurückgestellt.* Wäre der Prozess, den ein Client über stdio
  startet, und reichte an den Node weiter. Nur falls ein Client zwingend stdio braucht.

## Daten

- **store** (Bestand) — alle Collections eines Hubs.
- **address** (Adresse) — `<hub>:<collection>`, wie Node und Clients eine Collection
  ansprechen. Den Hub-Namen vergibt der Node.
- **collection** (Sammlung) — unabhängige Einheit des Stores, gemeint ist der Inhalt, nicht
  der Speicherort. Keine Überschneidung mit anderen. Einheit für Rechte und Abgleich. Ein Hub
  hat mehrere. Ersetzt den Begriff „Bereich“ aus dem Konzept.
- **document** (Dokument) — eine Datei im Store. Stabile `id`; der Pfad ist nur ein Merkmal.
- **id** (Kennung) — stabile Kennung eines Dokuments, vom Hub vergeben.
- **journal** (Journal) — fortlaufende Folge der Änderungen eines Hubs.
- **revision** (Stand) — fortlaufende Nummer je Hub über alle Collections, vergeben beim
  Schreiben. Der Node merkt sich je Collection die letzte und fragt „alles seit Revision X“.
- **replica** (Kopie) — der Ausschnitt des Stores auf einem Node. Abgeleitet, nie selbst
  beschrieben.

## Zugriff

- **account** (Konto) — wer zugreift: Mensch, KI oder Programm. Hat einen Namen.
- **token** — Geheimnis eines Accounts, Format `keph_<geheimnis>`. Jede Anfrage trägt
  Account-Name und Token. Gespeichert wird nur der Hash. Unabhängig vom Transport.
- **scope** — ein Recht eines Accounts auf einer Collection, geschrieben
  `<collection>:<recht>`. Ein Account hat mehrere. Gespeichert je Collection in der
  Account-Zeile. Rechte:
  - `read` — hat jeder in der Collection eingetragene Account.
  - `write` — anlegen; Eigenes ändern und löschen; gelöschte Namen neu anlegen.
  - `supersede` — Fremdes ändern, ablösen, löschen.
  - `replicate` — nur Nodes: Inhalt und Account-Zeilen der Collection abgleichen.
- **SYSTEM:** — reservierter Präfix im Namen eines Dokuments, nur der Hub schreibt ihn.
  `SYSTEM:A:<account>` ist die Zeile eines Accounts in einer Collection.
- **carrier** (Träger) — der Node, der eine Anfrage an den Hub trägt; meldet sich mit seinem
  eigenen Token an. Das Token des Accounts steht in der Anfrage.
- **rotate** — ersetzt das Token eines Accounts: altes Token zur Anmeldung, Hash des neuen.
  Erster Vorgang jedes Accounts; eine eigene Begrüßung gibt es nicht.
