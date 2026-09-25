# Projektregeln

Diese Datei gilt nur für dieses Projekt. Sie wird nach der mitgelieferten
Ebene gelesen und kann deren Aussagen ergänzen oder überstimmen.

Was hier hineingehört: Aufbau und Besonderheiten des Projekts, Konventionen,
wiederkehrende Abläufe, alles was ein Assistent in jeder Sitzung wissen sollte.

Was nicht: allgemeine k-playbook-Regeln — die stehen in der mitgelieferten
Ebene und werden bei jedem Update aktualisiert.

## Aufbau

Ein Go-Modul `github.com/kascada/kephalaion`, ein Binary `kephalaion`
(`cmd/kephalaion`). Unterkommandos per erstem Argument, Standardbibliothek `flag`,
keine CLI-Bibliothek. Meldungen des Binarys deutsch, Bezeichner englisch. Begriffe stehen
in `docs/begriffe.md` und werden dort eingetragen, bevor sie benutzt werden.

## Bauen

- Über das `Makefile` im Wurzelverzeichnis, lokal und in CI gleich: `make build` bzw.
  `make dist-host` für diese Plattform, `make dist` für alle vier (linux/darwin ×
  amd64/arm64) plus `SHA256SUMS` nach `dist/` (gitignored), `make dev-install` ersetzt
  `~/.local/bin/kephalaion`.
- Flags stehen nur im Makefile: `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, Version
  und Commit per `-ldflags -X` in `internal/buildinfo`. Ohne `VERSION=` entsteht ein
  dev build.
- Die Toolchain ist die `toolchain`-Zeile in `go.mod`. CI baut mit `GOTOOLCHAIN=local` und
  prüft sie mit `make check-toolchain`. Wer sie anhebt, hebt sie nur dort an.
- Actions in `.github/workflows/` sind auf Commit-SHA gepinnt, mit Versionskommentar;
  Dependabot hält `gomod` und `github-actions` wöchentlich nach.

## Testen

- Vor jedem Commit mit Code: `make check` (gofmt, `go vet`, `go test`, `sh -n install.sh`).
- `shellcheck` über `install.sh` läuft nur in CI; lokal ist es nicht installiert.
- Tests gegen GitHub laufen über `httptest`; die Basis-URL der API ist dafür im
  `upgrade.Upgrader` überschreibbar. Sie ist keine Nutzeroption und wird nicht dokumentiert.

## Release

- Ein Release ist ein Tag `vX.Y.Z` (mit Suffix `-…` eine Vorabversion). Es gibt keine
  `VERSION`-Datei und keine versionierte `SHA256SUMS`.
- Anlegen nur über `make -C k-playbook-local release VERSION=vX.Y.Z`: verlangt sauberen
  Arbeitsbaum, Branch `main` und `HEAD == origin/main`, legt den Tag an und pusht ihn.
- Den Rest macht `.github/workflows/release.yml` (Workflow „Release“): `make check`, `make
  dist`, Release als Entwurf, Assets (vier Binaries, `SHA256SUMS`, `install.sh`), dann
  veröffentlichen. `latest` bekommt nur die höchste Version ohne Suffix.
- Scheitert ein Lauf, wird er über `workflow_dispatch` mit dem Tag wiederholt
  (`gh workflow run release.yml -f tag=vX.Y.Z`); vorhandene Assets werden ersetzt.
- Ein Release wirkt öffentlich: Tag, Push und Veröffentlichung nur nach Rückfrage beim
  Nutzer. Ein v0.x-Release gilt nicht als Veröffentlichung im Sinne der Namensprüfung
  (`docs/konzept.md`); die ist erst vor v1.0 fällig.

## Sichern

- `make -C k-playbook-local sichern [MSG=…]` committet alles und pusht ohne Prüfung —
  für Zwischenstände, nicht für Releases. Da `git add -A` alles nimmt, vorher sehen, was
  im Arbeitsbaum liegt.
