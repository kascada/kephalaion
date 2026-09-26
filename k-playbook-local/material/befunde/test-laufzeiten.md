---
thema: Laufzeit der Tests
begonnen: 2026-09-26
zuletzt: 2026-09-26
status: offen
---

# Laufzeit der Tests

Wohin die Zeit in `go test ./...` geht und was davon nur scheinbar langsam ist (Task 013).

## 2026-09-26 — TestServe wartet oft 5 s im Shutdown auf eine ungenutzte Verbindung

**Befund:** `TestServe` braucht 0,05 s oder etwa 5,8–6,3 s; die 5 s wartet `http.Server.Shutdown` auf eine Verbindung im Zustand `StateNew`, die der Client von `http.DefaultClient` aufgebaut, aber nie benutzt hat.
**Beleg:** `go test -count=5 -v -run 'TestServe$' ./cmd/kephalaion`: 0,06 / 0,05 / 0,05 / 0,05 / 5,83 s. In einer Kopie mit `http.DefaultClient.CloseIdleConnections()` vor `srv.stop(t)`: dreimal 0,05 s. `net/http/server.go:3309–3312` (Go 1.27.1): `StateNew` gilt erst nach 5 s als idle („Issue 22682“).
**Sicherheit:** bestaetigt
**Frage:** Ist `TestServe` langsam, weil `serve` langsam beendet, oder wegen des Tests?
**Warum es so ist:** Der Transport wählt beim nächsten Request oft eine frei gewordene Verbindung, während eine neu gewählte schon aufgebaut wird; die neue bleibt ungenutzt im Pool.
**Sackgassen:** Weder Sperre noch zweiter `serve` noch der Abgleich kosten Zeit (keine Hub-Einträge). Task 013 markiert den Test als langsam (`slow`); ein eigener Client mit `CloseIdleConnections` vor dem Beenden machte ihn schnell, ist aber nicht umgesetzt (nicht Teil der Task).

<!-- sitzung: d52d0ec3-d3db-4473-a6ce-fa9ae90f265c -->

## 2026-09-26 — TestHTTPStatus wartet 0,5 s fest in net/http, lokale Zeiten schwanken stark

**Befund:** `TestHTTPStatus` (`internal/hub/replication`) braucht stabil 0,55 s, fast alles im Fall „zu groß“ (413 vor dem Lesen des Bodys): net/http wartet dann `rstAvoidanceDelay` = 500 ms vor dem Schließen. Lokale Laufzeiten ganzer Pakete schwanken um das Zwei- bis Vierfache (Plattenlast unter WSL2).
**Beleg:** `go test -count=3 -v -run 'TestHTTPStatus$' ./internal/hub/replication`: 0,55 / 0,54 / 0,55 s; CPU-Profil ohne Samples (reines Warten); `net/http/server.go:1806`. `GOFLAGS=-count=1 make check-quick` einmal 9,0 s, einmal 29,3 s, dabei alle Pakete gleich stark verlangsamt; Tests wie `TestNodeHubCreateAndCheck` 1,33 s im vollen Lauf, allein 0,13–0,22 s.
**Sicherheit:** bestaetigt
**Frage:** Welche Tests sind nach dem Kriterium von Task 013 langsam?
**Sackgassen:** `TestHTTPStatus` bleibt schnell: Die Zeit ist ein fester Verzug der Standardbibliothek, kein Warten auf Runden, und das Paket läuft parallel zu `cmd/kephalaion`, spart also keine Laufzeit. Einzelzeiten aus einem vollen Lauf taugen nicht zur Einstufung — allein messen.

<!-- sitzung: d52d0ec3-d3db-4473-a6ce-fa9ae90f265c -->
