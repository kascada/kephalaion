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
