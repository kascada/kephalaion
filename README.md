# kephalaion

Eine geteilte Wissensdatenbank mehrerer Nutzer und Projekte: ein Binary mit zwei Rollen —
dem Hub für Store, Journal und Accounts und dem Node als lokalem MCP-Server mit Replica.
Was geplant ist und warum, steht in [`docs/konzept.md`](docs/konzept.md), die Begriffe in
[`docs/begriffe.md`](docs/begriffe.md).

**Stand:** Gebaut sind das Gerüst (`version`, `upgrade`) und das Einrichten der Rollen:
`hub init`, `node init`, `status`, `config show|export|import`, dazu am Hub Collections und
Nodes, am Node seine Hubs und die gewünschten Collections. Verbindungen und Inhalte gibt es
noch nicht.

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
- **Kommandozeile** — `init`, `status`, `config`, Verwaltung von Collections, Nodes und Hubs;
  später Accounts, `serve`, `sync`, `search`.
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
```

`init` legt die Datenbank samt Schema an und trägt die Rolle in die config
`~/.config/kephalaion/config.yaml` ein. Steht die Rolle schon dort oder gibt es die
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
`init`. Migrationen gibt es noch nicht: Passt die Schemafassung einer Datenbank nicht zum
Binary, ist sie neu anzulegen; die Einstellungen rettet `config export`/`import`, die Inhalte
nicht.

### Hub und Node auf einem Rechner

Der Hub legt Collections und einen Node-Eintrag an und erlaubt ihm Collections; das Token
des Nodes zeigt er genau einmal und speichert nur den Hash. Der Node trägt den Hub unter
einem Alias ein, mit seinem Token über die Standardeingabe — nie als Argument, sonst stünde
es im Shell-Verlauf und in der Prozessliste.

```sh
kephalaion hub init
kephalaion node init

kephalaion hub collection add team-x --description "Wissen von Team X"
kephalaion hub node add laptop --description "dieser Rechner"   # zeigt das Token einmal
kephalaion hub node grant laptop team-x

# Transport local: Hub im selben Prozess, verlangt den Hub in derselben config
read -rs TOKEN            # Token einfügen, Enter
printf '%s\n' "$TOKEN" | kephalaion node hub add privat --transport local --token-stdin
kephalaion node collection add privat:team-x

# oder http auf localhost — verhält sich wie eine getrennte Installation
printf '%s\n' "$TOKEN" | kephalaion node hub add test --transport http \
  --address http://localhost:8080 --token-stdin
unset TOKEN

kephalaion status       # Collections und Nodes am Hub, Hubs und Collections am Node
```

Weitere Transporte sind `https` (`--address https://…`) und `ssh` (`--address
[user@]host[:port]`, optional `--ssh-key`). Namen von Collections, Nodes und Hub-Aliasen
bestehen aus `a–z`, `0–9`, `.`, `_` und `-`, beginnen mit Buchstabe oder Ziffer, haben
höchstens 63 Zeichen und nicht den Präfix `system`. Die Hilfe zeigt alle Kommandos:
`kephalaion hub node --help`, `kephalaion node hub --help`.

`config export` sichert auch die Collections, Nodes (nur mit Hash) und die Hubs des Nodes
samt ihrem Token im Klartext — die Datei entsteht deshalb mit `0600`. Ein Export aus einer
älteren Fassung (Format 1) lässt sich weiter importieren; er ersetzt nur die `settings`.

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
