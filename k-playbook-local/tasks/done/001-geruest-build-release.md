# Task 001 — Gerüst: Build, CI, Release, Installation und Upgrade

Ein Binary `kephalaion`, das noch nichts kann außer `version` und `upgrade`, aber den ganzen
Weg durchläuft: bauen, testen, Release per Tag, installieren, aktualisieren.

## Intent

Bevor Stufe 1 beginnt, steht der Weg, auf dem jede spätere Funktion auf alle Rechner kommt.
Installiert wird nur ein Binary, ohne Clone, ohne Verzeichnis, an dem es hängt.
- Ein Tag `v*` erzeugt ohne Handarbeit ein GitHub-Release mit vier Binaries und `SHA256SUMS`.
- Auf einem frischen Rechner (Linux, macOS) genügt ein Befehl zur Installation nach
  `~/.local/bin/kephalaion`.
- `kephalaion upgrade` bringt ein installiertes Binary auf das neueste Release, prüft die
  Prüfsumme und ersetzt sich atomar; ein Abbruch hinterlässt das alte Binary intakt.
- Jeder Push läuft durch CI (Format, vet, Tests, Cross-Build); Dependabot ist eingerichtet.
- Build lokal und in CI mit denselben Flags über dasselbe Makefile.

## Referenzen

- `docs/konzept.md` — Entscheidungen: Go, ein Binary, `CGO_ENABLED=0` wegen
  `modernc.org/sqlite`, Benennung `kephalaion`.
- `docs/begriffe.md` — neue Begriffe dort eintragen, bevor sie benutzt werden.
- `k-playbook-local/Makefile` — persönliche Abläufe (`sichern`); `release` kommt hinzu.
- `k-playbook/Makefile`, `k-playbook/.github/workflows/release.yml` (Installation, nur lesen)
  — Vorbild für Build-Flags, Toolchain-Pinning und Release-Workflow.

## Ziel

Ein lauffähiges Go-Modul `github.com/kascada/kephalaion` mit:

- `kephalaion version` — Version, Commit, Go-Version, Plattform.
- `kephalaion upgrade [--check] [--version vX.Y.Z]` — Selbstaktualisierung aus GitHub-Releases.
- `install.sh` — Erstinstallation mit
  `curl -fsSL https://github.com/kascada/kephalaion/releases/latest/download/install.sh | sh`.
- CI, Release-Workflow, Dependabot.
- Makefile im Wurzelverzeichnis für Build und Test; `release`-Target in
  `k-playbook-local/Makefile`.

## Kontext

- **Nur das Binary.** Anders als k-playbook gibt es auf Zielrechnern keinen Clone. Deshalb
  keine versionierte `SHA256SUMS` und keine `VERSION`-Datei: Die Version ist der Git-Tag,
  `SHA256SUMS` ist ein Release-Asset neben den Binaries.
- **Repo öffentlich.** Downloads laufen ohne Anmeldung über
  `https://github.com/kascada/kephalaion/releases/…` bzw. die GitHub-API. Kein `gh`, kein
  Token.
- **Plattformen:** linux/amd64, linux/arm64, darwin/amd64, darwin/arm64.
- **Build-Flags:** `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, Version per
  `-ldflags "-s -w -X …"`. Toolchain in `go.mod` festgelegt (`toolchain`-Zeile, das zur
  Ausführung aktuelle stabile Go-Release; der Pin in k-playbook ist nur Beispiel); CI nimmt
  genau diese mit `GOTOOLCHAIN=local`.
- **Asset-Namen:** `kephalaion-<os>-<arch>` als nacktes Binary, kein Archiv.
- **Kommandozeile:** Standardbibliothek (`flag`), keine CLI-Bibliothek. Unterkommandos per
  erstem Argument; ohne Argument oder mit `help` die Übersicht.
- **Dienst zurückgestellt.** `upgrade` startet keinen Dienst neu; das kommt, wenn es den
  Dienst gibt.
- **Entwicklungs-Builds** tragen die Version `dev` (plus Commit aus `git describe`).
  `upgrade` ersetzt einen `dev`-Build nur mit ausdrücklicher `--version`.
- **Versionen:** Vergleich nach Semver. Tags mit Suffix (z. B. `v0.2.0-rc1`) sind
  Vorabversionen und nie `latest`.
- **Sprache:** Doku deutsch, Begriffe und Bezeichner englisch. Meldungen des Binarys deutsch.

## Zu bauen

### Etappe 1 — Modul und Binary

- `docs/begriffe.md`: `upgrade`, `release` und weitere neue Begriffe dieses Tasks eintragen,
  bevor sie benutzt werden; welche Wörter Begriffe sind, entscheidet die Ausführung.
- `go.mod` (`module github.com/kascada/kephalaion`, `go`- und `toolchain`-Zeile).
- `cmd/kephalaion/main.go`: Verteilung auf Unterkommandos, `help`, `version`.
- `internal/buildinfo/`: Version, Commit, per `-ldflags` gesetzt; Fallback `dev`.
- `.gitignore` ergänzen: `/dist/`.

### Etappe 2 — Makefile und CI

- `Makefile` im Wurzelverzeichnis, Standardziel `help`: `build`, `test`, `check`
  (gofmt-Prüfung, `go vet`, Tests), `dist` (alle vier Plattformen nach `dist/` plus
  `SHA256SUMS`), `dist-host`, `dev-install` (baut und ersetzt `~/.local/bin/kephalaion`).
- `.github/workflows/ci.yml`: bei Push und Pull Request `make check` und `make dist`.
- `.github/dependabot.yml`: `gomod` und `github-actions`, wöchentlich.
- Workflow-Rechte knapp: CI `permissions: contents: read`, Release-Job `contents: write`;
  Actions gepinnt wie im Vorbild.

### Etappe 3 — Release

- `.github/workflows/release.yml`: bei Tag `v*` Toolchain aus `go.mod` prüfen, `make check`,
  `make dist` mit der Tag-Version; Release als Entwurf anlegen, vier Binaries, `SHA256SUMS`
  und `install.sh` anhängen, erst dann veröffentlichen. `SHA256SUMS` deckt nur die Binaries.
  Tag mit Suffix → als Vorabversion markieren.
- Wiederholungslauf wie im Vorbild: `workflow_dispatch` mit Tag; existiert das Release
  schon, Assets per `--clobber` hochladen und veröffentlichen.
- `k-playbook-local/Makefile`: `release VERSION=vX.Y.Z` — verlangt sauberen Arbeitsbaum,
  Branch `main`, gepusht; legt den Tag an und pusht ihn. Hilfe um `VERSION=` ergänzen.

### Etappe 4 — `upgrade`

- `internal/upgrade/`:
  - neueste Version über `GET /repos/kascada/kephalaion/releases/latest`, oder die mit
    `--version` genannte;
  - gleiche Version: nichts tun, melden; `--check`: nur melden;
  - installierte Version neuer als `latest`: melden, nichts tun; nur `--version` stuft
    ausdrücklich zurück;
  - Asset der eigenen Plattform und `SHA256SUMS` laden, Summe prüfen;
  - in eine temporäre Datei im Verzeichnis des laufenden Binarys schreiben (`os.Executable`,
    Links aufgelöst), `chmod 0755`, `rename` über das alte — atomar, dasselbe Dateisystem;
  - die temporäre Datei trägt ein erkennbares Namensmuster und wird bei Fehler sowie bei
    SIGINT/SIGTERM entfernt;
  - kein Schreibrecht, Prüfsumme falsch, Netz weg, API-Rate-Limit (403/429): klare Meldung,
    altes Binary unberührt.
- Die Basis-URLs sind für Tests überschreibbar (nicht als Nutzeroption dokumentiert).
- Tests mit `httptest`: Versionsvergleich, Asset-Auswahl, Prüfsumme richtig/falsch,
  atomarer Austausch in einem temporären Verzeichnis, dev-Build ohne `--version`.

### Etappe 5 — Installationsskript

- `install.sh` im Wurzelverzeichnis, POSIX-`sh`, zugleich Release-Asset (siehe Etappe 3):
  Plattform erkennen, neuestes Release oder `KEPHALAION_VERSION`, Binary und `SHA256SUMS`
  laden (`curl` oder `wget`), prüfen (`sha256sum` oder `shasum -a 256`), nach
  `~/.local/bin/kephalaion`. Fehlt `~/.local/bin` im `PATH`, die Zeile fürs Shell-Profil
  ausgeben. Rate-Limit (403/429) mit klarer Meldung.
- `shellcheck` im CI über `install.sh`.

### Etappe 6 — Doku und erster Durchlauf

- `README.md` (deutsch, kurz): was es ist, Verweis auf `docs/konzept.md`, Installation
  (der Befehl aus dem Ziel wörtlich), Upgrade, Bauen.
- `k-playbook-local/k-playbook.md`: Bauen, Testen, Release, Sichern als Projektregeln.
- Vor `release VERSION=v0.1.0` den Nutzer fragen, ob die Namensprüfung aus
  `docs/konzept.md` erledigt ist oder ob ein v0.x-Release nicht als Veröffentlichung gilt.
- Durchlauf, **jeder Schritt nach Rückfrage**, weil er öffentlich wirkt:
  `release VERSION=v0.1.0` → CI grün, Assets vorhanden → `install.sh` auf diesem Rechner →
  `kephalaion version`. Danach `v0.1.1` und `kephalaion upgrade` → neue Version läuft.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — Modul und Binary | erledigt | 2026-09-25 | go.mod (toolchain go1.27.1), `help`/`version`, buildinfo, Begriffe „Auslieferung“ |
| 2 — Makefile und CI | erledigt | 2026-09-25 | Makefile (help, build, test, check, check-toolchain, dist, dist-host, dev-install, clean), ci.yml, dependabot.yml; make check/dist lokal grün |
| 3 — Release | erledigt | 2026-09-25 | release.yml (Entwurf → Assets → veröffentlichen, Vorabversion, latest nur für höchste stabile Version, Wiederholungslauf per workflow_dispatch), `release`-Target in k-playbook-local/Makefile |
| 4 — `upgrade` | erledigt | 2026-09-25 | internal/upgrade (Semver, API latest/tags, SHA256SUMS, atomarer Austausch mit `.kephalaion-upgrade-*`, Abbruch per SIGINT/SIGTERM räumt auf), 21 httptest-Tests grün |
| 5 — Installationsskript | erledigt | 2026-09-25 | install.sh (POSIX, curl/wget, sha256sum/shasum, KEPHALAION_VERSION, 403/429); lokal mit sh, dash, busybox sh gegen einen lokalen Testserver geprüft; shellcheck nur in CI |
| 6 — Doku und erster Durchlauf | erledigt | 2026-09-25 | v0.1.0 + v0.1.1 veröffentlicht, install.sh und upgrade auf diesem Rechner geprüft |

---
## Review-Log (2026-09-25)

**Pfad:** k-playbook-local/tasks/001-geruest-build-release.md
**Intent:** inline (Abschnitt `## Intent`)
**Runden:** 1 (Fast Path)

### Diskussion
Es gab keine strittigen Punkte. Der Editor hat alle gerouteten Issues so umgesetzt, wie der Moderator entschieden hatte. FEHLER-02 (Namensprüfung vor der Veröffentlichung) entscheidet der Task bewusst nicht selbst, sondern legt ihn vor dem ersten Release als Rückfrage vor.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| FEHLER-01 | FEHLER | 001-geruest-build-release.md | Etappe 3, release.yml | Zwischen dem Anlegen des Releases und dem Upload zeigt `releases/latest` auf ein unvollständiges Release; scheitert ein Upload, bleibt ein halbes Release stehen | Entwurf, Upload, dann veröffentlichen; Wiederholungslauf mit `workflow_dispatch` und `--clobber` |
| FEHLER-02 | FEHLER | 001-geruest-build-release.md | Etappe 6 vs. konzept.md „Vor der Veröffentlichung zu prüfen“ | Das erste öffentliche Release ist eine Veröffentlichung; die Namensprüfung wird nicht erwähnt | Vor dem ersten Tag klären |
| WARNUNG-01 | WARNUNG | 001-geruest-build-release.md | Etappe 6, begriffe.md | Begriffe werden erst am Ende eingetragen, aber schon vorher benutzt | Nach Etappe 1 vorziehen |
| WARNUNG-02 | WARNUNG | 001-geruest-build-release.md | Etappe 4, Prüfsumme | `SHA256SUMS` prüft die Integrität, nicht die Herkunft | Signatur später vormerken |
| WARNUNG-03 | WARNUNG | 001-geruest-build-release.md | Etappe 3 | Der Release-Workflow testet nicht; ein Tag auf einem roten Commit wird veröffentlicht | `make check` vor `make dist` |
| WARNUNG-04 | WARNUNG | 001-geruest-build-release.md | Etappe 4, Versionsvergleich | Downgrade und Vorabversionen sind nicht geregelt | Semver; kein automatisches Downgrade; Suffix = Vorabversion |
| WARNUNG-05 | WARNUNG | 001-geruest-build-release.md | Kontext, „derzeit go1.26.x“ | Die Toolchain-Version ist ungeprüft aus k-playbook übernommen | Bei der Ausführung die aktuelle Version wählen |
| WARNUNG-06 | WARNUNG | 001-geruest-build-release.md | Etappe 4, anonyme API | Anonyme Abfragen sind auf 60/h begrenzt | Klare Meldung bei 403/429 |
| FEHLEND-01 | FEHLEND | 001-geruest-build-release.md | Ziel/Etappe 5 | Die Installations-URL ist nicht festgelegt | `install.sh` als Release-Asset, Befehl wörtlich festlegen |
| FEHLEND-02 | FEHLEND | 001-geruest-build-release.md | Etappe 2/3 | Workflow-Rechte und Pinning der Actions fehlen | `contents: read`/`write`, Pinning wie im Vorbild |
| FEHLEND-03 | FEHLEND | 001-geruest-build-release.md | Etappe 4, atomarer Austausch | Aufräumen der temporären Datei nach einem Abbruch ist nicht geregelt | Namensmuster, entfernen bei Fehler/Signal |
| FEHLEND-04 | FEHLEND | 001-geruest-build-release.md | Kontext, Dienst zurückgestellt | Installationspfad = späterer Dienstpfad, Neustart nach `upgrade` | Im Konzept vormerken |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| FEHLER-01 | pass | Gefährdet „ohne Handarbeit“ und `upgrade` | umgesetzt |
| FEHLER-02 | decide | Das Produktthema entscheidet der Nutzer; der Task legt es als Rückfrage vor dem ersten Release vor | umgesetzt |
| WARNUNG-01 | pass | Widerspruch zur eigenen Referenz „eintragen, bevor sie benutzt werden“ | umgesetzt |
| WARNUNG-02 | skip | Nicht blockierend, gehört zu einer späteren Stufe | offen |
| WARNUNG-03 | pass | Billig und verhindert Releases von roten Commits | umgesetzt |
| WARNUNG-04 | decide | Die Semantik von `latest` betrifft `upgrade` direkt | umgesetzt |
| WARNUNG-05 | decide | Toolchain bei der Ausführung wählen | umgesetzt |
| WARNUNG-06 | decide | Nur die Fehlermeldung ergänzt, am API-Weg ändert sich nichts | umgesetzt |
| FEHLEND-01 | decide | Der Intent verlangt „ein Befehl“; `install.sh` wird Release-Asset | umgesetzt |
| FEHLEND-02 | pass | Rechte knapp halten, Pinning wie im Vorbild | umgesetzt |
| FEHLEND-03 | pass | Gehört zu „Abbruch hinterlässt altes Binary intakt“ | umgesetzt |
| FEHLEND-04 | skip | Der Dienst ist bewusst zurückgestellt; gehört ins Konzept, nicht in diese Task | offen |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| FEHLER-01 | umgesetzt | Etappe 3: Entwurf → Assets → veröffentlichen; Wiederholungslauf mit `workflow_dispatch` und `--clobber` |
| FEHLER-02 | umgesetzt | Rückfrage in Etappe 6 vor `release VERSION=v0.1.0` |
| WARNUNG-01 | umgesetzt | begriffe.md in Etappe 1 vorgezogen, aus Etappe 6 entfernt |
| WARNUNG-03 | umgesetzt | `make check` vor `make dist` im Release-Workflow |
| WARNUNG-04 | umgesetzt | Kontext „Versionen“, Etappe 3 Vorabversion, Etappe 4 kein automatisches Downgrade |
| WARNUNG-05 | umgesetzt | „derzeit go1.26.x“ → aktuelles stabiles Go-Release bei der Ausführung |
| WARNUNG-06 | umgesetzt | 403/429 in Etappe 4 und 5 |
| FEHLEND-01 | umgesetzt | Befehl im Ziel, `install.sh` als Asset; `SHA256SUMS` deckt nur die Binaries |
| FEHLEND-02 | umgesetzt | Punkt in Etappe 2 |
| FEHLEND-03 | umgesetzt | Unterpunkt in Etappe 4 |

### Moderator-Entscheidungen
- WARNUNG-02 übersprungen: `SHA256SUMS` im selben Kanal wie das Binary ist eine reine Integritätsprüfung; eine Signatur (cosign/minisign, Attestations) ist ein späteres Thema.
- FEHLEND-04 übersprungen: Installationspfad als Dienstpfad und Neustart nach `upgrade` gehören ins Konzept, wenn es den Dienst gibt.
- FEHLER-02: Der Task entscheidet nicht, ob v0.x als Veröffentlichung gilt; das entscheidet der Nutzer beim Durchlauf.
- Kein Editor-Edit verworfen.

### Intent-Alignment
Ja. Jeder Intent-Punkt hat einen konkreten Bauschritt: Tag → Entwurf → vollständige Assets → veröffentlichen; `curl … | sh` mit `install.sh` als Release-Asset; `upgrade` mit Prüfsumme, atomarem `rename` und Aufräumen; CI mit `make check`/`make dist` und Dependabot; ein Makefile für lokal und CI. Der Durchlauf in Etappe 6 prüft den Weg von Anfang bis Ende.

### Geänderte Dateien
- 001-geruest-build-release.md: Release als Entwurf mit Wiederholungslauf und `make check` (FEHLER-01, WARNUNG-03); Rückfrage zur Namensprüfung (FEHLER-02); begriffe.md in Etappe 1 (WARNUNG-01); Semver/Vorabversionen/Downgrade (WARNUNG-04); Toolchain-Wahl (WARNUNG-05); Rate-Limit-Meldung (WARNUNG-06); Installationsbefehl und `install.sh` als Asset (FEHLEND-01); Workflow-Rechte (FEHLEND-02); Aufräumen der temporären Datei (FEHLEND-03)

### Offen (nicht gefixt)
- WARNUNG-02: Signatur statt nur Prüfsumme — späteres Thema.
- FEHLEND-04: Dienstpfad und Neustart nach `upgrade` — ins Konzept, wenn es den Dienst gibt.

## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** 2026-09-25  
**Zusammenfassung:** Go-Modul `github.com/kascada/kephalaion` (toolchain go1.27.1) mit `help`, `version` und `upgrade` (Semver, SHA256SUMS, atomarer rename mit Aufräumen bei Fehler/SIGINT/SIGTERM), POSIX-`install.sh`, Makefile (check/dist/dev-install), CI mit shellcheck, Release-Workflow (Entwurf → Assets → veröffentlichen, Vorabversion, Wiederholungslauf) und Dependabot; `release`-Target in `k-playbook-local/Makefile`, README und Projektregeln. Durchlauf: v0.1.0 und v0.1.1 veröffentlicht, auf diesem Rechner per `curl … | sh` installiert und mit `kephalaion upgrade` von v0.1.0 auf v0.1.1 gebracht; CI und Release grün.

**Entscheidungen während der Ausführung:**
- Namensprüfung: v0.x gilt nicht als Veröffentlichung (vermerkt in `docs/konzept.md`).
- Etappe 1–5 und Doku per Sub-Agent (ein Commit je Etappe), Durchlauf im Hauptkontext.
- Commits/Push/v0.x-Releases sind in der Frühphase ohne Rückfrage freigegeben.
- Nachbesserung vor v0.1.1: `git describe --long`, damit `Commit:` auch auf einem getaggten Stand einen Hash zeigt (`v0.1.1-0-g1cae340`).
- Actions per SHA auf v7 gepinnt (`actions/checkout` v7.0.1, `actions/setup-go` v7.0.0); das Vorbild pinnt nur per Major-Tag.

**Geänderte Dateien:**
```
 .github/dependabot.yml                             |  11 +
 .github/workflows/ci.yml                           |  63 +++
 .github/workflows/release.yml                      | 139 ++++++
 .gitignore                                         |   3 +
 Makefile                                           | 115 +++++
 README.md                                          |  56 +++
 cmd/kephalaion/main.go                             | 111 +++++
 cmd/kephalaion/main_test.go                        |  38 ++
 docs/begriffe.md                                   |  27 +
 docs/konzept.md                                    |  71 ++-
 go.mod                                             |   8 +
 install.sh                                         | 222 +++++++++
 internal/buildinfo/buildinfo.go                    |  54 ++
 internal/upgrade/github.go                         | 140 ++++++
 internal/upgrade/semver.go                         | 131 +++++
 internal/upgrade/upgrade.go                        | 294 +++++++++++
 internal/upgrade/upgrade_test.go                   | 550 +++++++++++++++++++++
 k-playbook-local/Makefile                          |  48 +-
 k-playbook-local/k-playbook.md                     |  49 ++
 .../material/befunde/install-sh-downloader.md      |  21 +
 20 files changed, 2129 insertions(+), 22 deletions(-)
```
(`docs/konzept.md` enthält zusätzlich die parallel entstandenen Konzeptänderungen zur Konfiguration, die mitgesichert wurden.)

**Code-Änderungen:** Der Diff umfasst rund 2100 Zeilen, davon 550 Testcode. Hier nur die Kernpunkte:
- `internal/upgrade/upgrade.go` — `replace()`: `os.CreateTemp(dir, ".kephalaion-upgrade-*")` neben dem aufgelösten `os.Executable`, Download über `io.MultiWriter(tmp, sha256)`, Summenvergleich, `Chmod 0755`, `Sync`, `Close`, dann ein letzter `ctx.Err()`-Check vor `os.Rename`. Ein `defer` entfernt die Temp-Datei bei jedem Fehler.
- `internal/upgrade/github.go` — `do()` übersetzt 404, Rate-Limit (429 oder 403 mit `X-RateLimit-Remaining: 0`/`Retry-After`), 403 und Netzfehler in deutsche Meldungen.
- `internal/upgrade/semver.go` — Vergleich nach Semver 2.0.0 samt Vorrang der Pre-Release-Kennungen.
- `.github/workflows/release.yml` — `gh release create --draft --verify-tag`, dann `gh release upload --clobber` (6 Assets), dann `gh release edit --draft=false --latest=<höchste Version ohne Suffix>`.
- `Makefile` — `LDFLAGS = -s -w -X …Version=$(VERSION) -X …Commit=$(COMMIT)`, `CGO_ENABLED=0 go build -trimpath -buildvcs=false`.

**Code-Review:** Grundlage war allein der Diff. Urteil: **Approve**, kein kritischer Befund.

| # | Datei | Hinweis | Kategorie |
|---|---|---|---|
| 1 | `install.sh` (`install_binary`) | `mv -f` ersetzt einen Symlink unter `~/.local/bin/kephalaion` durch die Datei. `upgrade` löst Links dagegen auf und ersetzt das Ziel. Das ist uneinheitlich, bei der Standardinstallation aber folgenlos. | Correctness (gering) |
| 2 | `internal/upgrade/upgrade.go` (`replace`) | Der Download ist in der Größe nicht begrenzt, nur durch den Client-Timeout von 10 min. Die Prüfsumme greift erst nach dem Schreiben. Ein fehlerhafter Server könnte also die Platte füllen. Das Risiko ist gering, weil der Kanal derselbe ist wie für `SHA256SUMS`. | Robustheit |
| 3 | `.github/workflows/ci.yml` | `push: branches: "**"` zusammen mit `pull_request` lässt CI für PRs aus Branches des eigenen Repos doppelt laufen. Das kostet nur Laufzeit. | Effizienz |
| 4 | `internal/upgrade` | Reste `.kephalaion-upgrade-*` nach SIGKILL oder Stromausfall räumt niemand weg; sie sind nur am Muster erkennbar. Das ist bewusst so und dokumentiert. | Wartung |
| 5 | `release.yml` | `latest` richtet sich nach allen Git-Tags ohne Suffix, nicht nach den veröffentlichten Releases. Ein Tag ohne Release könnte `latest` für ein Release unterdrücken. Beim vorgesehenen Ablauf über das `release`-Target passiert das nicht. | Correctness (gering) |

Positiv fallen auf:
- Temp-Datei, Prüfsumme und Abbruch sind sauber gekapselt.
- `SHA256SUMS` deckt nur die Binaries.
- Die Rechte der Workflows sind knapp: global `contents: read`, nur der Release-Job hat `write`.
- Der Semver-Vergleich ist vollständig getestet.
- `install.sh` ruft `main` erst in der letzten Zeile auf, ein abgebrochenes `curl | sh` führt also kein halbes Skript aus.

**Intent-Alignment:** Teilweise - Release per Tag, Installation mit einem Befehl, atomares `upgrade` mit Prüfsumme, CI/Dependabot und ein gemeinsames Makefile sind umgesetzt und auf Linux echt durchgespielt. Offen: Auf macOS wurde nichts echt getestet, es gibt nur die Binaries und den Rosetta-Pfad im Skript. Der Abbruch mitten im `upgrade` ist nur durch httptest-Tests belegt, nicht durch einen echten Abbruch.
