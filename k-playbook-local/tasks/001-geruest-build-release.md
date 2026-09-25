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
| 6 — Doku und erster Durchlauf | offen | | |

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
