---
thema: CI auf macOS
begonnen: 2026-09-26
zuletzt: 2026-09-26
status: offen
---

# CI auf macOS

Was der zweite CI-Job auf `macos-latest` (Task 011, Etappe 5) zeigt, das unter Linux nicht auffällt.

## 2026-09-26 — TestBackgroundSync scheitert auf macOS öfter als unter Linux

**Befund:** Die bekannte Race-Condition in `TestBackgroundSync` („wartet vergeblich auf: Logzeilen“, Task 013) trifft den macOS-Job in zwei von drei Versuchen desselben Laufs; Linux war in denselben Läufen grün.
**Beleg:** CI-Lauf 36259254294 (Commit 9aa96b0): Versuch 1 und 2 macOS rot mit `bgsync_test.go:116: wartet vergeblich auf: Logzeilen`, Versuch 3 (`gh run rerun --failed`) grün; Lauf 36259139097 (640ff1c) macOS ebenso rot. Ursache laut Befund in Task 013: `hub doc put` kommt vor dem Ende der ersten Runde des Abgleichs, die erste Runde loggt dann „4 Zeilen“ statt „3“ und „1 Zeile“ bleibt aus.
**Sicherheit:** bestaetigt
**Frage:** Ist der rote macOS-Job ein Fehler von Task 011 oder die bekannte Race-Condition?
**Warum es so ist:** Vermutlich braucht die erste Runde auf dem macOS-Runner länger (Replica anlegen, fsync), das Fenster für `hub doc put` ist größer. Nicht gemessen.
**Sackgassen:** Kein anderer Test scheiterte auf macOS; die übrigen Tests der Task 011 laufen dort grün (Pfade des Binarys werden in den Tests aufgelöst, weil macOS `/var` auf `/private/var` verlinkt). Bis Task 013 die Race-Condition behebt, hilft nur `gh run rerun --failed`.

<!-- sitzung: 262d4cab-642d-4b5d-9cf8-ff3d1768a0ab -->

## 2026-09-26 — Auf macOS scheitert TestBackgroundSync in den meisten Versuchen

**Befund:** Über alle Läufe der Task 011 scheiterte der macOS-Job in 13 von 15 Versuchen, jedes Mal nur an `TestBackgroundSync` (`bgsync_test.go:116`); `plutil -lint` und `install.sh` liefen in beiden grünen Versuchen durch, alle übrigen Tests waren in jedem Versuch grün.
**Beleg:** Läufe 36259139097 (1 Versuch rot), 36259254294 (2 rot, 3. grün), 36260080975 (2 rot), 36260519391 (2 rot), 36260759843 (6 rot); grün 36260980326 (Commit 76b70e0, erster Versuch). `gh run view <id> --log-failed` zeigt jeweils nur `--- FAIL: TestBackgroundSync … wartet vergeblich auf: Logzeilen`.
**Sicherheit:** bestaetigt
**Frage:** Ist das noch ein Wackler, den `gh run rerun --failed` überbrückt?
**Warum es so ist:** Die Race-Condition aus Task 013 (erste Runde gegen `hub doc put`) — auf dem macOS-Runner ist die erste Runde offenbar fast immer langsamer als das `put`. Nicht gemessen.
**Sackgassen:** Wiederholen hilft auf macOS kaum (2 von 15). Die Änderungen der Task 011 an `serve` (Frage nach Updates beim Start) laufen vor dem `ready` bzw. nebenher; der grüne Versuch in 36259254294 enthielt sie schon — sie sind also nicht die Ursache. Nutzervorgabe (Koordinator, 2026-09-26): CI-Fehler an `TestBackgroundSync` bis Task 013 ignorieren, nicht wiederholen, den Test nicht ändern.

<!-- sitzung: 262d4cab-642d-4b5d-9cf8-ff3d1768a0ab -->

## 2026-09-26 — Die Race-Condition in TestBackgroundSync ist behoben, der macOS-Job bleibt aus

**Befund:** `TestBackgroundSync` wartet jetzt vor `hub doc put` auf die erste Runde von `eigen` und `fern` (`hub_sync.OKAt != 0`); mit derselben künstlichen Verzögerung, die den Fehler vorher jedes Mal auslöste, ist er grün.
**Beleg:** Wegwerf-Kopie mit `time.Sleep(500 * time.Millisecond)` am Anfang von `syncOne` (`cmd/kephalaion/bgsync.go`): alter Stand `go test -count=1 -run 'TestBackgroundSync$' ./cmd/kephalaion` rot, „wartet vergeblich auf: Logzeilen“, Log zeigt „Abgleich eigen: 4 Zeilen, Revision 5“; neuer Stand (Commit c2daf33) `-count=3 -run TestBackgroundSync` alle fünf `TestBackgroundSync*` grün, ganzes `cmd/kephalaion` grün.
**Sicherheit:** bestaetigt
**Frage:** Liegt der Fehler im Test oder im Abgleich, und reicht es, die erste Runde abzuwarten?
**Warum es so ist:** `serve` startet den Abgleich vor dem ready-Callback, die erste Runde läuft asynchron; ohne Warten kamen Account-Zeilen und Dokument in einer Runde.
**Sackgassen:** Die übrigen `TestBackgroundSync*` haben diese Race nicht: `Errors` zählt Fehlerzeilen erst nach weiteren Runden (die Logzeile einer Runde steht, bevor die nächste startet), `Off` legt das Dokument vor `serve` an, `RemoveAddDuringSync` und `Shutdown` halten den ersten Abgleich über http selbst an. Auf macOS nicht nachgeprüft: Der Job bleibt auf Wunsch des Nutzers aus.

<!-- sitzung: d52d0ec3-d3db-4473-a6ce-fa9ae90f265c -->
