---
title: Kephalaion — VS Code
description: Erweiterung, die Collections über einen FileSystemProvider als Ordner in VS Code zeigt — lesen aus der Replica, schreiben über den Node am Hub; Ablauf, benötigte Werkzeuge, Status, Sprachen, Installation ohne Marketplace, gemeinsames Release, Ergebnis des Versuchs und Fundstellen.
---

# Kephalaion in VS Code

**Stand: Versuch mit Dummy gelungen** (2026-09-26, siehe „Versuch“ unten); die Anbindung an
den Node ist nicht gebaut. Sie setzt die Werkzeuge `list`, `read` und `changes` des Nodes
voraus; gebaut ist bisher `whoami` (Task 005). Begriffe nach [`begriffe.md`](begriffe.md), Hintergrund in
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
| `writeFile(uri, …)` | `create` bzw. `write` mit der Revision, auf der die Änderung beruht |
| `rename(alt, neu)` | `rename`, `id` bleibt |
| `delete(uri)` | `delete` (Löschmarke) bzw. `supersede`, je nach Recht |
| `createDirectory(uri)` | nichts am Hub — Verzeichnisse sind nur Präfixe von Namen; die Erweiterung merkt sich das leere Verzeichnis, bis darin etwas angelegt wird |
| `watch` / Event `onDidChangeFile` | `changes` mit dem Cursor der letzten Antwort, abgefragt alle paar Sekunden — aus der Replica, ohne Netz. Ein Umbenennen erkennt die Erweiterung an der `id` (alter Name gelöscht, neuer angelegt); `changes` nennt keinen alten Namen |

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
  verschieden sind. Quelle ist `whoami`, dasselbe Werkzeug, das die KI benutzt.
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
- **Account und Token** trägt die Erweiterung als Header ein, wie jeder Client. Sie liegen
  im `SecretStorage` von VS Code (Schlüsselbund des Systems), gesetzt über einen Befehl, nie
  in `settings.json` — die landet leicht im Repository oder in Settings Sync. Die
  MCP-Konfigurationen der KI-Clients (`.mcp.json`, `.cursor/mcp.json`, `opencode.json`,
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

**Für Kephalaion: TypeScript als Übersetzer, geschätzt 200–400 Zeilen; die Logik bleibt im
Binary.**

**Weg zum Node: MCP über HTTP.** Der Node ist ohnehin ein MCP-Server über HTTP auf
`127.0.0.1` (siehe [`konzept.md`](konzept.md), „Kommunikation“), und `list`, `read`, `write`,
`rename`, `delete` sind genau die Vorgänge, die der Provider braucht. Die Erweiterung ist damit
ein MCP-Client wie jeder andere (`@modelcontextprotocol/sdk`); ein eigenes Protokoll entfällt.
Voraussetzung ist ein laufender Node (`kephalaion serve`). Account und Token trägt die
Erweiterung als Header ein, wie jeder Client; das Token liegt im `SecretStorage` von VS Code,
nicht in den Einstellungen.

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
- Leere Verzeichnisse: nur in der Erweiterung gemerkt, oder als `SYSTEM:D:`-Zeile am Hub
  (vgl. „Persönliche Verzeichnisse“ im Konzept).
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
