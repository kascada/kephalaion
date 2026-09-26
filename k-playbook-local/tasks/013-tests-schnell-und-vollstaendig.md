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
- `k-playbook-local/Makefile` — Target `release`, CI-Prüfung über `gh run list`, Kommentar
  darüber, `help`
- `k-playbook-local/k-playbook.md` — „Testen“, „Branches“, „Release“
- `README.md` — Abschnitt „Bauen“ (`make check`, Release „gepusht, CI grün“)
- `k-playbook-local/tasks/done/010-arbeitsbranch-dev.md` — Herkunft von release und CI-Regel
- `docs/fortschritt.md` — Punkt „`TestBackgroundSync` wackelt in CI“, Review-Punkt 11 zu
  Task 008 (Zeile ~131)

## Ziel

1. Race-Condition in `TestBackgroundSync` beheben.
2. Tests in schnell und langsam teilen, über `-short`.
3. CI: `dev` schnell, alles andere vollständig.
4. `release` verlangt den vollständigen Lauf auf `dev`, stößt ihn bei Bedarf selbst an und
   wartet darauf.
5. Doku nachziehen.

## Kontext

- **Vorbedingung:** Task 012 liegt in `done/`; sonst nicht beginnen. 012 ändert
  `serve`/`bgsync` und `mcpnode` und bringt neue `serve`-Tests; sie gehören in die Messung
  von Etappe 2.
- **Befund (2026-09-26, nachgestellt):** `newCommEnv` legt drei `SYSTEM:A:`-Zeilen in
  `team-x` an (alice, bob, carol). `serve` startet den Abgleich im Hintergrund vor dem
  ready-Callback; die erste Runde läuft asynchron. Kommt `hub doc put team-x a.md` vor der
  ersten Runde, loggt diese „4 Zeilen, Revision 5“ statt „3 Zeilen“ und danach „1 Zeile“ —
  die erwartete Zeile „Abgleich eigen: 1 Zeile, Revision“ kommt nie. Nachgestellt durch
  500 ms Verzögerung in `syncOne`: scheitert jedes Mal mit „wartet vergeblich auf:
  Logzeilen“, wie in CI (Lauf 36252944318). Der Fehler liegt im Test, nicht im Abgleich.
- **Laufzeiten (letzter grüner CI-Lauf):** `cmd/kephalaion` 19,5 s, alle `internal/...`
  zusammen etwa 3 s. Die Zeit steckt in wenigen Tests, die auf echte Zeit warten; lokal
  gemessen (2026-09-26, Review): `TestBackgroundSyncErrors` ~9 s, `TestBackgroundSync`
  ~6,5 s, `TestServe` ~5,9 s, `TestBackgroundSyncRemoveAddDuringSync` ~1,2 s,
  `TestBackgroundSyncOff` ~0,8 s. `TestNodeWhoami*`, `TestMCPWhoami*` und
  `TestBackgroundSyncShutdown` starten `serve`, brauchen aber etwa 0,2 s.
- **`-short` statt Build-Tag:** Ein Build-Tag blendet Tests aus dem Kompilieren aus, ein
  Umbau bricht sie dann unbemerkt. `-short` ist Go-Standard.
- **Das Repo ist öffentlich,** Actions-Minuten kosten nichts; es geht um schnelles,
  verlässliches CI auf `dev`, nicht um Kosten.
- Push auf `main` passiert nur über `release`; dort läuft dann noch einmal vollständig —
  gewollt, obwohl doppelt.
- `GH_REPO` in `k-playbook-local/Makefile` ist fest `kephalaion/kephalaion`: Ein `release`
  in einer Kopie spricht ohne Stub das echte Repo an.

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

- Langsam ist, was die Messung als langsam ausweist, nicht die Struktur: Richtwert mehr als
  etwa 0,5 s, oder der Test wartet auf `sync_interval`, Timer oder Runden. Dass ein Test
  `serve` startet, reicht allein nicht. Mit `go test -v` messen, einschließlich der Tests
  aus 012; Liste mit Zeiten festhalten.
- Diese Tests beginnen mit `if testing.Short() { t.Skip("langsam: …") }`, über einen
  gemeinsamen Helfer, je Test aufgerufen — nicht in `startServe`.
- `Makefile`: neues Target `check-quick` (wie `check`, aber `go test -short ./...`);
  `check` bleibt vollständig. Laufzeit beider vorher/nachher festhalten.

### Etappe 3 — CI je Branch

- `ci.yml`: Push auf `dev` → `make check-quick`; Push auf `main`, `pull_request` →
  `make check`; `workflow_dispatch` mit Input (etwa `suite: quick|full`, Vorgabe `full`).
- Der Lauf muss erkennbar machen, ob er vollständig war (etwa über `run-name`), damit
  `release` ihn finden kann.
- Kommentar oben in `ci.yml` nachziehen.

### Etappe 4 — release verlangt den vollständigen Lauf

- Voraussetzung: Etappe 3 ist auf `origin/dev` gepusht (Push auf `dev` ist frei); erst dann
  kennt `ci.yml` den Input `suite`.
- Es zählt der neueste vollständige Lauf von `ci.yml` auf `dev` für `HEAD`. Ein schneller
  Lauf zählt nicht.
  - Keiner vorhanden: `release` stößt ihn an (`gh workflow run ci.yml --ref dev -f
    suite=full`).
  - Läuft er noch: `release` beobachtet ihn, statt einen zweiten anzustoßen.
  - Rot: Abbruch mit URL. Ein neuer Versuch nur ausdrücklich (`gh run rerun <id>`, danach
    `release` erneut); `release` wiederholt nie selbst.
- Den angestoßenen Lauf über die URL finden, die `gh workflow run` zurückgibt (gh ≥ 2.98),
  als Rückfall über eine Kennung im `run-name` — nicht einfach „den neuesten“.
- Der Lauf zählt nur, wenn sein `headSha` gleich `HEAD` ist, sonst Abbruch: `--ref dev`
  nimmt die Spitze von `dev` beim Anstoß, und `dev` kann inzwischen weitergerückt sein.
- Warten mit `gh run watch --exit-status`; scheitert der Lauf, Abbruch vor jedem Push.
- Die CI-Prüfung ist die letzte vor dem Push. Nach dem Warten `git fetch` und die
  Remote-Prüfungen wiederholen (`HEAD == origin/dev`, `origin/main` Vorfahre von `HEAD`,
  Tag nicht auf `origin`).
- Weiter gilt: alle Prüfungen vor dem ersten Push, wiederholbar.
- Nachweis in einer Wegwerf-Kopie mit lokalem Bare-Repo als `origin` und einem `gh`-Stub
  über `PATH`, je Pfad: kein Lauf, läuft noch, rot, nur schneller Lauf, falscher `headSha`,
  grün. Ein echter Anstoß eines vollständigen Laufs auf `dev` ist erlaubt (CI ändert und
  veröffentlicht nichts); kein Push nach `main` auf GitHub, kein Tag, kein echtes Release.

### Etappe 5 — Doku

- `k-playbook-local/k-playbook.md`: „Testen“ (Zwischenstände: `make check-quick`; vor dem
  Abschluss einer Task, auch unter `/k-task-run`, und bei Änderungen an `serve`/`bgsync`
  vollständig `make check`; was als langsam gilt), „Branches“, „Release“. In „Release“ den
  Satz „es zählt der neueste Lauf auf `dev`“ durch die Regel aus Etappe 4 ersetzen.
- `README.md`, Abschnitt „Bauen“: `make check-quick` neben `make check`; der Release-Satz
  „(gepusht, CI grün)“ nennt den vollständigen Lauf.
- `k-playbook-local/Makefile`: Kommentar über `release` („grüne CI“), falls nötig `help`.
- `docs/fortschritt.md`: Punkt „`TestBackgroundSync` wackelt“ als erledigt. Bei
  Review-Punkt 11 von Task 008 festhalten, dass die Langsamkeit über `-short` gelöst ist
  (langsame Log-Tests laufen nur noch im vollständigen Lauf); der Unit-Test der
  Log-Zustandsmaschine mit gefälschtem Connector bleibt als offener Rest stehen.

## Fortschritt

| Etappe | Status | Datum | Notiz |
|---|---|---|---|
| 1 — Race-Condition in TestBackgroundSync | erledigt | 2026-09-26 | Wartet vor `hub doc put` auf `OKAt` von eigen und fern; `eventuallyLog` gibt den Log von serve aus (bgsync, nodewhoami, updatecheck). Übrige `TestBackgroundSync*` ohne diese Race. Nachweis in Wegwerf-Kopie mit 500 ms in `syncOne`: vorher rot („4 Zeilen, Revision 5“), danach 3× grün, `cmd/kephalaion` ganz grün |
| 2 — Aufteilung über -short | offen | | |
| 3 — CI je Branch | offen | | |
| 4 — release verlangt den vollständigen Lauf | offen | | |
| 5 — Doku | offen | | |

---
## Review-Log (2026-09-26)

**Pfad:** k-playbook-local/tasks/013-tests-schnell-und-vollstaendig.md
**Intent:** inline (`## Intent` in 013)
**Runden:** 2

### Diskussion
- **Review-Punkt 11 (Task 008):** Der Critic stellte fest, dass 013 den Punkt als erledigt
  abhaken wollte, ohne den geforderten Unit-Test der Log-Zustandsmaschine zu bauen. Auf `dev`
  wäre die Zustandsmaschine danach gar nicht mehr geprüft worden. Der Nutzer entschied, den
  Punkt offen zu lassen: 013 hakt nur die Langsamkeit ab, der Unit-Test bleibt als Rest stehen.
- **Kriterium „langsam“:** Laut Aufgabe hätte „startet `serve`“ gereicht. Der Critic hat
  gemessen: Die whoami- und MCP-Tests starten `serve`, brauchen aber nur ~0,2 s. Mit dem
  Strukturkriterium hätte `dev` Abdeckung verloren und keine Zeit gewonnen. Umgestellt auf
  Messung und Skip je Test, nicht in `startServe`.
- **Den Lauf von `release` finden:** `gh workflow run --ref dev` läuft auf der Spitze von
  `dev`, nicht auf einem SHA, und `dev` rückt durch parallele Tasks frei vor. Zählen soll
  deshalb nur ein Lauf mit `headSha == HEAD`. Nach dem Warten werden die Remote-Prüfungen
  wiederholt. Für schon vorhandene Läufe gilt die Regel „neuester vollständiger Lauf zählt,
  ein roter Lauf wird nie selbst wiederholt“. Sonst hätte `release` einen wackelnden Test so
  lange wiederholt, bis er einmal grün ist.
- **Intent-Satz (E-4a):** Der Editor wollte „ein vollständiger“ in „der neueste
  vollständige“ ändern. Abgelehnt: Die strengere Regel in Etappe 4 erfüllt den Intent
  bereits. Der Critic stimmte in Runde 2 zu.

### Critic-Issues
| ID | Kategorie | Datei | Stelle | Problem | Empfehlung |
|---|---|---|---|---|---|
| 1 | FEHLER | 013 | Etappe 5, Punkt 11 als erledigt | Unit-Test der Log-Zustandsmaschine wird nicht gebaut; auf `dev` fällt die Prüfung ganz weg | Test ergänzen oder Punkt 11 offen lassen |
| 2 | WARNUNG | 013 | Etappe 2, Kriterium „startet `serve`“ | nur wenige Tests wirklich langsam; whoami/MCP ~0,2 s gingen `dev` verloren | Messung entscheidet; Skip je Test, nicht in `startServe` |
| 3 | WARNUNG | 013 | Etappe 4, Lauf finden | `--ref dev` nimmt die Spitze von `dev`, nicht `HEAD`; Warten zwischen Prüfung und Push | `headSha == HEAD` verlangen; URL aus `gh workflow run`; Remote-Prüfungen nach dem Warten wiederholen |
| 4 | FEHLEND | 013 | Etappe 4, vorhandene Läufe | offen, was bei laufendem, rotem oder mehreren Läufen gilt; stilles Wiederholen möglich | neuester vollständiger Lauf zählt; läuft → beobachten; rot → Abbruch |
| 5 | FEHLEND | 013 | Etappe 4, Nachweis | `GH_REPO` fest; Kopie stößt echte Läufe an; Input `suite` wirkt erst auf `origin/dev` | Etappe 3 vorher pushen; `gh`-Stub; echten Anstoß festlegen |
| 6 | WARNUNG | 013 | Etappe 5, „vor jedem Commit `check-quick`“ | langsame Tests liefen erst bei `release` | `make check` vor Abschluss einer Task und bei `serve`/`bgsync` |
| 7 | FEHLEND | 013 | Kontext „erst nach 012“ | nur Hinweis, keine Vorbedingung; neue Tests aus 012 | 012 in `done/` als Vorbedingung; in Messung aufnehmen |
| 8 | FEHLEND | 013 | Etappe 5, Doku-Umfang | `README.md`, `k-playbook-local/Makefile`, Satz „neuester Lauf“ in `k-playbook.md` | aufnehmen |
| 9 | WARNUNG | 013 | Intent „kein Test verschwindet still“ | Skips unter `-short` ohne `-v` unsichtbar | Skips ausgeben oder vollständigen Lauf als Kontrolle festhalten |

### Moderator-Routing
| ID | Route | Begründung | Ergebnis |
|---|---|---|---|
| 1 | ask-user + pass | Umfangsfrage; Nutzer: offen lassen | behoben |
| 2 | pass | sonst Verlust an Abdeckung ohne Zeitgewinn | behoben |
| 3 | pass | Kern des Intents „für genau `HEAD`“ | behoben |
| 4 | decide + pass | Fortschreibung der bisherigen Regel „neuester Lauf zählt“ | behoben |
| 5 | decide + pass | echter Anstoß auf `dev` erlaubt, kein Push nach `main`, kein Tag | behoben |
| 6 | ask-user + pass | Ablauffrage; Nutzer: `make check` vor Abschluss einer Task | behoben |
| 7 | pass | Reihenfolge prüfbar machen | behoben |
| 8 | pass | Docs-Sync | behoben |
| 9 | skip | Intent definiert „nicht still“ als kompiliert + gevettet; vollständiger Lauf bei `release` kontrolliert | vom Critic in Runde 2 akzeptiert |

### Editor-Entscheidungen
| ID | Aktion | Begründung |
|---|---|---|
| 1 | behoben | Etappe 5: nur die Langsamkeit abhaken, der Unit-Test bleibt als offener Rest; Referenz mit Zeile |
| 2 | behoben | Etappe 2: Messung statt Struktur, Richtwert > 0,5 s, Skip je Test; Messwerte im Kontext |
| 3 | behoben | Etappe 4: `headSha == HEAD`, URL aus `gh workflow run`, Rückfall `run-name`; CI-Prüfung zuletzt, danach Remote-Prüfungen wiederholen |
| 4 | behoben | Etappe 4: Regel mit Unterpunkten; Ziel 4 „bei Bedarf“; Intent-Satz geändert (vom Moderator abgelehnt) |
| 5 | behoben | Etappe 4: Voraussetzung Etappe 3 auf `origin/dev`; Nachweis mit Bare-Repo und `gh`-Stub je Pfad; `GH_REPO` im Kontext |
| 6 | behoben | Etappe 5 „Testen“ nach Nutzerentscheidung |
| 7 | behoben | Kontext: Vorbedingung 012 in `done/`; Etappe 2 misst deren Tests mit |
| 8 | behoben | Etappe 5 und Referenzen: `README.md`, `k-playbook-local/Makefile`, Satz in „Release“ ersetzen |

### Moderator-Entscheidungen
- E-4a: Die Änderung des Intent-Satzes durch den Editor ist abgelehnt. Der Intent ist die
  Vorgabe des Nutzers, und die strengere Regel in Etappe 4 erfüllt ihn bereits.
- E-2a: Die Einzelzeiten sind als „lokal gemessen (2026-09-26, Review)“ gekennzeichnet, nicht
  als Werte aus dem CI-Lauf.
- 4: Es zählt der neueste vollständige Lauf für `HEAD`. Läuft er noch, wird er beobachtet. Ist
  er rot, bricht `release` ab; neu versucht wird nur ausdrücklich per `gh run rerun`.
- 5: Ein echter Anstoß eines vollständigen Laufs auf `dev` ist für den Nachweis erlaubt. Ein
  Push nach `main` auf GitHub, ein Tag oder ein Release sind es nicht.
- 9 übersprungen (siehe Routing).
- Nutzerentscheidungen (2026-09-26): Punkt 11 bleibt offen (1). `make check` läuft vor dem
  Abschluss einer Task und bei Änderungen an `serve`/`bgsync` (6).

### Intent-Alignment
Ja. Jeder Intent-Punkt ist durch eine Etappe mit Nachweis abgedeckt; Task 012 liegt inzwischen
in `done/`. Kleine Unschärfen, kein Hindernis, siehe „Offen“.

### Geänderte Dateien
- 013-tests-schnell-und-vollstaendig.md: Punkt 11 bleibt offen (1), Kriterium „langsam“ über
  Messung (2), Regeln für den CI-Lauf in `release`: `headSha`, vorhandene Läufe,
  Remote-Prüfungen nach dem Warten (3, 4), Voraussetzung und Nachweis mit `gh`-Stub (5),
  `make check` vor Abschluss einer Task (6), Vorbedingung 012 (7), Doku-Umfang `README.md`
  und `k-playbook-local/Makefile` (8)

### Offen (nicht gefixt)
- 9: übersprungene Tests unter `-short` nicht sichtbar; bewusst, siehe Routing.
- Alignment-Unschärfe: Für den Rückfall „Kennung im `run-name`“ braucht `ci.yml` einen
  eigenen Input. Etappe 3 nennt nur `suite`; der Hauptweg über die URL genügt.
- Alignment-Unschärfe: `check-quick` in `.PHONY` und mit `##`-Hilfetext; ein
  Umsetzungsdetail.
