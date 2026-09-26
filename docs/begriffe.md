# Begriffe

Verbindliche Namen. Begriffe sind englisch — in Code, Konfiguration, Befehlen und Werkzeugen;
die Dokumentation ist vorerst deutsch. Wer einen neuen Begriff braucht, trägt ihn hier ein,
bevor er ihn benutzt.

Ausführlich: [`konzept.md`](konzept.md).

## Programm und Rollen

- **kephalaion** — Produkt, Paket, Binary. Kurzform im Gespräch: Keph.
- **serve** — `kephalaion serve`, der Dienst. Trägt die Rollen, die in der Konfiguration
  stehen: `hub:`, `node:` oder beide in einem Prozess. Keine eigene Rolle und kein eigener
  Eintrag in der config, sondern der eine Aufruf, der nicht endet: Er lauscht auf Ports (MCP
  für Clients, HTTP für Nodes), gleicht im Hintergrund ab und hält den Index warm. Alle
  anderen Kommandos sind kurze Aufrufe und arbeiten ohne ihn direkt auf der Datenbank.
- **config** — `~/.config/kephalaion/config.yaml`. Sagt nur, welche Rollen eingerichtet sind,
  wo ihre Datenbank liegt (`db:`) und wo ihr Dienst lauscht (`listen:`). Alles andere steht
  in der Datenbank der Rolle.
  `kephalaion config show` zeigt sie samt den `settings` je Rolle.
  - **export** — `kephalaion config export`: sichert config, `settings` und lokale Tabellen
    je Rolle als YAML mit einer Fassung des Formats (`format: 4`, darin `tables:`; am Hub
    auch die Accounts samt Rechten), getrennt von den Inhalten; ohne `db_info` und `actions`.
  - **import** — `kephalaion config import <datei>`: schreibt einen Export in bereits
    eingerichtete Rollen und ersetzt dort je Rolle `settings` und lokale Tabellen; ein
    Export im Format 1 ersetzt nur die `settings`, einer vor Format 4 lässt die Accounts. Am
    Hub gleicht er die `SYSTEM:A:`-Zeilen an die Accounts des Exports an, unter einer
    Revision. Die config bleibt unverändert.
- **db address** (db-Adresse) — der Wert von `db:` in der config: `sqlite:///<absoluter
  Pfad>`; `postgres://…` ist vorgesehen.
- **settings** (Einstellungen) — Tabelle `settings (key, value)` in der Datenbank jeder Rolle:
  alles, was nicht in der config steht. Wird mit `config export` gesichert.
- **db_info** — Tabelle `db_info (key, value)` in der Datenbank jeder Rolle und in jeder
  Replica. Hält die Schemafassung (`schema_version`), die Rolle (`role`: `hub`, `node` oder
  `replica`), die Anlagezeit (`created_at`), am Hub auch die Revision (`revision`), in der
  Replica die `hub_id`. Passt Fassung oder Rolle nicht, wird die Datenbank nicht benutzt.
- **init** — `kephalaion hub init`, `kephalaion node init`: richtet eine Rolle ein — Datenbank,
  Schema, Abschnitt in der config. Nur `init` legt eine Datenbank an; einzige Ausnahme ist die
  Replica, die der erste `sync` anlegt.
- **status** — `kephalaion status`: welche Rollen eingerichtet sind, wo ihre Datenbank liegt,
  welche Verbindungen bestehen.
- **client** (MCP-Client) — was per MCP mit dem Node redet: Claude Code, Cursor, OpenCode,
  k-playbook. Für ihn ist der Node der MCP-Server. Die KI im Client sieht Name und Token nicht.
- **node** (Knoten) — Rolle, Abschnitt `node:`. Einmal je Rechner. MCP-Server über HTTP
  für Clients, Client eines oder mehrerer Hubs. Hält Replica, Index, Suche,
  Revisionen.
- **hub** (Zentrale) — Rolle, Abschnitt `hub:`. Einmal je Installation. Hält Store, Journal,
  Accounts, Token. Einziger Schreiber. Kein MCP, sucht nicht. Eigene Datenbank, getrennt von
  der Replica eines Nodes im selben Prozess.
- **transport** — wie ein Node einen Hub erreicht: `https`, `ssh` (dasselbe HTTP, getunnelt)
  oder `local` (Funktionsaufruf im selben Prozess, mit denselben Prüfungen). Dazu `http` ohne
  TLS, nur für `localhost` — zum Testen des HTTP-Wegs auf einem Rechner.
- **bridge** (Brücke) — *zurückgestellt.* Wäre der Prozess, den ein Client über stdio
  startet, und reichte an den Node weiter. Nur falls ein Client zwingend stdio braucht.

## Daten

- **store** (Bestand) — alle Collections eines Hubs.
- **address** (Adresse) — `<hub>:<collection>`, wie Node und Clients eine Collection
  ansprechen. Den Hub-Namen vergibt der Node.
- **collection** (Sammlung) — unabhängige Einheit des Stores, gemeint ist der Inhalt, nicht
  der Speicherort. Keine Überschneidung mit anderen. Einheit für Rechte und Abgleich. Ein Hub
  hat mehrere. Ersetzt den Begriff „Bereich“ aus dem Konzept.
- **document** (Dokument) — eine Datei im Store. Stabile `id`; der Name ist sein Pfad und
  kann sich ändern.
- **name** (Name) — Pfad eines Dokuments in seiner Collection, Segmente durch `/` getrennt;
  eindeutig je Collection. Verzeichnisse gibt es nur als Präfix vorhandener Namen. Regeln in
  `konzept.md`, „Datenmodell“. Einzige Ausnahme: `SYSTEM:`-Zeilen.
- **deleted** (Löschmarke) — ein gelöschtes Dokument: Die Zeile bleibt mit `deleted = 1`, ohne
  Inhalt und mit neuer Revision, damit der Abgleich davon erfährt. Der Name ist danach wieder
  frei; eine Neuanlage bekommt eine neue `id`.
- **doc** — `kephalaion hub doc put|get|list|rm`: Dokumente am Hub. `put` legt an oder
  ersetzt (Admin-Upsert; unveränderter Inhalt zählt keine Revision), `rm` setzt eine
  Löschmarke. Am Node liest `kephalaion node doc list|get <hub>:<collection> …` aus der
  Replica, ohne Löschmarken und `SYSTEM:`-Zeilen.
- **hub import** — `kephalaion hub import <collection> <verzeichnis>`: spielt ein Verzeichnis
  als Dokumente ein, Name = relativer Pfad; ein Schreibvorgang, eine Revision. Nicht zu
  verwechseln mit `config import`.
- **mask** (Maske) — Glob auf das letzte Segment eines Namens (`*.md`, `0*-*.md`), etwa bei
  `list`; kein regulärer Ausdruck.
- **tool** (Werkzeug) — ein MCP-Werkzeug des Nodes für Clients. Gesammelt in `konzept.md`,
  „Werkzeuge“.
- **id** (Kennung) — stabile Kennung eines Dokuments, vom Hub vergeben.
- **journal** (Journal) — fortlaufende Folge der Änderungen eines Hubs.
- **revision** (Stand) — fortlaufende Nummer je Hub über alle Collections, vergeben beim
  Schreiben. Der Node merkt sich je Collection die letzte und fragt „alles seit Revision X“.
- **replica** (Kopie) — der Ausschnitt des Stores auf einem Node: je Hub-Eintrag eine eigene
  SQLite-Datei `replicas/<alias>.db` neben `node.db` (Verzeichnis `0700`), in `db_info` mit
  der Rolle `replica` und der `hub_id`. Darin `documents` wie am Hub, aber ohne eindeutigen
  Index auf den Namen — auf dem Node zählt die `id` —, und `sync_state`. Abgeleitet, nie
  selbst beschrieben: Sie enthält genau die Zeilen, die der Hub geliefert hat, und lässt sich
  jederzeit neu abgleichen. Der erste `sync` eines Hub-Eintrags legt sie an; `node hub rm`
  löscht sie mit, ebenso `config import` für Aliase, die im Export fehlen. Eine Replica mit
  fremder Schemafassung verwirft `sync` und legt sie neu an.
- **sync_state** — Tabelle der Replica: je Collection der Stand des Abgleichs (`revision`) und
  der Zeitpunkt der letzten Seite (`synced_at`).
- **contract** (Vertrag) — die Schnittstelle zwischen Node und Hub, beschrieben in
  [`vertrag.md`](vertrag.md), im Code das neutrale Paket `internal/contract`. Trägt eine
  **Fassung** (`version`, derzeit 1); der Hub nennt sie in jeder Antwort.
- **sync** (Abgleich) — der Vorgang des Vertrags, mit dem ein Node je Collection alles seit
  einer Revision holt, in Seiten; jede Seite ist eine Transaktion in der Replica. Collections,
  die der Hub nicht erlaubt oder der Node nicht mehr will, entfernt er aus der Replica; bei
  anderer `hub_id` oder einem Stand über der Revision des Hubs gleicht er von vorn ab.
  Kommando: `kephalaion node sync [<alias>]`, bisher nur über `transport local`; scheitert ein
  Hub-Eintrag, laufen die übrigen weiter, der Exit-Code ist 1.
- **page** (Seite) — eine Antwort des Abgleichs: ganze Revisionen, bis die **page size**
  (Seitengröße, Zeilen je Seite, Standard 500, am Hub höchstens 5000) erreicht ist; eine
  einzelne größere Revision kommt ganz. **until** (`bis`) ist die Revision, bis zu der der
  Node danach alles hat — auf der letzten Seite, auch einer leeren, die Revision des Hubs
  (**hub_revision**); **more** (`mehr`) sagt, dass eine weitere Seite folgen kann. Der Node
  setzt den Stand jeder angefragten Collection auf max(`since`, `until`), nie zurück.

## Zugriff

- **account** (Konto) — wer zugreift: Mensch, KI oder Programm. Hat einen Namen. Am Hub
  `kephalaion hub account add|list|show|set|rm|lock|unlock|grant|revoke|token`: Beschreibung,
  gesperrt und den maßgeblichen Hash führt die lokale Tabelle **accounts**; die Rechte je
  Collection stehen in den `SYSTEM:A:`-Zeilen, bei einem gesperrten Account gemerkt in
  `accounts` (`locked_rights`). Name gemeinsam mit den Nodes eindeutig.
- **setup token** (Einrichtungstoken) — das Token, das `hub account add` und `hub account
  token` einmal anzeigen. Es gilt nur für den ersten Vorgang, ein `rotate`.
- **token** — Geheimnis eines Accounts, Format `keph_<geheimnis>`. Jede Anfrage trägt
  Account-Name und Token. Gespeichert wird nur der Hash. Unabhängig vom Transport.
- **scope** — ein Recht eines Accounts auf einer Collection, geschrieben
  `<collection>:<recht>`. Ein Account hat mehrere. Gespeichert je Collection in der
  Account-Zeile. Rechte:
  - `read` — hat jeder in der Collection eingetragene Account.
  - `write` — anlegen; Eigenes ändern und löschen; gelöschte Namen neu anlegen.
  - `supersede` — Fremdes ändern, ablösen, löschen.
  - `replicate` — kein Scope eines Accounts, sondern das Recht eines Nodes: Inhalt und
    Account-Zeilen der Collection abgleichen. Steht am Hub in `node_collections`.
- **node entry** (Node-Eintrag) — ein Node am Hub: Zeile in `nodes`, Name vom Admin, Token
  (nur der Hash), gesperrt ja/nein. Name gemeinsam mit den Accounts eindeutig.
- **hub entry** (Hub-Eintrag) — ein Hub am Node: Zeile in `hubs` in `node.db`, Alias vom
  Node, Name des Nodes am Hub (`node_name`, `--node`), Transport, Adresse, Token, `hub_id`
  (Kopie aus der Replica).
- **hub_id** — Kennung des Hubs, eine ULID, von `hub init` vergeben und in `db_info`
  gehalten; `status` zeigt sie. Am Node ist `db_info.hub_id` der Replica maßgeblich,
  `hubs.hub_id` in `node.db` nur Kopie für Anzeige und Export. Weicht sie ab, gleicht der Node
  von vorn ab.
- **grant** / **revoke** — `kephalaion hub node grant <node> <collection>`: gibt einem Node
  `replicate` auf eine Collection; `revoke` nimmt es zurück. `kephalaion hub account grant
  <name> <collection> [--write] [--supersede]` setzt die Rechte eines Accounts in einer
  Collection vollständig (ohne `--write` wird `write` entzogen); `revoke` macht seine Zeile
  zur Löschmarke.
- **lock** / **unlock** — `kephalaion hub node lock <name>`: sperrt einen Node; `unlock` hebt
  die Sperre auf. `hub account lock` macht alle Zeilen eines Accounts zu Löschmarken und merkt
  seine Rechte; `unlock` legt sie wieder an.
- **admin** — der Account, als der die CLI am Hub handelt; steht in `created_by` und in
  `actions`. Als Account- und Node-Name reserviert (`ident.CheckPrincipalName`).
- **actions** (Protokoll) — Tabelle des Hubs: wer wann was getan hat. `subject` nennt das
  Ziel einer Handlung ohne Dokument — Collection, Node oder `<node>:<collection>`.
- **--token-stdin** — liest ein Token als eine Zeile von der Standardeingabe. Ein Token wird
  nie als Argument übergeben und nur gekürzt angezeigt (`keph_…` und die letzten vier
  Zeichen).
- **name rule** (Namensregel) — Namen von Collections, Nodes, Accounts und Hub-Aliasen:
  `[a-z0-9][a-z0-9._-]{0,62}`, kein `:`, kein Präfix `system` in beliebiger Schreibweise;
  Accounts und Nodes dürfen nicht `admin` heißen.
- **SYSTEM:** — reservierter Präfix im Namen eines Dokuments, nur der Hub schreibt ihn.
  `SYSTEM:A:<account>` ist die Zeile eines Accounts in einer Collection.
- **carrier** (Träger) — der Node, der eine Anfrage an den Hub trägt; meldet sich mit seinem
  eigenen Token an. Das Token des Accounts steht in der Anfrage.
- **rotate** — ersetzt das Token eines Accounts: altes Token zur Anmeldung, Hash des neuen.
  Erster Vorgang jedes Accounts; eine eigene Begrüßung gibt es nicht.

## Auslieferung

- **release** — eine veröffentlichte Version auf GitHub, erzeugt aus einem Git-Tag `v*`. Die
  Version ist der Tag; eine `VERSION`-Datei gibt es nicht. Trägt die Assets.
- **asset** — eine Datei an einem Release: je Plattform ein nacktes Binary
  `kephalaion-<os>-<arch>`, dazu `SHA256SUMS` und `install.sh`.
- **SHA256SUMS** — Asset mit den SHA-256-Prüfsummen der Binaries eines Releases, nicht
  mehr. Prüft die Unversehrtheit, nicht die Herkunft.
- **latest** — das neueste veröffentlichte Release ohne Suffix; so, wie die GitHub-API es
  unter `releases/latest` nennt.
- **prerelease** (Vorabversion) — ein Release, dessen Tag ein Suffix trägt
  (`v0.2.0-rc1`). Nie `latest`; nur ausdrücklich per Version zu erreichen.
- **dev build** (Entwicklungs-Build) — ein Binary, das nicht aus einem Release stammt. Trägt
  die Version `dev`, dazu den Commit aus `git describe`.
- **version** — `kephalaion version`: zeigt Version, Commit, Go-Version und Plattform.
- **upgrade** — `kephalaion upgrade`: ersetzt das laufende Binary durch das Binary eines
  Releases, nach Prüfung gegen `SHA256SUMS`, atomar. Stuft nie von selbst zurück.
- **install.sh** — Installationsskript für die Erstinstallation nach
  `~/.local/bin/kephalaion`; liegt im Repo und hängt an jedem Release.
