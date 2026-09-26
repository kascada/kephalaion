---
title: Kephalaion — VS Code
description: Erweiterung, die Collections über einen FileSystemProvider als Ordner in VS Code zeigt — lesen aus der Replica, schreiben über den Node am Hub; Ablauf, benötigte Werkzeuge, Status, Sprachen, Installation ohne Marketplace, gemeinsames Release, Ergebnis des Versuchs und Fundstellen.
---

# Kephalaion in VS Code

**Stand: Lesen gebaut** (2026-09-26, Version 0.0.4, siehe „Umsetzung“ unten): Statusleiste
aus `whoami`, Collections als Ordner mit Inhalt über `list` und `read`, Änderungen über
`changes`. Schreiben fehlt. Was die Erweiterung zum Lesen braucht, gibt es am Node: `whoami` in der Form,
die die Statusleiste braucht — Version, alle Hubs mit `login`, Node-Name und Stand des
Abgleichs (Task 008) —, dazu `list`, `read` und `changes` (Task 009, Festlegungen in
[`konzept.md`](konzept.md), „Allgemein — lesen“), und `serve` gleicht im Hintergrund ab. Begriffe nach [`begriffe.md`](begriffe.md), Hintergrund in
[`konzept.md`](konzept.md).

## Wozu

Eine Collection erscheint in VS Code als Ordner neben dem Projekt: im Explorer blättern,
Dokumente lesen, in der Markdown-Vorschau ansehen, bearbeiten und speichern. Vor allem lassen
sich **Daten leicht übergeben** — Dateien per Drag & Drop oder Kopieren und Einfügen aus dem
Projekt in eine Collection ziehen und umgekehrt.

Die Erweiterung ist eine **Oberfläche für Menschen**. KI-Agenten brauchen sie nicht; sie
lesen die echte Platte und sprechen mit dem Node über MCP.

Damit können auch Dokumente wie Tasks in den Store, ohne aus VS Code zu verschwinden (siehe
[`konzept.md`](konzept.md), „Für k-playbook — Kandidaten“).

## Wie es läuft

VS Code kennt dafür die Schnittstelle `FileSystemProvider`. Eine Erweiterung meldet ein
eigenes URI-Schema an, etwa `keph`, und beantwortet für dessen URIs die Vorgänge eines
Dateisystems. Explorer, Editor, Vorschau, Speichern und Kopieren arbeiten danach mit
`keph://…` wie mit `file://…`.

```ts
vscode.workspace.registerFileSystemProvider('keph', provider, {
  isCaseSensitive: true,
  isReadonly: false,
});
```

**URI:** `keph://<hub-alias>/<collection>/<name>`, etwa
`keph://team/entscheidungen/2026/009-transport.md`. Der Alias ist der Hub-Eintrag des Nodes.

```
keph://<hub-alias>/                  ← lesbare Collections (whoami bzw. list auf die Wurzel)
keph://<hub-alias>/<collection>/     ← list
keph://<hub-alias>/<collection>/…    ← list / read
```

Welche Collections es gibt, fragt die Erweiterung ab; sie stehen nirgends in VS Code.

**Einbinden** als Ordner eines Multi-Root-Workspace:

```json
// projekt.code-workspace
{ "folders": [
  { "path": "." },
  { "uri": "keph://team/wissen", "name": "Wissen" }
]}
```

### Die Vorgänge und ihre Entsprechung

| `FileSystemProvider` | Kephalaion (MCP-Werkzeug des Nodes) |
|---|---|
| `stat(uri)` | `read` mit `content: false`: Dokument, Verzeichnis oder nichts; `updated` als `mtime`, Größe als `size`, schreibbar ja/nein |
| `readDirectory(uri)` | `list` mit `path`, ohne Unterverzeichnisse, Verzeichnisse als eigene Einträge, mit Cursor bis zum Ende |
| `readFile(uri)` | `read` — aus der Replica, lokal und schnell, auch offline |
| `writeFile(uri, …)` | gibt es das Dokument noch nicht (`read` mit `content: false`), `create`; sonst `write` mit der Revision aus diesem `read`. Was die Replica schon kennt, fängt VS Code über `mtime` selbst ab („Datei ist neuer“); was noch nicht abgeglichen ist, lehnt der Hub an der Revision ab |
| `rename(alt, neu)` | `rename`, `id` bleibt; ein Verzeichnis als Ganzes. Nur innerhalb einer Collection, kein Überschreiben eines belegten Ziels |
| `delete(uri)` | `delete` (Löschmarke) — Eigenes mit `write`, Fremdes nur mit `supersede`; ein Verzeichnis als Ganzes (`recursive`) |
| `createDirectory(uri)` | nichts am Hub — Verzeichnisse sind nur Präfixe von Namen; die Erweiterung merkt sich das leere Verzeichnis, bis darin etwas angelegt wird |
| `watch` / Event `onDidChangeFile` | `changes` mit dem Cursor der letzten Antwort, abgefragt alle paar Sekunden — aus der Replica, ohne Netz. Ein Umbenennen erkennt die Erweiterung an der `id` (alter Name gelöscht, neuer angelegt); `changes` nennt keinen alten Namen. Bei `reset` liest sie den Hub neu mit `list`, bei `dropped` entfernt sie die Collection |

- **Konflikte:** VS Code vergleicht `mtime` beim Speichern selbst und fragt nach, wenn die
  Datei inzwischen neuer ist. Zusätzlich lehnt der Hub ab, wenn die mitgeschickte Revision
  veraltet ist (siehe [`konzept.md`](konzept.md), „Zwei Arten von Eingaben“). Die
  Erweiterung meldet das als Fehler; nichts wird still überschrieben.
- **Offline wird gelesen, nicht geschrieben.** Ist der Hub nicht erreichbar, wirft
  `writeFile` `FileSystemError.Unavailable` mit der Meldung des Nodes. Lesen läuft weiter.
- **Rechte:** Collections ohne Schreibrecht meldet `stat` als schreibgeschützt
  (`FilePermission.Readonly`); VS Code öffnet sie dann nur zum Lesen.
- **Verbindung und Stand:** `whoami` — Account, Hubs, lesbare Collections und
  Rechte, Stand des Abgleichs, Version.
- **Die Replica bleibt unberührt.** Die Erweiterung schreibt nie in sie; jeder
  Schreibvorgang geht über den Node zum Hub, die Replica zieht über den Abgleich nach — wie bei
  jedem anderen Client.

### Welche Werkzeuge — keine eigens für VS Code

Entschieden am 2026-09-26: Die Erweiterung braucht **`whoami`, `list`, `read` und
`changes`**, zum Schreiben später `create`, `write`, `rename`, `delete`. Alle taugen auch für
die KI; es gibt keine Werkzeuge nur für VS Code, kein eigenes Profil, keine Schnittstelle
neben MCP. Dafür wurden drei Werkzeuge im Konzept erweitert bzw. neu aufgenommen
([`konzept.md`](konzept.md), „Allgemein — lesen“):

- `list` liefert Verzeichnisse der nächsten Ebene als eigene Einträge;
- `read` mit `content: false` ersetzt ein eigenes `stat`;
- `changes` meldet, was sich seit einer Revision oder einem Zeitpunkt geändert hat.

Erwogen und verworfen: Werkzeuge mit dem Vermerk „nur für VS Code“ in der Beschreibung — die
KI lädt die Beschreibung trotzdem mit und ruft sie gelegentlich auf; ein Profil (eigener
Pfad oder Header, der Zusatzwerkzeuge freischaltet); MCP-„Resources“ — ohne Verzeichnisse,
und deren Benachrichtigungen brauchen eine Sitzung.

## Status und Auswahl

- **Statusleiste:** etwa `Keph ✓ team` bzw. `Keph ⚠ offline`. Tooltip: Account je Hub,
  letzter Abgleich, Revision, Version von Node und Erweiterung — mit Warnung, wenn sie
  verschieden sind. Quelle ist `whoami`, dasselbe Werkzeug, das die KI benutzt: je Hub
  `login` (`ok`, `invalid`, `missing`) und `sync` (`last_success`, `revision`, `last_error`,
  `never_synced`). Ist `login` `invalid` und `sync.never_synced` gesetzt, liegt es nicht am
  Token, sondern der Node hat den Hub noch nie abgeglichen. `unknown_hubs` nennt Header zu
  Aliasen, die der Node nicht kennt — ein Hinweis auf eine falsch eingerichtete
  Erweiterung.
- **Klick darauf:** ein kleines Menü — neu verbinden, Token setzen, Collection einbinden,
  Log anzeigen (Output-Channel „Kephalaion“ mit den Aufrufen und Fehlern).
- **Node nicht erreichbar:** Dann scheitert auch das Lesen, weil die Erweiterung die Replica
  nie selbst öffnet. Die Statusleiste zeigt das, mit einem Hinweis, wie der Node gestartet
  wird.
- **In VS Code wird gewählt, was angezeigt wird:** „Collection einbinden“ bietet die
  lesbaren Collections aus `whoami` zur Auswahl an und fügt die gewählte als Ordner in den
  Workspace ein.
- **Über die CLI wird eingerichtet:** Hubs, Tokens, gewünschte Collections, Accounts. Das
  ist Verwaltung mit Tokens und heute schon Kommandozeile; die Verwaltung über MCP ist im
  Konzept nur vorgemerkt. Nicht doppelt bauen.
- **Welcher Node:** Es gibt einen je Rechner. Die Erweiterung läuft neben ihm (siehe
  `extensionKind`) und kann die Adresse aus `listen` in `~/.config/kephalaion/config.yaml`
  lesen; eine Einstellung `kephalaion.nodeUrl` braucht es nur zum Überschreiben.
- **Account und Token** trägt die Erweiterung als Header ein, wie jeder Client
  (`X-Keph-Account-<hub>`, `X-Keph-Token-<hub>`). **Sie liest sie aus den Token-Dateien**
  `~/.config/kephalaion/tokens/<hub>/<account>.token` (siehe [`konzept.md`](konzept.md),
  „Orte nach XDG“) — sie läuft als `workspace` auf demselben Rechner und braucht so keine
  eigene Einrichtung. Die Zuordnung ist eindeutig: Das Verzeichnis ist der Alias des Hubs und
  damit der Name im Header, der Dateiname der Account; Dateien auf `.pending` übergeht sie.
  `whoami` bestätigt danach nur, dass die Anmeldung gilt (`login: ok`). **Mehrere Accounts an
  einem Hub:** Auswahl über „Kephalaion: Account wählen“ (auch im Menü der Statusleiste); die
  Wahl steht in der Einstellung `kephalaion.accounts` (`{"<hub>": "<account>"}`) — nur Namen,
  kein Token —, Scope `machine-overridable`: je Rechner, nicht über Settings Sync, ein
  Workspace kann sie überschreiben. Ohne Wahl nimmt die Erweiterung den ersten nach Namen und
  zeigt die Statusleiste gelb, ebenso bei einem gewählten Account ohne Token-Datei.
  Der `SecretStorage` von VS Code bleibt nur der Ausweg, wenn keine Datei da ist; nie
  `settings.json` — die landet leicht im Repository oder in Settings Sync.
  Geprüft am 2026-09-26: `whoami` mit dem Token aus `tokens/home/kamran-desktop.token`
  liefert `login: ok`, Account `kamran-desktop`, User `kamran`, Collection `home:eins` mit
  `read` und `write`. Die MCP-Konfigurationen der KI-Clients (`.mcp.json`,
  `.cursor/mcp.json`, `opencode.json`,
  `.vscode/mcp.json`) liest die Erweiterung nicht.

## Sprachen

**Eine VS-Code-Erweiterung ist JavaScript bzw. TypeScript.** Sie läuft im Extension Host, einem
Node.js-Prozess (oder als Web-Erweiterung in einem Browser-Worker). Andere Sprachen gehen nur
mittelbar:

- **Dünne Erweiterung in TypeScript, Logik in Go.** Die Erweiterung übersetzt nur zwischen
  VS Code und einem Prozess in einer anderen Sprache — über stdio (JSON-RPC, wie beim Language
  Server Protocol) oder HTTP. So arbeiten gopls, rust-analyzer und die meisten großen
  Erweiterungen.
- **WASM:** VS Code kann WASI-Module laden (`@vscode/wasm-wasi`), Go lässt sich nach WASM
  übersetzen. Für Kephalaion ungeeignet: SQLite, Dateizugriff und Netz sind aus WASM heraus
  mühsam, und die Logik gibt es ohnehin schon im Node.

**Für Kephalaion: ein dünner Übersetzer; die Logik bleibt im Binary.** Gebaut ist er in
reinem JavaScript ohne Abhängigkeiten (siehe „Umsetzung“).

**Weg zum Node: MCP über HTTP.** Der Node ist ohnehin ein MCP-Server über HTTP auf
`127.0.0.1` (siehe [`konzept.md`](konzept.md), „Kommunikation“), und `list`, `read`, `write`,
`rename`, `delete` sind genau die Vorgänge, die der Provider braucht. Die Erweiterung ist damit
ein MCP-Client wie jeder andere; ein eigenes Protokoll entfällt, und das SDK
(`@modelcontextprotocol/sdk`) braucht sie nicht — JSON-RPC über das `fetch` von Node.js genügt.
Voraussetzung ist ein laufender Node (`kephalaion serve`). Account und Token trägt die
Erweiterung als Header ein, wie jeder Client; sie liest sie aus den Token-Dateien (siehe
„Status und Auswahl“), nie aus den Einstellungen.

## WSL und Remote: `extensionKind`

VS Code teilt Erweiterungen in zwei Arten:

- **`ui`** — läuft auf dem Rechner, auf dem das Fenster ist (unter WSL: Windows).
- **`workspace`** — läuft dort, wo der Workspace liegt (unter WSL: im Linux der WSL, im
  VS-Code-Server).

**Die Erweiterung muss `workspace` sein**, in `package.json`:

```json
"extensionKind": ["workspace"]
```

Nur dort erreicht sie den Node auf `127.0.0.1` der WSL und das Binary unter
`~/.local/bin/`. Als `ui` liefe sie unter Windows und sähe weder das eine noch das andere.
Dasselbe gilt für SSH-Remotes und Devcontainer: Die Erweiterung läuft neben dem Node.
Folge: Sie muss auch **dort** installiert sein, nicht nur im Windows-VS-Code (siehe
Installation).

## Installation — ohne Marketplace

Der Marketplace ist nicht nötig. Eine Erweiterung ist eine Datei `.vsix` und lässt sich lokal
installieren:

```sh
code --install-extension kephalaion-0.3.0.vsix
```

oder in VS Code über „Extensions: Install from VSIX…“. Unter WSL installiert `code` aus einem
Terminal der WSL in den VS-Code-Server der WSL, also dorthin, wo eine Erweiterung der Art
`workspace` hingehört — bestätigt am 2026-09-26: `code` meldet dabei „Installing extensions on
WSL: Ubuntu…“. Cursor und VSCodium nehmen dieselbe Datei.

Was ohne Marketplace fehlt: automatische Updates. Die übernimmt das gemeinsame Release (unten).

## Ein Repository, ein Release

**Entschieden am 2026-09-26: Die Erweiterung liegt in diesem Repository und wird mit dem
Binary zusammen veröffentlicht, unter derselben Version.** Ein Release erneuert beide, auch
wenn sich nur eines geändert hat. Bei wenigen Nutzern ist das unkritisch; bei vielen Nutzern
lässt es sich später trennen.

- **Gleiche Version heißt: kein Abgleich von Versionen zwischen Erweiterung und Node.** Die
  Erweiterung fragt beim Start die Version des Nodes ab (`whoami`) und weist auf einen
  Unterschied hin, statt Kompatibilitäten zu verwalten.
- **Verzeichnis:** etwa `vscode/` im Repository, mit eigenem `package.json`. Der Build
  braucht Node.js und `@vscode/vsce`; das kommt zur CI hinzu.
- **Auslieferung, zwei Möglichkeiten:**
  - als eigenes Asset `kephalaion-<version>.vsix` am Release, neben den Binaries und in
    `SHA256SUMS`;
  - oder **ins Binary eingebettet** (`go:embed`, eine `.vsix` ist klein) mit einem Befehl
    wie `kephalaion vscode install`, der sie auspackt und `code --install-extension`
    aufruft. Dann bringt `kephalaion upgrade` die passende Erweiterung gleich mit, und es gibt
    nur eine Datei zu verteilen. Der Go-Build hängt dann vom Bau der Erweiterung ab
    (Makefile).
  - Neigung: eingebettet — ein Binary, eine Version, ein Upgrade.
- **Vorabversionen:** `vsce` nimmt nach bisheriger Kenntnis keine Version mit Suffix
  (`0.3.0-rc1`) an, sondern nur `major.minor.patch` und kennzeichnet Vorabversionen über
  einen Schalter. Beim Bau zu prüfen, wie ein Tag `v0.3.0-rc1` auf die Version der
  Erweiterung abgebildet wird.

## Suche: bewusst nicht

Die Suche von VS Code (Strg+Umschalt+F) und Quick Open (Strg+P) finden in `keph://`-Ordnern
nichts. Die Schnittstellen dafür (`TextSearchProvider`, `FileSearchProvider`) sind
„proposed API“ und in einer normalen Installation nicht freigeschaltet.

**Das ist gewollt** (2026-09-26): In VS Code wird Code gesucht, nicht Dokumentation;
Treffer aus der Doku stören dort. Gesucht wird im Store über MCP — von der KI, mit der Suche
des Nodes (FTS5, später mehr), die schneller und besser rankt, als es die Textsuche von
VS Code könnte. Die Erweiterung bekommt keinen eigenen Suchbefehl.

**Ausnahme, beobachtet am 2026-09-26:** Dokumente, die VS Code gerade geladen hat, findet die
Suche doch — sie durchsucht neben der Platte immer auch die offenen Dokumente, damit
Ungespeichertes gefunden wird, unabhängig vom Dateisystem. Nach dem Schließen verschwindet der
Treffer (mit etwas Verzögerung, erst nach einer weiteren Suche). Das ist in Ordnung.

## Offen

- Wie der Node-Dienst gestartet wird, wenn er nicht läuft — Hinweis in VS Code oder selbst
  starten (wie k-playbook beim Briefing).
- Abstand der Abfrage von `changes` — fest, oder länger, solange das Fenster nicht im Fokus
  ist.
- Leere Verzeichnisse: vorerst nur in der Erweiterung gemerkt (Task 014); ob später als
  `SYSTEM:D:`-Zeile am Hub, bleibt offen (vgl. „Persönliche Verzeichnisse“ im Konzept).
- Wie die Erweiterung mit `personal`-Verzeichnissen umgeht (nur Eigenes zeigen, Schalter für
  alles).
- Ob eine TreeView zusätzlich zum Dateisystem sinnvoll ist, etwa für Status und Abgleich.

## Versuch: Dummy (2026-09-26)

Unter [`vscode/`](../vscode/): reines JavaScript ohne Abhängigkeiten, ein
`FileSystemProvider` für `keph://` mit einem festen Verzeichnis und einer Datei, nur lesen,
ohne Verbindung zum Node. Zwei Befehle: „Dummy-Datei öffnen“ und „Dummy-Ordner zum Workspace
hinzufügen“. Gebaut mit `npx @vscode/vsce package --skip-license`, installiert mit
`code --install-extension` aus der WSL.

Ergebnis:

- Die Datei öffnet sich schreibgeschützt, die Markdown-Vorschau funktioniert.
- Der Ordner erscheint im Explorer als Ordner des Workspace; das Fenster wird dabei zu
  einem Workspace mit mehreren Ordnern und lädt neu.
- Die Suche verhält sich wie oben beschrieben.
- Der Weg trägt; offen ist nur noch die Anbindung an den Node.

## Umsetzung: Lesen (2026-09-26)

Ohne Task gebaut, in zwei Schritten, weiter reines JavaScript ohne Abhängigkeiten
([`vscode/extension.js`](../vscode/extension.js)). MCP über das `fetch` von Node.js, JSON-RPC
ohne Sitzung — das SDK braucht es dafür nicht.

- **0.0.2 — `whoami`:** Adresse aus `listen`, Tokens aus `tokens/<hub>/<account>.token`
  (höchstens alle 5 s neu von der Platte), Statusleiste mit Tooltip, abgefragt alle 30 s;
  Menü; „Collection einbinden“. Im echten VS Code geprüft.
- **0.0.3 — Inhalte:** `stat` über `read` mit `content: false`, `readDirectory` über `list`
  (Namen kommen als voller Pfad ab der Collection, die Erweiterung nimmt das letzte Segment;
  blättert mit `cursor`), `readFile` über `read` — der Inhalt ist der Text des Ergebnisses.
  `changes` alle 3 s mit dem Cursor der letzten Antwort; je Eintrag `Changed` bzw. `Deleted`
  für das Dokument und `Changed` für jedes Verzeichnis darüber bis zur Collection, damit der
  Explorer neue und leer gewordene Verzeichnisse sieht. Ein Umbenennen kommt so als
  gelöscht und neu an; die `id` wertet die Erweiterung dafür nicht aus. `reset` und
  `dropped` lassen die Wurzel des Hubs neu lesen. Dazu „Kephalaion: Status anzeigen“ — der
  Inhalt des Tooltips im Output „Kephalaion“, weil der Tooltip schwer zu finden ist (der
  Tooltip eines Ordners im Explorer zeigt nur den Pfad, etwa `\eins`).
- **Fehler:** Eine Antwort des Werkzeugs mit Fehler („nicht lesbar“) wird zu
  `FileNotFound`, ein Fehler der Verbindung zu `Unavailable`.
- **Alles schreibgeschützt**, auch mit `writable: true`, bis Schreiben gebaut ist.
- **Geprüft** mit einem Ersatz für das Modul `vscode` gegen den laufenden Node (`home:eins`,
  Dokumente unter `test/`): Verzeichnisse, Metadaten, Inhalt, „nicht gefunden“, fremde
  Collection, Ereignisse nach `hub doc put` und `node sync`. Im echten VS Code: ein Dokument
  aus `keph://home/eins` geöffnet.
- **0.0.4 — Account wählen:** alle `tokens/<hub>/*.token` je Hub, Wahl über „Kephalaion:
  Account wählen“ in `kephalaion.accounts` (siehe „Status und Auswahl“). Eine andere Wahl —
  auch von Hand in `settings.json` — liest die Tokens sofort neu, setzt `changes` neu an und
  lässt die Hubs neu lesen, weil Rechte und Collections dann andere sein können. Tooltip und
  „Status anzeigen“ nennen die Accounts eines Hubs und welcher gewählt ist. Geprüft mit dem
  Ersatz für `vscode` und Kopien der Token-Dateien samt einem erfundenen zweiten Account:
  ohne Wahl, gewählt, gewählt ohne Datei, falscher Account gewählt; `.pending` übergangen.

## Fundstellen

- API-Referenz `FileSystemProvider`:
  https://code.visualstudio.com/api/references/vscode-api#FileSystemProvider
- Beispiel eines Dateisystems im Speicher („MemFS“), die beste Vorlage:
  https://github.com/microsoft/vscode-extension-samples/tree/main/fsprovider-sample
- Virtuelle Workspaces — was in ihnen geht und was nicht:
  https://code.visualstudio.com/api/extension-guides/virtual-workspaces
- Erweiterungen unter Remote, WSL, Devcontainer (`extensionKind`):
  https://code.visualstudio.com/api/advanced-topics/remote-extensions
- Einstieg in eine erste Erweiterung:
  https://code.visualstudio.com/api/get-started/your-first-extension
- Verpacken und lokal installieren (`vsce package`, `.vsix`):
  https://code.visualstudio.com/api/working-with-extensions/publishing-extension
- WASI in VS Code (nur zur Einordnung):
  https://code.visualstudio.com/blogs/2023/06/05/vscode-wasm-wasi
- MCP-Client in TypeScript: https://github.com/modelcontextprotocol/typescript-sdk
- Vorbild aus der Praxis: „GitHub Repositories“ von Microsoft, bindet Repos als
  `vscode-vfs://` ein, ohne zu klonen.
