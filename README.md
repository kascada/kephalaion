# kephalaion

Eine geteilte Wissensdatenbank mehrerer Nutzer und Projekte: ein Binary mit zwei Rollen —
dem Hub für Store, Journal und Accounts und dem Node als lokalem MCP-Server mit Replica.
Was geplant ist und warum, steht in [`docs/konzept.md`](docs/konzept.md), die Begriffe in
[`docs/begriffe.md`](docs/begriffe.md).

**Stand:** Gebaut sind das Gerüst (`version`, `upgrade`) und das Einrichten der Rollen:
`hub init`, `node init`, `status`, `config show|export|import`, dazu am Hub Collections und
Nodes, am Node seine Hubs und die gewünschten Collections. Der Hub nimmt Dokumente auf
(`hub doc`, `hub import`), der Node gleicht sie in seine Replica ab (`node sync`) und zeigt
sie an (`node doc`) — bisher nur über `transport local`, also mit Hub und Node in derselben
config. Noch nicht gebaut: `serve`, jede Verbindung über das Netz, Accounts, Suche.

## Was gebraucht wird (grob)

Vorläufige Übersicht, damit nichts fehlt; ausgearbeitet wird sie später. Einzelheiten stehen
im Konzept.

- **Hub** — einmal je Installation, einziger Schreiber. Hält Store, Collections, Accounts,
  Nodes, Protokoll. Datenbank SQLite, später wahlweise PostgreSQL. Kein MCP, sucht nicht.
- **Node** — einmal je Rechner, als Dienst. MCP-Server über HTTP für Clients (Claude Code,
  Cursor, OpenCode, k-playbook). Hält je Hub eine Replica, indiziert und sucht lokal (FTS5).
- **Beide in einem Prozess** ist der häufige Fall; der Node erreicht einen Hub über `local`,
  `http` (nur `localhost`, zum Testen), `https` oder `ssh`.
- **Collections** — Einheit für Rechte und Abgleich. **Accounts** mit Token für Menschen, KIs
  und Programme; **Nodes** mit eigenem Token und den Collections, die sie abgleichen dürfen.
- **Dokumente** — Name ist ein Pfad, stabile `id`, Revision. Löschen als Löschmarke.
- **Abgleich** — der Node fragt „alles seit Revision X“, in Seiten; `hub_id` erkennt einen neu
  angelegten Hub.
- **Werkzeuge** — lesen (`search`, `read`, `list`), schreiben (`create`, `create_numbered`,
  `write`, `append`, `replace_section`, `rename`, `supersede`, `delete`,
  `replace_directory`), dazu eigene für k-playbook (Eingang, Warteschlange, Todos, Tasks).
- **Kommandozeile** — `init`, `status`, `config`, Verwaltung von Collections, Nodes und Hubs,
  Dokumente am Hub, `node sync`; später Accounts, `serve`, `search`.
- **Stufen** — 1 lesen, 2 schreiben mit Rechten, 3 Vorgänge auf Dateien, 4 Schnipsel
  (zurückgestellt), 5 semantische Suche.

## Installation

Linux und macOS, jeweils amd64 und arm64:

```sh
curl -fsSL https://github.com/kascada/kephalaion/releases/latest/download/install.sh | sh
```

Das Skript lädt das Binary der Plattform und `SHA256SUMS` aus dem neuesten Release, prüft die
Prüfsumme und legt das Binary nach `~/.local/bin/kephalaion`. Liegt `~/.local/bin` nicht im
`PATH`, nennt es die Zeile fürs Shell-Profil. Eine bestimmte Version:

```sh
curl -fsSL https://github.com/kascada/kephalaion/releases/latest/download/install.sh | KEPHALAION_VERSION=v0.1.0 sh
```

## Upgrade

```sh
kephalaion upgrade            # auf das neueste Release
kephalaion upgrade --check    # nur nachsehen
kephalaion upgrade --version v0.1.0   # genau diese Version, auch zurück
```

`upgrade` prüft die Prüfsumme und ersetzt das Binary atomar; scheitert etwas, bleibt das alte
unverändert. Vorabversionen (`v0.2.0-rc1`) und ältere Versionen gibt es nur mit `--version`,
ebenso das Ersetzen eines selbst gebauten `dev`-Binarys. Ohne Anmeldung erlaubt die GitHub-API
60 Anfragen je Stunde.

## Einrichten

Hub und Node werden je mit einem Aufruf eingerichtet — ohne Rückfragen, nie überschreibend:

```sh
kephalaion hub init     # Datenbank ~/.local/share/kephalaion/hub.db, Abschnitt hub: in der config
kephalaion node init    # Datenbank ~/.local/share/kephalaion/node.db, Abschnitt node: in der config
kephalaion node init --db sqlite:///pfad/node.db   # anderer Ort, absoluter Pfad
kephalaion hub init --listen 0.0.0.0:7434          # auch für Nodes anderer Rechner
```

`init` legt die Datenbank samt Schema an und trägt die Rolle in die config
`~/.config/kephalaion/config.yaml` ein, mit `db:` und `listen:` — wo der Dienst der Rolle
später lauscht; Standard Node `127.0.0.1:7433`, Hub `127.0.0.1:7434`. Noch lauscht nichts. Steht die Rolle schon dort oder gibt es die
Datenbankdatei schon, bricht es ab. Die Orte folgen `XDG_CONFIG_HOME` und `XDG_DATA_HOME`;
eine andere config wählt `--config` oder `KEPHALAION_CONFIG`. PostgreSQL ist vorgesehen, aber
noch nicht unterstützt.

```sh
kephalaion status       # welche Rollen, wo ihre Datenbank liegt, Kennzahlen
kephalaion config show  # config und settings je Rolle
kephalaion config export --output keph-config.yaml   # Einstellungen sichern, ohne Inhalte
kephalaion config import keph-config.yaml            # in eingerichtete Rollen zurückschreiben
```

`status` und alle anderen Kommandos öffnen nur vorhandene Datenbanken, angelegt wird nur mit
`init` — einzige Ausnahme ist die Replica, die der erste `node sync` anlegt (siehe unten).
Migrationen gibt es noch nicht: Passt die Schemafassung einer Datenbank nicht zum
Binary, ist sie neu anzulegen; die Einstellungen rettet `config export`/`import`, die Inhalte
nicht. Dokumente am Hub sind wieder einzuspielen (`hub import`); eine Replica mit fremder
Schemafassung verwirft `node sync` selbst und gleicht sie neu ab.

### Hub und Node auf einem Rechner

Der Hub legt Collections und einen Node-Eintrag an und erlaubt ihm Collections; das Token
des Nodes zeigt er genau einmal und speichert nur den Hash. Der Node trägt den Hub unter
einem Alias ein, mit dem Namen, unter dem der Hub ihn kennt (`--node`), und seinem Token
über die Standardeingabe — nie als Argument, sonst stünde es im Shell-Verlauf und in der
Prozessliste.

```sh
kephalaion hub init
kephalaion node init

kephalaion hub collection add team-x --description "Wissen von Team X"
kephalaion hub node add laptop --description "dieser Rechner"   # zeigt das Token einmal
kephalaion hub node grant laptop team-x

# Transport local: Hub im selben Prozess, verlangt den Hub in derselben config
read -rs TOKEN            # Token einfügen, Enter
printf '%s\n' "$TOKEN" | kephalaion node hub add privat --node laptop --transport local --token-stdin
kephalaion node collection add privat:team-x

# oder http auf localhost — verhält sich wie eine getrennte Installation
printf '%s\n' "$TOKEN" | kephalaion node hub add test --node laptop --transport http \
  --address http://localhost:7434 --token-stdin
unset TOKEN

kephalaion status       # Collections und Nodes am Hub, Hubs und Collections am Node
```

Weitere Transporte sind `https` (`--address https://…`) und `ssh` (`--address
[user@]host[:port]`, optional `--ssh-key`). Namen von Collections, Nodes und Hub-Aliasen
bestehen aus `a–z`, `0–9`, `.`, `_` und `-`, beginnen mit Buchstabe oder Ziffer, haben
höchstens 63 Zeichen und nicht den Präfix `system`. Die Hilfe zeigt alle Kommandos:
`kephalaion hub node --help`, `kephalaion node hub --help`.

`config export` sichert auch die Collections, Nodes (nur mit Hash) und die Hubs des Nodes
samt ihrem Token im Klartext — die Datei entsteht deshalb mit `0600`. Das Exportformat ist 3
(`format: 3`, mit `node_name` je Hub-Eintrag). Ein Export im Format 1 lässt sich weiter
importieren und ersetzt nur die `settings`; einer im Format 2 nur, wenn er am Node keine
Hub-Einträge enthält — ihnen fehlt `node_name`. Dokumente und Replicas gehören nicht zum
Export.

### Dokumente einspielen und abgleichen

Der ganze Weg auf einem Rechner, mit Hub und Node in derselben config: Der Hub nimmt
Dokumente auf, der Node gleicht sie über `transport local` in seine Replica ab und liest sie
dort. Ausgangspunkt ist ein Verzeichnis `~/wissen/team-x` mit `leitfaden.md`,
`tasks/001-start.md`, `tasks/002-ende.md` und einem `.git/`.

```sh
kephalaion hub init
kephalaion node init
kephalaion hub collection add team-x --description "Wissen von Team X"
kephalaion hub node add laptop --description "dieser Rechner"   # zeigt das Token einmal
kephalaion hub node grant laptop team-x

kephalaion hub import team-x ~/wissen/team-x      # ein Verzeichnis, eine Revision
echo "Heute: Abgleich ausprobieren" | kephalaion hub doc put team-x notizen/heute.md

read -rs TOKEN            # Token einfügen, Enter
printf '%s\n' "$TOKEN" | kephalaion node hub add privat --node laptop --transport local --token-stdin
unset TOKEN
kephalaion node collection add privat:team-x

kephalaion node sync                              # alle Hub-Einträge; oder: node sync privat
kephalaion node doc list privat:team-x
kephalaion node doc get privat:team-x leitfaden.md
kephalaion status
```

`hub import` liest das Verzeichnis rekursiv; der Name eines Dokuments ist sein relativer
Pfad, mit `--prefix pfad/` davor. Versteckte Dateien und Verzeichnisse wie `.git` übergeht
es still; was kein UTF-8-Text, größer als 1 MiB oder keine gewöhnliche Datei ist, meldet es
als „übersprungen“. Alle Dokumente eines Imports sind ein Schreibvorgang mit einer Revision;
scheitert eines (ungültiger Name, Konflikt zwischen Dokument und Verzeichnis), wird nichts
geschrieben. Was im Verzeichnis fehlt, bleibt am Hub stehen.

```text
angelegt: leitfaden.md
angelegt: tasks/001-start.md
angelegt: tasks/002-ende.md
Import nach team-x: 3 angelegt, 0 ersetzt, 0 unverändert, 0 übersprungen — Revision 1.
```

`hub doc put` legt ein Dokument an oder ersetzt es, der Inhalt kommt von der
Standardeingabe oder aus `--file pfad`; unveränderter Inhalt zählt keine Revision. `hub doc
get|list` lesen am Hub, `hub doc rm` löscht — es bleibt eine Löschmarke, die der Abgleich
weitergibt. Namen mit dem Präfix `SYSTEM:` schreibt nur der Hub selbst.

`node sync` fragt je gewünschter Collection alles seit dem letzten Stand ab, in Seiten; den
ersten Abgleich eines Hub-Eintrags legt seine Replica an, `replicas/<alias>.db` neben
`node.db`:

```text
Hub privat (hub_id 01M3ECGQP32QBHTGXERZSMVBWR): 1 Seite
  team-x: abgeglichen, 4 Zeilen, Revision 2
```

`node doc list|get` lesen nur die Replica, so wie der letzte Abgleich sie hinterlassen hat,
ohne Löschmarken und `SYSTEM:`-Zeilen:

```text
NAME          REVISION  GEÄNDERT
leitfaden.md  1         2026-09-26 10:15 von admin
notizen/      –         –
tasks/        –         –
```

Nach `kephalaion hub doc rm team-x notizen/heute.md` bringt der nächste `node sync` die
Löschmarke (`team-x: abgeglichen, 1 Zeile, Revision 3`), und `node doc list` zeigt
`notizen/` nicht mehr. `status` zeigt am Node je Hub-Eintrag die `hub_id` aus der Replica und je
Collection Stand und letzten Abgleich:

```text
node: eingerichtet
  …
  Hubs:
    privat: local, als Node laptop
      hub_id:      01M3ECGQP32QBHTGXERZSMVBWR
      Collections:
        team-x: Revision 3, abgeglichen 2026-09-26 10:15
```

Bisher geht nur `transport local`: der Hub derselben config, im selben Prozess. Für
Einträge mit `http`, `https` oder `ssh` meldet `node sync` „noch nicht unterstützt“, ohne
`hub:`-Abschnitt in der config scheitert auch `local`. Ein gescheiterter Eintrag hält die
übrigen nicht auf; die Meldung steht auf stderr, und der Exit-Code ist 1 — hier mit dem
Eintrag `test` (`http`) aus dem Beispiel davor:

```text
Hub privat (hub_id 01M3ECGQP32QBHTGXERZSMVBWR): 1 Seite
  team-x: abgeglichen, 0 Zeilen, Revision 3
Hub test: gescheitert
node sync: Hub test: Transport http wird noch nicht unterstützt; bisher geht nur local
```

Collections, die der Hub nicht (mehr) erlaubt oder die der Node nicht mehr will
(`node collection rm`), entfernt `node sync` aus der Replica. Nennt der Hub eine andere
`hub_id` als bisher oder steht die Replica weiter als der Hub, leert `node sync` sie und
gleicht von vorn ab. Die Replica ist abgeleitet: `node hub rm` löscht sie mit, ebenso `config import` für
Aliase, die im Export fehlen. Die Regeln des Abgleichs stehen in
[`docs/vertrag.md`](docs/vertrag.md).

## Bauen

Go in der Version aus der `toolchain`-Zeile von `go.mod`; ein älteres Go lädt sie selbst
nach.

```sh
make check        # gofmt, go vet, Tests, Syntax von install.sh
make dist         # alle vier Plattformen und SHA256SUMS nach dist/
make dev-install  # diese Plattform bauen und ~/.local/bin/kephalaion ersetzen
make              # alle Targets
```

Ein Release entsteht aus einem Tag `v*`: `.github/workflows/release.yml` prüft, baut und
veröffentlicht es mit den Binaries, `SHA256SUMS` und `install.sh`.

## Lizenz

Apache-2.0, siehe [`LICENSE`](LICENSE).
