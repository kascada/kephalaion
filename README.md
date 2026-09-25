# kephalaion

Eine geteilte Wissensdatenbank mehrerer Nutzer und Projekte: ein Binary mit zwei Rollen —
dem Hub für Store, Journal und Accounts und dem Node als lokalem MCP-Server mit Replica.
Was geplant ist und warum, steht in [`docs/konzept.md`](docs/konzept.md), die Begriffe in
[`docs/begriffe.md`](docs/begriffe.md).

**Stand:** Gebaut sind das Gerüst (`version`, `upgrade`) und das Einrichten der Rollen:
`hub init`, `node init`, `status` und `config show|export|import`. Inhalte fließen noch nicht.

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
