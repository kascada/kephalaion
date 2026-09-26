# Task 010 — Arbeitsbranch dev: sichern auf dev, main nur per Release

Gearbeitet und gesichert wird künftig auf `dev`; `main` trägt nur noch veröffentlichte Stände
und rückt allein über `make -C k-playbook-local release` per Fast-Forward vor.

## Intent

Sichern soll jederzeit und für jeden gefahrlos gehen, und `main` soll immer einem
veröffentlichten Stand mit grüner CI entsprechen.
- Auf `dev` darf jeder — Mensch, KI, `/k-task-run` — ohne Prüfung und ohne Rückfrage sichern.
  Er dient nur der Sicherung; Force-Push und Löschen sind auf GitHub gesperrt.
- `main` bewegt sich nur durch `release`, und nur per Fast-Forward auf einen gepushten
  `dev`-Stand, dessen CI grün ist.
- Scheitert eine Vorbedingung, bricht `release` ab, bevor irgendetwas gepusht ist, und sagt,
  was zu tun ist — auch wenn `main` Commits hat, die `dev` fehlen (etwa ein auf GitHub
  gemergter Dependabot-PR).
- `main` bleibt Standard-Branch auf GitHub; `release.yml`, `install.sh` und `upgrade` bleiben,
  wie sie sind.
- Die Regel steht in den Projektregeln und im README, sodass jede Sitzung sie kennt.

## Referenzen

- `k-playbook-local/Makefile` — Targets `sichern` und `release`, werden umgebaut.
- `k-playbook-local/k-playbook.md` — Abschnitte „Bauen“, „Release“, „Sichern“.
- `README.md` — Abschnitt „Bauen“ (Absatz zum Release).
- `.github/workflows/ci.yml` — Trigger.
- `.github/workflows/release.yml` — bleibt unverändert; zur Orientierung: Tag-Format, was es
  prüft, wie es `latest` setzt.
- `.github/dependabot.yml` — bekommt `target-branch`.
- `docs/fortschritt.md` — Anleitung am Anfang der Datei.

## Tools

- `gh` über Bash — CI-Status abfragen, Ruleset über die GitHub-API anlegen und prüfen.
  Freigegeben für: `dev` pushen, Ruleset `main und dev` anlegen bzw. ändern, lesende
  Abfragen. **Nicht** freigegeben: Push nach `main`, Tags pushen, Releases anlegen.

## Ziel

1. Branch `dev` existiert lokal und auf `origin`; ab da wird auf `dev` gearbeitet.
2. `make -C k-playbook-local sichern` sichert nur noch auf `dev`.
3. `make -C k-playbook-local release VERSION=vX.Y.Z` prüft `dev`, schiebt `main` per
   Fast-Forward auf `dev`, taggt und pusht den Tag; den Rest macht wie bisher `release.yml`.
4. CI läuft für Pushes auf `main` und `dev`, für Pull Requests und von Hand.
5. Dependabot stellt Versions-Updates gegen `dev`.
6. Ein Ruleset auf GitHub sperrt Force-Push und Löschen für `main` und `dev`.
7. Projektregeln, README und `docs/fortschritt.md` beschreiben das.

## Kontext

Entschieden am 2026-09-26 im Gespräch:

- **Zwei Branches.** `dev` für die tägliche Arbeit und zum Sichern, jeder darf dort pushen,
  ohne Prüfung — er ist nur Sicherung. `main` nur für veröffentlichte Stände.
- **Fast-Forward**, kein Merge-Commit: `main` ist nach einem Release genau der Stand von
  `dev`, der Verlauf bleibt linear.
- **`main` bleibt Standard-Branch** auf GitHub. Ein `git clone` bekommt damit den
  Release-Stand; wer arbeiten will, wechselt nach `dev`.
- **Dependabot:** Versions-Updates lassen sich mit `target-branch: dev` umlenken.
  Security-Updates (seit 2026-09-26 eingeschaltet) gehen nach GitHub-Vorgabe immer gegen den
  Standard-Branch, also `main`; `target-branch` wirkt auf sie nicht. Solche PRs werden nicht
  auf GitHub gemergt, sondern lokal nach `dev` geholt (`git merge origin/<branch>`, sichern);
  sobald `main` beim nächsten Release die Commits enthält, markiert GitHub den PR als
  gemergt. Wird doch einer auf GitHub gemergt, muss `release` das erkennen (siehe unten).
  Was damit wirklich passiert, sehen wir uns an, wenn der erste Fall auftritt — nicht Teil
  dieser Task.
- **Dependabot liest `dependabot.yml` vom Standard-Branch.** `target-branch: dev` wirkt also
  erst, wenn `main` sie enthält, d. h. nach dem nächsten Release. Das ist in Ordnung.
- **Offener PR #1** (go-sdk 1.7.0 → 1.8.0) zielt auf `main` und ist nicht Teil dieser Task —
  nicht mergen, nicht schließen.
- **Parallele Sitzungen.** Im selben Arbeitsbaum arbeiten oft andere Sitzungen; beim Anlegen
  dieser Task lagen z. B. fremde Änderungen an `Makefile`, `.gitignore` und
  `k-playbook-local/k-playbook.md` (Targets `race`, `cover`, `mutate`). `git switch -c dev`
  trägt solche Änderungen unverändert mit. Nur eigene Dateien committen
  (`git add <pfade>`), nicht `sichern` mit `git add -A`.
- **Tag-Format** wie in `release.yml` und `install.sh`:
  `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`.
- **Nicht Teil dieser Task:** ein echtes Release (v0.2.0 folgt danach, gesondert, nach
  Rückfrage), Schutz der Tags `v*`, eine PR-Pflicht für `main`, `dev` als Standard-Branch.

## Zu bauen

### Etappe 1 — dev anlegen

- `git switch -c dev` vom aktuellen lokalen `main` aus. Hat der lokale `main` Commits, die
  auf `origin/main` fehlen, gehen sie so nach `dev`; `origin/main` bleibt, wo er ist, bis
  zum nächsten Release. Nicht nach `main` pushen.
- `git push -u origin dev`.
- Ab hier alle Commits dieser Task auf `dev`.

### Etappe 2 — Makefile: sichern und release

In `k-playbook-local/Makefile`, im Stil der vorhandenen Targets (POSIX-sh im Rezept,
`set -eu`, Meldungen deutsch, Fehler nach stderr, Pfade über `$(REPO)`):

- **Variable** `GH_REPO := kephalaion/kephalaion` für die Abfrage des CI-Status. Ausdrücklich
  gesetzt statt aus `origin` abgeleitet, damit die Prüfung auch in einer Kopie läuft, deren
  `origin` kein GitHub ist (Etappe 5).
- **`sichern`:**
  1. Nur auf Branch `dev`; sonst Abbruch mit Hinweis `git switch dev`. Detached HEAD ebenso.
  2. Wie bisher `git add -A`, Commit mit `MSG` oder „Zwischenstand“.
  3. `git push origin dev` (ausdrücklich, auch ohne Upstream). Lehnt `origin` ab, weil `dev`
     dort weiter ist: Hinweis auf `git pull --no-rebase` und erneut sichern. Kein Force.
- **`release`** — in dieser Reihenfolge; alle Prüfungen vor dem ersten Push:
  1. `VERSION` prüfen (wie bisher).
  2. Branch `dev`; sonst Abbruch („Release nur von dev, aktuell: …“).
  3. Arbeitsbaum sauber (wie bisher).
  4. `git fetch --quiet origin dev main`.
  5. `HEAD == origin/dev`; sonst Abbruch mit Hinweis auf `sichern` bzw. `git pull`.
  6. `origin/main` ist Vorfahre von `HEAD` (`git merge-base --is-ancestor`); sonst Abbruch
     mit den fehlenden Commits (`git log --oneline HEAD..origin/main`) und dem Weg:
     `git merge origin/main`, sichern, erneut `release`.
  7. Tag nicht auf `origin` vorhanden (wie bisher). Lokal vorhanden nur erlaubt, wenn er auf
     `HEAD` zeigt (`git rev-parse "<tag>^{commit}"`) — Rest eines Abbruchs in 10; zeigt er
     auf einen anderen Commit → Abbruch.
  8. CI auf `HEAD` grün: `gh run list -R "$(GH_REPO)" --workflow ci.yml --branch dev
     --commit <sha> --json status,conclusion,url` — der neueste Lauf auf `dev` zählt; Läufe
     desselben Commits auf `main` (nach Schritt 9) zählen nicht. Kein Lauf → Abbruch („noch
     nicht gestartet? später erneut“); läuft noch → Abbruch mit Hinweis auf `gh run watch`;
     nicht `success` → Abbruch mit URL. Fehlt `gh` → Abbruch mit Hinweis.
  9. `git push origin HEAD:refs/heads/main` — ohne Force; git lehnt alles außer
     Fast-Forward ab.
  10. Tag annotiert auf `HEAD` anlegen, falls lokal noch nicht vorhanden, und pushen (wie
      bisher).
  11. Lokalen `main`, falls vorhanden, nachziehen (`git fetch --quiet origin main:main`);
      scheitert das, nur ein Hinweis, kein Fehler.
  12. Abschlussmeldung wie bisher, dazu: `main` steht jetzt auf `<sha>`.
  - Ein Wiederholungsaufruf nach einem Abbruch nach Schritt 9 muss funktionieren: `main`
    steht dann schon auf `HEAD`, Schritt 6 besteht, Schritt 9 ist ein leerer Push. Abbruch
    zwischen 9 und 10: mit derselben oder einer neuen `VERSION`. Tag lokal angelegt, Push in
    10 gescheitert: mit derselben `VERSION` — Schritt 7 lässt den lokalen Tag durch, 10
    pusht ihn nur.
- **Hilfe** (`make -C k-playbook-local`) und Kommentare an beiden Targets anpassen.

### Etappe 3 — GitHub: CI-Trigger, Dependabot, Ruleset

- **`ci.yml`:** `push` nur noch für die Branches `main` und `dev`; `pull_request` und
  `workflow_dispatch` bleiben. Grund in den Kommentar: Dependabot-Branches liefen bisher
  doppelt (Push und PR). Der Kommentar zu Tags bleibt sinngemäß.
- **`dependabot.yml`:** `target-branch: dev` für `gomod` und `github-actions`; Kommentar,
  dass Security-Updates trotzdem gegen `main` gehen und wie man sie holt.
- **Ruleset** über `gh api repos/kephalaion/kephalaion/rulesets`: Name `main und dev`,
  `target: branch`, `enforcement: active`, `conditions.ref_name.include:
  ["refs/heads/main", "refs/heads/dev"]`, Regeln `deletion` und `non_fast_forward`, keine
  `bypass_actors`. Gibt es ein Ruleset dieses Namens schon, ändern statt doppelt anlegen.
- Prüfen über `gh api repos/kephalaion/kephalaion/rules/branches/dev` und `…/main`: beide
  Regeln aktiv. **Nicht** durch einen echten Force-Push ausprobieren.

### Etappe 4 — Doku

- **`k-playbook-local/k-playbook.md`:**
  - Neuer Abschnitt **„Branches“** vor „Release“: `dev` und `main` wie im Kontext; auf `dev`
    darf jeder ohne Prüfung und ohne Rückfrage sichern, auch KI und `/k-task-run`; nach
    `main` wird nie direkt committet oder gepusht — das ist Regel, keine Sperre (das Ruleset
    sperrt nur Force-Push und Löschen); weicht `main` ab, erkennt `release` das in Schritt 6;
    `main` ist Standard-Branch, ein Clone
    bekommt den Release-Stand; Umgang mit Dependabot-PRs (Versions-Updates gegen `dev`,
    Security-Updates gegen `main`, lokal nach `dev` holen, nicht auf GitHub mergen).
  - **„Release“:** Ablauf von `release` nach Etappe 2 (von `dev`, CI grün, Fast-Forward nach
    `main`, Tag). Der Satz „Ein Release wirkt öffentlich … nur nach Rückfrage“ bleibt; klar
    machen, dass das für `release` gilt, nicht für Pushes auf `dev`.
  - **„Sichern“:** nur auf `dev`; Hinweis auf fremde Änderungen im Arbeitsbaum bleibt.
  - **„Bauen“:** Dependabot „gegen `dev`“ ergänzen.
  - Fremde Änderungen in derselben Datei (etwa unter „Testen“) nicht anfassen und nicht
    mitcommitten, falls sie dann noch offen liegen.
- **`README.md`, Abschnitt „Bauen“:** zwei, drei Sätze zu `main` (Release-Stand, Standard)
  und `dev` (Arbeitsstand, `git switch dev` nach dem Klonen); der Absatz zum Release nennt
  `make -C k-playbook-local release`.
- **`docs/fortschritt.md`** nach der Anleitung am Anfang der Datei: Task 010 unter
  „Erledigt“.

### Etappe 5 — Durchlauf

Voraussetzung: Etappe 2–4 auf `origin/dev` gepusht, CI auf diesem Commit grün.

In einer Wegwerf-Kopie im Scratchpad, damit nichts auf GitHub landet:

- `git clone https://github.com/kephalaion/kephalaion.git work`, `git -C work switch dev`,
  daneben `git clone --bare work remote.git`, dann in `work`
  `git remote set-url origin ../remote.git` und `git fetch origin`.
- **sichern:** auf `main` → Abbruch; auf `dev` mit einer Änderung → Commit landet in
  `remote.git` auf `dev`.
- **release**, jeweils Abbruch mit verständlicher Meldung und nichts gepusht:
  falsche Version; auf `main`; Arbeitsbaum schmutzig; `dev` nicht gepusht; `main` in
  `remote.git` mit einem Commit, den `dev` nicht hat; Tag existiert schon auf `origin` bzw.
  lokal auf einem anderen Commit; `HEAD` ist ein Commit ohne CI-Lauf (etwa der Commit aus
  dem sichern-Test).
- Den `main`-Commit, den `dev` nicht hat, nur in `remote.git` anlegen: in `work` detached
  committen (`git switch --detach`), `git push origin <sha>:main`, zurück mit `git switch
  dev`. Nach dem Test `main` zurücksetzen (`git push --force origin <alter-sha>:main` — nur
  `remote.git`), sonst bricht der Erfolgstest in Schritt 6 ab.
- **release, Erfolg:** `dev` zurück auf den Commit, der auf GitHub grün ist (`git reset
  --hard`, dann `git push --force origin dev` — nur in die lokale `remote.git`, nie nach
  GitHub), `VERSION=v0.0.0-test` → `main` in `remote.git` steht auf `HEAD`, der Tag liegt
  dort; auf GitHub kein neuer Tag, kein Release, `main` unverändert
  (`git ls-remote https://github.com/kephalaion/kephalaion.git`).
- **Wiederholung**, Abbruch nachgestellt:
  - zwischen 9 und 10: `main` in `remote.git` von Hand auf `HEAD` ohne Tag
    (`git push origin HEAD:main`), dann `VERSION=v0.0.0-test2` → läuft durch, der Push nach
    `main` ist leer, der neue Tag liegt in `remote.git`.
  - Tag-Push in 10 gescheitert: `git tag -a v0.0.0-test3 -m test HEAD` nur lokal, dann
    `VERSION=v0.0.0-test3` → läuft durch, legt den Tag nicht neu an, er liegt danach in
    `remote.git`.
- Kopie danach löschen.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — dev anlegen | erledigt | 2026-09-26 | `dev` von lokalem `main` (a92d9e1, inkl. 51aa5ac) angelegt, nach `origin/dev` gepusht; `origin/main` bleibt auf e758ee8 |
| 2 — Makefile: sichern und release | erledigt | 2026-09-26 | `GH_REPO`, `sichern` nur auf `dev` (Push nach `origin dev`, Hinweis auf `git pull --no-rebase`), `release` in 12 Schritten; ls-remote unterscheidet „kein Tag“ (Code 2) von Fehler; Hilfe um Branches ergänzt |
| 3 — GitHub: CI-Trigger, Dependabot, Ruleset | erledigt | 2026-09-26 | `ci.yml` push nur `main`/`dev`; `dependabot.yml` `target-branch: dev` mit Hinweis zu Security-Updates; Ruleset „main und dev“ (ID 24044142, active, `deletion` + `non_fast_forward`, keine bypass_actors) angelegt, `rules/branches/main` und `…/dev` zeigen beide Regeln |
| 4 — Doku | erledigt | 2026-09-26 | `k-playbook.md`: neuer Abschnitt „Branches“, „Release“, „Sichern“, „Bauen“ nachgezogen; README „Bauen“; `docs/fortschritt.md`: Task 010 unter „Erledigt“, dazu wackelnder `TestBackgroundSync` (Zu tun) und erster Security-PR (Zu testen) |
| 5 — Durchlauf | erledigt | 2026-09-26 | In Wegwerf-Kopie gegen lokale `remote.git`: `sichern` (main, detached, dev, origin weiter) und alle `release`-Abbrüche wie erwartet ohne Push, zusätzlich CI rot und `gh` fehlt; Erfolg `v0.0.0-test` auf 1b3892c (CI grün), Wiederholungen `test2`/`test3` laufen durch; GitHub unverändert (`main` e758ee8, keine neuen Tags); Kopie gelöscht |

---
## Review-Log (2026-09-26)

**Pfad:** k-playbook-local/tasks (nur 010; 008 und 009 waren bereits gegengelesen)
**Intent:** inline
**Runden:** 1 (Fast Path)

### Diskussion
- **1/3 (Wiederholung):** Der Critic zeigte, dass die Wiederholung nur für einen Abbruch zwischen 9 und 10 durchdacht war. Scheitert in 10 der Tag-Push, steht `main` schon vorn, aber derselbe Aufruf bricht in Schritt 7 am lokalen Tag ab. Der Moderator entschied: Ein lokaler Tag auf `HEAD` wird durchgelassen und nur gepusht. Der Test in Etappe 5 stellt beide Abbrüche jetzt wirklich nach, statt nur ein zweites Release zu fahren.
- **7 (welcher CI-Lauf):** Nach Schritt 9 startet für denselben Commit ein Lauf auf `main`, der eine Wiederholung blockieren könnte. Es zählen nur Läufe auf `dev`.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| 1 | WARNUNG | 010 | Etappe 2, Schritt 7/10 | Nach gescheitertem Tag-Push ist die Wiederholung mit derselben `VERSION` versperrt, obwohl `main` schon vorn steht | lokalen Tag auf `HEAD` zulassen oder Ausweg nennen; testen |
| 2 | FEHLEND | 010 | Etappe 5, `main` mit fremdem Commit | Wie der Commit nach `main` in `remote.git` kommt und dass er vor dem Erfolgstest weg muss, fehlt | anlegen und zurücksetzen, nur in `remote.git` |
| 3 | WARNUNG | 010 | Etappe 5, Wiederholung | Der Test ist ein zweites Release, kein nachgestellter Abbruch | Abbruch gezielt herbeiführen, auch Tag-Fall |
| 4 | WARNUNG | 010 | Intent vs. Ruleset | „`main` nur per `release`“ ist Konvention, keine Sperre, steht aber nirgends | in „Branches“ sagen |
| 5 | WARNUNG | 010 | Kontext Dependabot | Automatisches Schließen der Security-PRs hält nicht, wenn Dependabot seinen Branch rebased | als Unsicherheit notieren |
| 6 | FEHLEND | 010 | Etappe 1 / parallele Sitzungen | Parallele Sitzungen landen nach `git switch -c dev` auf `dev` und sichern womöglich Halbfertiges | vor Etappe 5 grünen Stand prüfen |
| 7 | WARNUNG | 010 | Etappe 2, Schritt 8 | Lauf auf `main` für denselben Commit kann die Wiederholung blockieren | nur Läufe auf `dev` zählen |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| 1 | pass | Wiederholung war in genau dem Fall versperrt, in dem `main` schon verschoben ist | behoben |
| 2 | pass | ohne Zurücksetzen scheitert der Erfolgstest | behoben |
| 3 | pass | Test belegt sonst die Zusage nicht | behoben |
| 4 | decide | ein Halbsatz in der Doku | behoben |
| 5 | skip | ausdrücklich zurückgestellt („sehen wir uns an, wenn der erste Fall auftritt“) | offen |
| 6 | skip | Etappe 5 setzt `dev` ohnehin auf den grünen Commit zurück; ein fremder Commit auf `dev` ist ungefährlich | offen |
| 7 | pass | Wiederholung könnte grundlos blockieren | behoben |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| 1 | fixed | Schritt 7 lässt einen lokalen Tag auf `HEAD` durch (Vergleich über `^{commit}`), Schritt 10 legt nur an, wenn er fehlt; Hinweis auf alle Abbrüche nach 9 erweitert |
| 2 | fixed | Commit detached anlegen, per SHA nach `main` in `remote.git`, danach dort zurücksetzen; Test „Tag existiert schon“ auf `origin` bzw. anderen Commit präzisiert |
| 3 | fixed | beide Abbrüche nachgestellt (`main` von Hand auf `HEAD`; Tag nur lokal) |
| 4 | fixed | Halbsatz in „Branches“: Regel, keine Sperre; `release` erkennt Abweichung in Schritt 6 |
| 7 | fixed | `--branch dev`; Läufe auf `main` zählen nicht |

### Moderator-Entscheidungen
- 5 übersprungen: Der Umgang mit Security-PRs ist bewusst auf den ersten echten Fall vertagt.
- 6 übersprungen: nicht blockierend; der Durchlauf in Etappe 5 läuft ohnehin auf dem grünen Commit.
- Alle Editor-Vorschläge übernommen, keine abgelehnt.

### Intent-Alignment
Ja. Sichern auf `dev` ohne Prüfung, geschützt durch das Ruleset. `main` rückt nur per Fast-Forward auf einen gepushten, auf `dev` grünen Stand vor. Alle Prüfungen laufen vor dem ersten Push, auch ein auf GitHub gemergter PR wird in Schritt 6 erkannt, und Wiederholungen nach einem Abbruch funktionieren. `release.yml`, `install.sh` und `upgrade` bleiben unverändert, die Regel steht in den Projektregeln und im README.

### Geänderte Dateien
- 010-arbeitsbranch-dev.md: `release` Schritte 7, 8, 10 und Hinweis zur Wiederholung (1, 7), Abschnitt „Branches“ in Etappe 4 (4), Etappe 5 Abbruch-Tests und Wiederholung (2, 3)

### Offen (nicht gefixt)
- 5: automatisches Schließen der Security-PRs unsicher, wenn Dependabot rebased — auf den ersten Fall vertagt.
- 6: parallele Sitzungen auf `dev` während der Umsetzung — nicht blockierend.

## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** 2026-09-26  
**Zusammenfassung:** Branch `dev` ist lokal und auf origin angelegt. `sichern` sichert nur noch auf `dev` (`git push origin dev`, kein Force). `release` prüft alles vor dem ersten Push: `dev` gepusht, `origin/main` Vorfahre, Tag, CI auf `dev` grün über `gh`. Danach schiebt es `main` per Fast-Forward nach und legt den Tag an. Wiederholungen nach einem Abbruch funktionieren. CI läuft bei Push nur für `main` und `dev`, Dependabot hat `target-branch: dev`. Das Ruleset „main und dev“ (ID 24044142, `deletion` + `non_fast_forward`, ohne Bypass) ist aktiv. Die Doku (Projektregeln „Branches“/„Release“/„Sichern“/„Bauen“, README, `docs/fortschritt.md`) ist nachgezogen. Den Durchlauf in einer Wegwerf-Kopie haben alle Abbruchfälle, der Erfolgsfall und beide Wiederholungsfälle bestanden. GitHub blieb unverändert: `main` auf e758ee8, kein neuer Tag, kein Release.

**Hinweise:**
- Commit a92d9e1 („Task 008: abgeschlossen, Task 012 angelegt“) ist fremder Stand, auf Wunsch des Users vor dem Lauf mitgesichert.
- Commit 31d70e8 („Task 012: Refine“) stammt aus einer parallelen Sitzung auf `dev` und ist mit gepusht.
- CI auf a92d9e1 war rot: `TestBackgroundSync` wackelt, notiert in `docs/fortschritt.md`. CI auf 1b3892c und 342a2b7 ist grün.

**Geänderte Dateien** (ohne die fremde Task 012):
```
 .github/dependabot.yml                          |  11 ++
 .github/workflows/ci.yml                        |  14 ++-
 README.md                                       |   9 +-
 docs/fortschritt.md                             |  13 +-
 k-playbook-local/Makefile                       | 151 +++++++++++++++++++-----
 k-playbook-local/k-playbook.md                  |  42 +++++--
 k-playbook-local/tasks/010-arbeitsbranch-dev.md |  10 ++
 7 files changed, 207 insertions(+), 43 deletions(-)
```

**Code-Änderungen** (Auszug; vollständig: `git diff a92d9e1 -- .github k-playbook-local/Makefile README.md k-playbook-local/k-playbook.md docs/fortschritt.md`):
- `.github/workflows/ci.yml`: `push.branches` von `"**"` auf `main`, `dev`; Kommentar zum Grund (doppelte Dependabot-Läufe).
- `.github/dependabot.yml`: `target-branch: dev` für `gomod` und `github-actions`, Kommentar zu Security-Updates.
- `k-playbook-local/Makefile`: `GH_REPO := kephalaion/kephalaion`; `sichern` mit Branch-Prüfung und `git push origin dev`; `release` mit neuer Prüfkette:

```sh
  if ! git -C "$(REPO)" merge-base --is-ancestor origin/main HEAD; then \
    printf 'main hat Commits, die dev fehlen (etwa ein auf GitHub gemergter PR):\n' >&2; \
    git -C "$(REPO)" log --oneline HEAD..origin/main >&2; ...
  run="$$(gh run list -R "$(GH_REPO)" --workflow ci.yml --branch dev --commit "$$head" --limit 1 \
    --json status,conclusion,url --jq '...')" || { ... }
  ...
  git -C "$(REPO)" push origin HEAD:refs/heads/main || { ... }
  if test -z "$$tagged"; then git -C "$(REPO)" tag -a "$$version" -m "Release $$version"; fi
  git -C "$(REPO)" push origin "refs/tags/$$version" || { ... }
```

**Code-Review** (nur der Diff):

Keine kritischen Befunde. Die Prüfkette läuft vollständig vor dem ersten Push, der Push nach `main` erfolgt ohne Force, und die Wiederholbarkeit ist bedacht.

| # | Datei | Befund | Schwere |
|---|---|---|---|
| 1 | Makefile `release` | `version="$(VERSION)"` wird von make vor der Shell eingesetzt: Ein `VERSION` mit `"` oder `$(…)` würde als Shell-Code ausgeführt. Das war schon vorher so, und der Wert kommt nur vom lokalen Aufrufer. Robuster: `VERSION` exportieren und in der Shell `$$VERSION` lesen (wie `MSG`). | Niedrig |
| 2 | Makefile `sichern` | Scheitert der Push aus einem anderen Grund (Netz, Auth), rät die Meldung trotzdem zu `git pull --no-rebase`. Die git-Ausgabe steht zwar darüber, der Hinweis sollte aber „z. B. wenn dev auf origin weiter ist“ lauten. | Niedrig |
| 3 | Makefile `release`, Schritt 8 | Die CI-Prüfung hängt an einem Lauf auf `dev` für genau diesen Commit. Ein Push mit `[skip ci]` erzeugt keinen Lauf und damit einen dauerhaften Abbruch („noch kein CI-Lauf“). Das ist beabsichtigt, aber die Meldung könnte `gh workflow run ci.yml --ref dev` als Ausweg nennen. | Hinweis |
| 4 | Makefile `release`, Schritt 7 | Der Remote-Tag wird vor dem lokalen Tag geprüft. Ist der Tag-Push gelungen und erst Schritt 11 gescheitert, meldet eine Wiederholung „Tag gibt es auf origin bereits“. Das ist korrekt, weil das Release dann fertig ist; der Hinweis, dass nichts mehr zu tun ist, fehlt aber. | Hinweis |
| 5 | Makefile `sichern` | `git add -A` auf `dev` nimmt auch fremde Änderungen paralleler Sitzungen mit. So gewollt und dokumentiert. | Hinweis |

Positiv: `ls-remote --exit-code` unterscheidet jetzt „fehlt“ (2) von einem Fehler, der lokale Tag wird über `^{commit}` verglichen, `cancelled`/`skipped` gelten nicht als grün, und nach Schritt 9 gibt jeder Fehler einen konkreten Wiederholungsweg an.

**Intent-Alignment:** Ja. Jede Vorgabe des Intents ist umgesetzt und in der Wegwerf-Kopie geprüft. Dass `main` sich nur über `release` bewegt, ist eine dokumentierte Regel und keine technische Sperre; `release` erkennt Abweichungen aber in Schritt 6.
