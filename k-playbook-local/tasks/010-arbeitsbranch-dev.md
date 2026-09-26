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
  7. Tag weder lokal noch auf `origin` vorhanden (wie bisher).
  8. CI auf `HEAD` grün: `gh run list -R "$(GH_REPO)" --workflow ci.yml --commit <sha>
     --json status,conclusion,url` — der neueste Lauf zählt. Kein Lauf → Abbruch („noch
     nicht gestartet? später erneut“); läuft noch → Abbruch mit Hinweis auf `gh run watch`;
     nicht `success` → Abbruch mit URL. Fehlt `gh` → Abbruch mit Hinweis.
  9. `git push origin HEAD:refs/heads/main` — ohne Force; git lehnt alles außer
     Fast-Forward ab.
  10. Tag annotiert auf `HEAD` anlegen und pushen (wie bisher).
  11. Lokalen `main`, falls vorhanden, nachziehen (`git fetch --quiet origin main:main`);
      scheitert das, nur ein Hinweis, kein Fehler.
  12. Abschlussmeldung wie bisher, dazu: `main` steht jetzt auf `<sha>`.
  - Ein Wiederholungsaufruf nach einem Abbruch zwischen 9 und 10 muss funktionieren: `main`
    steht dann schon auf `HEAD`, Schritt 6 besteht, Schritt 9 ist ein leerer Push.
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
    `main` wird nie direkt committet oder gepusht; `main` ist Standard-Branch, ein Clone
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
  `remote.git` mit einem Commit, den `dev` nicht hat; Tag existiert schon; `HEAD` ist ein
  Commit ohne CI-Lauf (etwa der Commit aus dem sichern-Test).
- **release, Erfolg:** `dev` zurück auf den Commit, der auf GitHub grün ist (`git reset
  --hard`, dann `git push --force origin dev` — nur in die lokale `remote.git`, nie nach
  GitHub), `VERSION=v0.0.0-test` → `main` in `remote.git` steht auf `HEAD`, der Tag liegt
  dort; auf GitHub kein neuer Tag, kein Release, `main` unverändert
  (`git ls-remote https://github.com/kephalaion/kephalaion.git`).
- **Wiederholung** nach Abbruch zwischen Schritt 9 und 10: auf demselben Stand
  `VERSION=v0.0.0-test2` → läuft durch, der Push nach `main` ist leer, der neue Tag liegt in
  `remote.git`.
- Kopie danach löschen.
