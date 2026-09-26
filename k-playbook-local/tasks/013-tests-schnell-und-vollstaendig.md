# Task 013 — Tests schnell und vollständig, CI je Branch

Die Tests werden in schnell und langsam geteilt; `dev` bekommt in CI nur die schnellen,
`main`, Pull Requests und `release` die vollständigen — und `TestBackgroundSync` wackelt
nicht mehr.

## Intent

`dev` ist Zwischenspeicher und soll schnell und ohne zufällige Fehlschläge durch CI kommen;
was veröffentlicht wird, ist trotzdem vollständig geprüft, bevor `main` sich bewegt.
- Ein Push auf `dev` löst nur die schnellen Tests aus (`go test -short`), `make dist` und
  shellcheck bleiben.
- Push auf `main`, Pull Requests und `workflow_dispatch` (Vorgabe) laufen vollständig.
- `release` schiebt `main` nur, wenn für genau `HEAD` ein vollständiger CI-Lauf auf `dev`
  grün ist; alle Prüfungen bleiben vor dem ersten Push.
- `TestBackgroundSync` hat keine Race-Condition mehr zwischen erster Runde und `hub doc put`.
- Kein Test verschwindet still: langsame Tests werden weiter kompiliert und von `go vet`
  gesehen.

## Referenzen

- `cmd/kephalaion/bgsync_test.go` — `TestBackgroundSync`, Race-Condition bei Zeile ~114
- `cmd/kephalaion/serve.go` (~200–209) — Abgleich startet vor dem ready-Callback
- `cmd/kephalaion/serve_test.go` — `startServe`
- `cmd/kephalaion/nodeaccount_test.go` — `newCommEnv` (drei Account-Zeilen in team-x)
- `Makefile` — Targets `test`, `check`
- `.github/workflows/ci.yml` — Trigger und Schritte
- `.github/workflows/release.yml` — ruft `make check` (bleibt vollständig)
- `k-playbook-local/Makefile` — Target `release`, CI-Prüfung über `gh run list`
- `k-playbook-local/k-playbook.md` — „Testen“, „Branches“, „Release“
- `k-playbook-local/tasks/done/010-arbeitsbranch-dev.md` — Herkunft von release und CI-Regel
- `docs/fortschritt.md` — Punkt „`TestBackgroundSync` wackelt in CI“, Review-Punkt 11 zu Task 008

## Ziel

1. Race-Condition in `TestBackgroundSync` beheben.
2. Tests in schnell und langsam teilen, über `-short`.
3. CI: `dev` schnell, alles andere vollständig.
4. `release` stößt den vollständigen Lauf auf `dev` selbst an und wartet darauf.
5. Doku nachziehen.

## Kontext

- **Befund (2026-09-26, nachgestellt):** `newCommEnv` legt drei `SYSTEM:A:`-Zeilen in
  `team-x` an (alice, bob, carol). `serve` startet den Abgleich im Hintergrund vor dem
  ready-Callback; die erste Runde läuft asynchron. Kommt `hub doc put team-x a.md` vor der
  ersten Runde, loggt diese „4 Zeilen, Revision 5“ statt „3 Zeilen“ und danach „1 Zeile“ —
  die erwartete Zeile „Abgleich eigen: 1 Zeile, Revision“ kommt nie. Nachgestellt durch
  500 ms Verzögerung in `syncOne`: scheitert jedes Mal mit „wartet vergeblich auf:
  Logzeilen“, wie in CI (Lauf 36252944318). Der Fehler liegt im Test, nicht im Abgleich.
- **Laufzeiten (letzter grüner CI-Lauf):** `cmd/kephalaion` 19,5 s, alle `internal/...`
  zusammen etwa 3 s. Die Zeit steckt in Tests, die `serve` starten und auf echte Zeit warten.
- **`-short` statt Build-Tag:** Ein Build-Tag blendet Tests aus dem Kompilieren aus, ein
  Umbau bricht sie dann unbemerkt. `-short` ist Go-Standard.
- **Das Repo ist öffentlich,** Actions-Minuten kosten nichts; es geht um schnelles,
  verlässliches CI auf `dev`, nicht um Kosten.
- Push auf `main` passiert nur über `release`; dort läuft dann noch einmal vollständig —
  gewollt, obwohl doppelt.
- Task 012 ändert parallel `serve`/`bgsync` und `mcpnode`; diese Task erst nach 012 ausführen.

## Zu bauen

### Etappe 1 — Race-Condition in TestBackgroundSync

- Vor `hub doc put` warten, bis `eigen` und `fern` die erste Runde abgeschlossen haben
  (`syncStatus(…).OKAt != 0`); die Prüfung auf „1 Zeile“ bleibt so streng.
- `eventually`-Prüfungen auf den Log geben beim Fehlschlag den Log aus (bisher kam in CI
  nur „wartet vergeblich“).
- Die übrigen `TestBackgroundSync*` auf dieselbe Art Race-Condition prüfen.
- Nachweis: die Verzögerung aus dem Befund in einer Wegwerf-Kopie einbauen; der Test muss
  damit grün sein.

### Etappe 2 — Aufteilung über -short

- Langsam ist ein Test, der `serve` startet oder auf echte Zeitabstände wartet (Runden,
  Timer, Logzeilen). Mit `go test -v` messen, Liste festhalten.
- Diese Tests beginnen mit `if testing.Short() { t.Skip("langsam: …") }`, am besten über
  einen gemeinsamen Helfer.
- `Makefile`: neues Target `check-quick` (wie `check`, aber `go test -short ./...`);
  `check` bleibt vollständig. Laufzeit beider vorher/nachher festhalten.

### Etappe 3 — CI je Branch

- `ci.yml`: Push auf `dev` → `make check-quick`; Push auf `main`, `pull_request` →
  `make check`; `workflow_dispatch` mit Input (etwa `suite: quick|full`, Vorgabe `full`).
- Der Lauf muss erkennbar machen, ob er vollständig war (etwa über `run-name`), damit
  `release` ihn finden kann.
- Kommentar oben in `ci.yml` nachziehen.

### Etappe 4 — release verlangt den vollständigen Lauf

- `release` sucht für `HEAD` einen grünen vollständigen Lauf auf `dev`. Fehlt er, stößt es
  ihn an (`gh workflow run ci.yml --ref dev -f suite=full`), findet den Lauf zuverlässig
  (nicht einfach „den neuesten“) und wartet (`gh run watch --exit-status`); scheitert er,
  Abbruch vor jedem Push. Ein schneller Lauf zählt nicht.
- Weiter gilt: alle Prüfungen vor dem ersten Push, wiederholbar.
- Nachweis in einer Wegwerf-Kopie oder einem Trockenlauf; kein echtes Release.

### Etappe 5 — Doku

- `k-playbook-local/k-playbook.md`: „Testen“ (vor jedem Commit `make check-quick`,
  `make check` vollständig, was als langsam gilt), „Branches“, „Release“.
- `docs/fortschritt.md`: Punkt „`TestBackgroundSync` wackelt“ und Review-Punkt 11 von
  Task 008 als erledigt.
