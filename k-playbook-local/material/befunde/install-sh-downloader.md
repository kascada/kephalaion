---
thema: install.sh — Verhalten von curl, GNU wget und busybox wget bei HTTP-Fehlern
begonnen: 2026-09-25
zuletzt: 2026-09-25
status: geklaert
---

# install.sh — Verhalten von curl, GNU wget und busybox wget bei HTTP-Fehlern

Woran erkennt install.sh ein Rate-Limit der GitHub-API, je nach Downloader?

## 2026-09-25 — busybox wget liefert bei einem HTTP-Fehler weder Antwortköpfe noch Body

**Befund:** Bei 403 schreibt busybox wget mit `-S` nur die Statuszeile nach stderr, keine Köpfe (also kein `X-RateLimit-Remaining`) und legt die `-O`-Datei gar nicht an; curl (`-D`) und GNU wget (`-S`) liefern die Köpfe.
**Beleg:** `busybox wget -q -S -O o http://127.0.0.1:18766/x` gegen einen Testserver mit 403 und `X-RateLimit-Remaining: 0` → Ausgabe nur `HTTP/1.0 403 Forbidden`, `o` existiert nicht.
**Sicherheit:** bestaetigt
**Frage:** Reicht der Kopf `X-RateLimit-Remaining: 0`, um in install.sh das Rate-Limit zu erkennen?
**Warum es so ist:** busybox wget bricht bei Status ≥ 400 vor dem Lesen der Köpfe ab.
**Sackgassen:** Nur auf den Kopf zu prüfen fällt unter busybox auf „Zugriff verweigert“ zurück; auf den Body („rate limit“) zu prüfen ebenso, weil es keinen gibt. Deshalb gilt in install.sh ein 403 von `api.github.com` stets als Rate-Limit — für ein öffentliches Repo der einzige realistische Grund.

<!-- sitzung: task-001-geruest-2026-09-25 -->
