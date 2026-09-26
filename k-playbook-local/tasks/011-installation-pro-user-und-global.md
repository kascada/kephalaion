# Task 011 — Installation pro User und global: config, Dienst, Upgrade, Doku

Kephalaion lässt sich ohne Clone auf zwei Arten betreiben — pro User und global für alle
User eines Linux-Rechners — und sagt selbst, ob und wie es aktualisiert wird.

## Intent

Wer Kephalaion installiert, kommt ohne Git und Go aus, pro User ebenso wie global per
Ansible, und erfährt jederzeit, ob es eine neue Version gibt und wie er sie bekommt.
- Pro User: `install.sh`, dann ein Kommando für den Dienst (systemd `--user`, auf macOS
  LaunchAgent); `upgrade` ersetzt das Binary und startet den laufenden Dienst neu.
- Global (nur Linux): Nach `docs/installation.md` richten Ansible oder ein Mensch Binary,
  Systembenutzer, config, Datenbanken und System-Unit ein, ohne dass ein Token durch Ansible
  geht; jeder User des Rechners erreicht den Node über Loopback.
- Die config wird in beiden Arten ohne Angabe gefunden; zwei Arten auf einem Rechner lässt
  Kephalaion nicht zu.
- `upgrade --check` (auch `--json`) und `whoami` sagen, ob es eine neue Version gibt, ob sich
  dieses Binary selbst ersetzen kann und wie das Upgrade sonst geht — über MCP ohne Anfrage an
  GitHub je Aufruf.
- README und `docs/installation.md` beschreiben beide Arten, den Dienst und das Upgrade so,
  dass Mensch, Ansible und KI ohne Rückfrage auskommen.

## Referenzen

- `docs/konzept.md` — **„Installation und Betrieb“** (maßgeblich, entschieden 2026-09-26);
  „Ein Programm, zwei Rollen“ (config, `init`); „Speicherung“ (Orte nach XDG, globale
  Installation unter `/var/lib/kephalaion`); „Kommunikation“ (Loopback, Devcontainer).
- `docs/begriffe.md` — user installation, system installation, service, upgrade, install.sh,
  config, serve, status, init.
- `internal/config/config.go` — `Path`, `DataDir`.
- `internal/upgrade/` — `Upgrader`, `Options.Check`, Tests über `httptest`.
- `cmd/kephalaion/main.go` (Kommandos, `upgradeUsage`), `roles.go` (`runInit`, `runStatus`,
  `printConfigLine`), `serve.go`, `bgsync.go` (Abgleich im Hintergrund, Task 008),
  `nodewhoamicmd.go`; `internal/node/mcpnode/whoami.go`.
- `install.sh`, `README.md` (Installation, Upgrade), `.github/workflows/ci.yml`.
- `k-playbook-local/k-playbook.md` — Aufbau, Regeln, Testen.

## Tools

- `docker` über Bash — Wegwerf-Container für den Durchlauf der globalen Installation
  (Docker 29 läuft auf diesem Rechner).
- `ansible-playbook` über Bash (`~/.local/bin`) — Syntaxprüfung und Lauf des Beispiels aus
  der Doku gegen den Container.
- `systemd-analyze` über Bash — Prüfung der erzeugten Units.
- **Nicht** freigegeben: `sudo` auf diesem Rechner; einen Dienst dieses Users einrichten oder
  starten nur nach Rückfrage beim Nutzer (Etappe 7); Ansible-Collections installieren nur
  nach Rückfrage.

## Ziel

1. Die config wird gefunden: `--config` > `KEPHALAION_CONFIG` > config des Users, wenn es sie
   gibt > `/etc/kephalaion/config.yaml`, wenn es sie gibt > Ort des Users (dort legt `init`
   an). `status` nennt Ort und Quelle; `init` pro User bricht ab, wenn es die globale config
   gibt.
2. `kephalaion service install|uninstall|status` richtet den Dienst pro User ein (systemd
   `--user`, auf macOS LaunchAgent), entfernt ihn und zeigt ihn; `service unit --system` gibt
   die System-Unit für die globale Installation aus. `status` zeigt, ob der Dienst
   eingerichtet ist und läuft.
3. `upgrade --check [--json]` meldet Version, neueste Version, ob sich dieses Binary selbst
   ersetzen kann, und den Weg; `upgrade` scheitert bei fehlendem Schreibrecht mit diesem Weg
   statt eines Schreibfehlers und startet nach Erfolg einen laufenden Benutzerdienst neu.
4. `serve` fragt höchstens einmal am Tag bei GitHub nach und gibt das Ergebnis in `whoami`
   mit.
5. CI prüft auf macOS.
6. `docs/installation.md` (neu), README, Begriffe, Konzept, Projektregeln, Fortschritt.
7. Durchlauf: pro User hier, global im Container, Ansible-Beispiel gegen den Container.

## Kontext

- **Voraussetzung:** Task 008 (Abgleich im Hintergrund, `whoami`, `config set`, `status` je
  Hub) und Task 010 (`dev`/`main`; `ci.yml`, README „Bauen“) abgeschlossen. **Nicht
  gleichzeitig mit Task 009** — dieselben Dateien (`mcpnode`, README, Konzept „Stand“).
- **Entscheidungen** stehen in `docs/konzept.md`, „Installation und Betrieb“; hier nur, was
  die Umsetzung braucht:
  - Je Rechner genau eine Art. Global: Binary `/usr/local/bin/kephalaion` (root, `0755`),
    Systembenutzer `kephalaion`, `/etc/kephalaion/config.yaml` (für alle lesbar, kein
    Geheimnis), `/var/lib/kephalaion/` (`0700`, gehört `kephalaion`), System-Unit. Die User
    sind nur Clients. Pro User: `~/.local/bin`, `~/.config/kephalaion/`,
    `~/.local/share/kephalaion/`.
  - Global nur Linux; macOS nur pro User. Dienst nur über systemd bzw. launchd; ohne systemd
    nennt die Doku nur den Aufruf `kephalaion serve` für einen eigenen Supervisor.
  - Die Units erzeugt das Binary, damit sie zur Version passen; Ansible legt die System-Unit
    aus dessen Ausgabe ab.
  - Kein Token durch Ansible. Hub-Einträge des globalen Nodes (`node hub add`, erstes
    `rotate`) richtet der Verwalter von Hand ein, als Systembenutzer.
  - Global kann ein User nicht upgraden. Weg für den Verwalter: Ansible (Version anheben)
    oder `sudo kephalaion upgrade && sudo systemctl restart kephalaion`.
  - Über MCP meldet Kephalaion nur; niemand ersetzt das Binary über MCP.
  - GitHub-API ohne Anmeldung: 60 Anfragen je Stunde und Adresse, geteilt von allen Usern
    eines Rechners — deshalb in `serve` höchstens einmal am Tag.
- **Kommando für den Dienst — festgelegt am 2026-09-26** (mit dem Nutzer, Sitzung
  kephalaion-70): `kephalaion service install|uninstall|status` pro User, dazu
  `service unit --system` als Ausgabe der System-Unit. Im Konzept steht es noch als
  „vorläufig“; die Task trägt es in `docs/begriffe.md` ein, **bevor** es im Code steht.
- **Nicht in `init`.** `init` gilt je Rolle, der Dienst gehört zu `serve` (beide Rollen in
  einem Prozess), und `init` überschreibt nie. `init` nennt am Ende nur den nächsten Schritt
  („Dienst einrichten: kephalaion service install“).
- **Logs:** unter Linux ins Journal (`journalctl --user -u kephalaion`, global
  `journalctl -u kephalaion`), keine eigene Logdatei. launchd hat kein Journal; auf macOS
  eine Datei unter `~/.local/state/kephalaion/`.
- **Linger:** `service install` schaltet es nicht selbst ein. Ist eine Hub-Rolle eingerichtet,
  nennt es `loginctl enable-linger` — ohne Linger laufen Dienste eines Users nur, solange er
  angemeldet ist; für den Node reicht das, für einen Hub, den andere Rechner erreichen
  sollen, nicht. Unter WSL zusätzlich der Hinweis: Sie fährt ohne offenes Terminal bzw. VS
  Code herunter, laufende Dienste halten sie nicht wach.
- **Offen in der Umsetzung, hier festzulegen und in Konzept und Doku nachzutragen:**
  - Wem `/etc/kephalaion/` gehört, damit `sudo -u kephalaion kephalaion node init --config
    /etc/kephalaion/config.yaml --db sqlite:///var/lib/kephalaion/node.db` die config
    schreiben kann (etwa Verzeichnis `kephalaion`, `0755`; config `0644`).
  - Label des LaunchAgent (es gibt noch keine Domain; etwa `io.github.kephalaion`).
  - Namen der JSON-Felder (englisch, wie alle Bezeichner).
- **Nicht Teil dieser Task:** Lauschen auf der Docker-Bridge für Devcontainer (eigene Task;
  Konzeptdetails offen — Docker Desktop gegenüber nativem Docker, Host-Prüfung), deb-Paket,
  Homebrew, globale Installation auf macOS, Abschalten der täglichen Frage (im Konzept
  offen), die Oberfläche von k-playbook (anderes Repo), ein Release.
- **Parallele Sitzungen:** Im selben Arbeitsbaum arbeiten oft andere Sitzungen. Nur eigene
  Dateien bzw. Hunks committen, nicht `sichern` mit `git add -A` über fremde Änderungen.

## Zu bauen

### Etappe 1 — config finden, nie zwei Arten

- `config.Path` nach der Reihenfolge aus Ziel 1; dazu die Quelle (Flag, Umgebung, User,
  global) für `status`. Tests für jede Stufe, auch „User-config fehlt, globale fehlt“ und
  „beide vorhanden“ (die des Users gewinnt).
- `init` ohne `--config` und ohne `KEPHALAION_CONFIG`: Gibt es die globale config, Abbruch mit
  Hinweis (global eingerichtet, Verwaltung als Systembenutzer). Mit ausdrücklichem `--config`
  bleibt `init` wie bisher.
- `status` zeigt config-Ort und Quelle. Ist die config global und die Datenbank nicht lesbar
  (anderer User), meldet `status` die globale Installation und den Weg
  (`sudo -u kephalaion kephalaion …`) statt eines Fehlers beim Öffnen; Exit-Code festlegen
  und dokumentieren.
- Die Unit setzt `KEPHALAION_CONFIG` ohnehin (Etappe 2); die Suche ist für Menschen und
  k-playbook.

### Etappe 2 — Dienst

- `service` in `docs/begriffe.md` eintragen (siehe Kontext), dann das Kommando.
- **`service install`, Linux:** Unit nach `~/.config/systemd/user/kephalaion.service` (bzw.
  `$XDG_CONFIG_HOME`); `ExecStart` = absoluter Pfad des eigenen Binarys + `serve`, dazu
  `--config`, wenn die config nicht am Standardort liegt; `Restart=on-failure`,
  `WantedBy=default.target`; `systemctl --user daemon-reload` und `enable --now`.
  - Bricht ab, wenn schon ein `serve` von Hand läuft (Sperre `<db>.lock`, wie `status` sie
    prüft) — sonst startete der Dienst immer wieder neu und scheiterte.
  - Linger- und WSL-Hinweis wie im Kontext.
- **`service uninstall`:** `systemctl --user disable --now`, Unit entfernen,
  `daemon-reload`.
- **`service status`:** eingerichtet ja/nein, Pfad der Unit, läuft ja/nein; `kephalaion
  status` zeigt dieselbe Angabe in einer Zeile.
- **Ohne systemd** (`/run/systemd/system` fehlt: Alpine, Container, WSL ohne
  `systemd=true`): klare Meldung mit dem Aufruf `kephalaion serve` für einen eigenen
  Supervisor, Exit ≠ 0.
- **Pro User, macOS:** LaunchAgent unter `~/Library/LaunchAgents/<label>.plist`
  (`RunAtLoad`, `KeepAlive`, Log nach `~/.local/state/kephalaion/serve.log`), `launchctl
  bootstrap gui/<uid>`; `uninstall` mit `bootout`; `status` über `launchctl print`.
- **`init`** nennt am Ende den nächsten Schritt („Dienst einrichten: kephalaion service
  install“), mehr nicht.
- **Global:** `unit --system` gibt die System-Unit aus: `User=`/`Group=kephalaion`,
  `Environment=KEPHALAION_CONFIG=/etc/kephalaion/config.yaml`,
  `ExecStart=/usr/local/bin/kephalaion serve`, `StateDirectory=kephalaion`,
  `Restart=on-failure`, Härtung (`NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`,
  `PrivateTmp`), `WantedBy=multi-user.target`. Schreibt nichts nach `/etc`.
- Tests: erzeugte Units und plist als Text (golden), Pfade über überschreibbare Wurzeln wie
  beim `Upgrader`; `systemctl`/`launchctl` hinter einer Schnittstelle, in Tests ersetzt.
- `install.sh` nennt am Ende den nächsten Schritt (Dienst einrichten).

### Etappe 3 — Upgrade: prüfen, melden, neu starten

- `upgrade --check` meldet zusätzlich, ob sich dieses Binary selbst ersetzen kann
  (Schreibrecht im Verzeichnis, geprüft wie beim Ersetzen, etwa über eine Probedatei nach
  `TempPattern`) und den Weg: selbst → `kephalaion upgrade`; nicht selbst →
  `sudo kephalaion upgrade && sudo systemctl restart kephalaion` bzw. „über die Verwaltung,
  etwa Ansible“; dev build → `kephalaion upgrade --version vX.Y.Z`.
- `--json` gibt dieselben Angaben aus (für k-playbook); Exit-Codes von `--check` bleiben, wie
  sie sind, und werden dokumentiert.
- `upgrade` ohne Schreibrecht: Abbruch vor dem Download mit dem Weg, das Binary bleibt
  (Meldung wie bisher mit „bleibt unverändert“).
- Nach erfolgreichem `upgrade`: Läuft der Benutzerdienst (systemd `--user` bzw. LaunchAgent),
  startet `upgrade` ihn neu und sagt das; läuft ein System-Dienst, nur der Hinweis auf
  `sudo systemctl restart kephalaion`.
- **`make dev-install`** tut dasselbe: Läuft der Benutzerdienst, startet es ihn nach dem
  Ersetzen neu — sonst läuft nach dem Bauen weiter das alte Binary.
- Tests über `httptest` wie bisher; Schreibrecht über ein schreibgeschütztes Testverzeichnis.

### Etappe 4 — Update-Hinweis über MCP

- `serve` (Rolle Node) fragt höchstens einmal am Tag nach dem neuesten Release — neben dem
  Abgleich im Hintergrund aus Task 008, eigene Goroutine, beendet mit `serve`; bei einem
  Fehler frühestens nach einer Stunde erneut. Ergebnis nur im Speicher.
- `whoami` bekommt ein Feld mit: neueste Version, ob es ein Update gibt, ob sich das Binary
  selbst ersetzen kann, Weg, Zeitpunkt der Prüfung; vor der ersten Prüfung bzw. nach Fehler
  erkennbar leer mit Grund. Dieselbe Funktion wie Etappe 3, nur mit zwischengespeicherter
  Antwort; `node whoami --json` bleibt gleich der MCP-Antwort.
- Konzept, Werkzeuge → `whoami`-Tabelle: Feld ergänzen.
- Tests: Zeitsteuerung und GitHub über `httptest`, kein Netz.

### Etappe 5 — macOS in CI

- `ci.yml`: zweiter Job auf `macos-latest`, Actions wie im ersten Job auf SHA gepinnt,
  Toolchain aus `go.mod` mit `GOTOOLCHAIN=local`: `make check-toolchain`, `make check`,
  `make dist-host`; LaunchAgent des gebauten Binarys ausgeben und mit `plutil -lint` prüfen;
  `install.sh` mit `KEPHALAION_VERSION` (neuestes Release, ohne API-Anfrage) und `HOME` in
  einem Temp-Verzeichnis ausführen.
- Kein Aufruf der GitHub-API ohne Token in CI (geteilte Adressen, Rate-Limit).

### Etappe 6 — Doku

- **`docs/installation.md` (neu):**
  1. Überblick: zwei Arten, je Rechner eine, Tabelle der Orte.
  2. Pro User: `install.sh`, Rollen einrichten (Verweis aufs README), Dienst, Linger,
     Upgrade, Entfernen; macOS-Hinweise (`~/.local/bin` im `PATH`, nur über `curl`/
     `install.sh` laden, nicht über den Browser).
  3. Global (Linux): Voraussetzung systemd; Schritte von Hand in Reihenfolge — Binary mit
     Prüfsumme, Systembenutzer (`useradd --system`), Verzeichnisse und Rechte, `init` als
     Systembenutzer mit `--config`/`--db`, Unit aus `unit --system`, `enable --now`; Hub-
     Einträge und erstes `rotate` von Hand; Verwaltung mit `sudo -u kephalaion`.
  4. Upgrade: pro User, global (Ansible bzw. `sudo kephalaion upgrade` + Neustart); was
     `upgrade --check`/`whoami` melden.
  5. Ohne systemd: nur `kephalaion serve` für einen eigenen Supervisor.
  6. **Für Automatisierung (Ansible, KI):** feste Angaben — URL-Schema
     `https://github.com/kephalaion/kephalaion/releases/download/<tag>/kephalaion-<os>-<arch>`,
     `SHA256SUMS`, `releases/latest`, Architekturen (`x86_64` → `amd64`, `aarch64` →
     `arm64`), Pfade, Benutzer, Rechte, Reihenfolge, idempotente Schritte (`init` nur, wenn
     die Datenbank fehlt), Neustart nach Wechsel des Binarys, kein Token. Ein Beispiel mit
     Ansible-Tasks (`get_url` mit `checksum: sha256:<URL von SHA256SUMS>`, `user`, `file`,
     `command` mit `creates:`, Unit aus `unit --system` per `copy`, `systemd`, Handler für den
     Neustart). Der Hinweis auf die stabile URL
     `https://raw.githubusercontent.com/kephalaion/kephalaion/main/docs/installation.md`
     (Tag statt `main` für eine feste Version) — die nennt das Ansible-Repo.
- **README:** „Installation“ und „Upgrade“ mit beiden Arten kurz, Verweis auf
  `docs/installation.md`.
- **`docs/begriffe.md`:** Kommando für den Dienst, `upgrade`, system installation (dann
  gebaut), config (globaler Ort).
- **`docs/konzept.md`:** „Installation und Betrieb“ um die in dieser Task festgelegten Punkte
  ergänzen, „vorläufig“ und „nicht gebaut“ entfernen; Absatz „Stand“ oben.
- **`k-playbook-local/k-playbook.md`:** Aufbau (neue Pakete), Testen (macOS-Job).
- **`docs/fortschritt.md`** nach der Anleitung am Anfang der Datei.

### Etappe 7 — Durchlauf

- **Pro User, hier:** Ubuntu 22.04 in der WSL, `systemd=true`, `systemctl --user` läuft,
  Linger aus; Hub und Node sind pro User eingerichtet (Dev-Build, `serve` bisher von Hand).
  Erzeugte Unit mit `systemd-analyze --user verify` prüfen. Einrichten, Abbruch bei laufendem
  `serve`, Neustart nach `upgrade` bzw. `make dev-install`, `service status` und `uninstall`
  echt nur nach Rückfrage beim Nutzer — es trifft seinen echten Dienst und seine echte config.
- **Global im Wegwerf-Container** (Ubuntu oder Debian, ohne systemd als PID 1): das lokal
  gebaute Binary nach `/usr/local/bin`, die Schritte aus `docs/installation.md`, Abschnitt
  „Global“, von Hand; Hub und Node in der globalen config (`transport local`), ein Account;
  `serve` als `kephalaion` im Hintergrund. Als zweiter User im Container: `status` meldet die
  globale Installation; `whoami` über Loopback mit den Headern des Accounts antwortet;
  `upgrade --check` nennt den Weg für den Verwalter. `init` als zweiter User ohne `--config`
  bricht ab. Die System-Unit mit `systemd-analyze verify` prüfen (im Container mit
  installiertem systemd oder auf dem Rechner gegen passende Pfade).
- **Ansible:** das Beispiel aus der Doku mit `ansible-playbook --syntax-check`; ist die
  Collection `community.docker` vorhanden, echter Lauf gegen einen zweiten Container —
  `get_url` mit einem veröffentlichten Release, damit die Prüfsumme aus `SHA256SUMS` echt
  geprüft wird. Fehlt sie: nur nach Rückfrage installieren, sonst die Schritte per Shell im
  Container und im Bericht vermerken.
- **macOS:** Job in CI grün.
- Container danach entfernen.
