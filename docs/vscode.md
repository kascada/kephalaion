---
title: Kephalaion — VS Code
description: Idee für eine Erweiterung, die Collections über einen FileSystemProvider als Ordner in VS Code zeigt — lesen aus der Replica, schreiben über den Node am Hub; Ablauf, Sprachen, Installation ohne Marketplace, gemeinsames Release und Fundstellen.
---

# Kephalaion in VS Code

**Stand: Idee, nicht gebaut** (2026-09-26). Festgehalten, damit der Weg klar ist, wenn
`kephalaion serve` steht. Begriffe nach [`begriffe.md`](begriffe.md), Hintergrund in
[`konzept.md`](konzept.md).

## Wozu

Eine Collection erscheint in VS Code als Ordner neben dem Projekt: im Explorer blättern,
Dokumente lesen, in der Markdown-Vorschau ansehen, bearbeiten und speichern. Vor allem lassen
sich **Daten leicht übergeben** — Dateien per Drag & Drop oder Kopieren und Einfügen aus dem
Projekt in eine Collection ziehen und umgekehrt.

Die Erweiterung ist eine **Oberfläche für Menschen**. KI-Agenten brauchen sie nicht; sie
lesen die echte Platte und sprechen mit dem Node über MCP.

Unabhängig davon bleiben Tasks vorerst Dateien im Projekt (siehe
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
| `stat(uri)` | Dokument oder Verzeichnis; Revision bzw. `updated` als `mtime`, Länge als `size` |
| `readDirectory(uri)` | `list` mit `path`, ohne Unterverzeichnisse, mit Cursor bis zum Ende |
| `readFile(uri)` | `read` — aus der Replica, lokal und schnell, auch offline |
| `writeFile(uri, …)` | `create` bzw. `write` mit der Revision, auf der die Änderung beruht |
| `rename(alt, neu)` | `rename`, `id` bleibt |
| `delete(uri)` | `delete` (Löschmarke) bzw. `supersede`, je nach Recht |
| `createDirectory(uri)` | nichts am Hub — Verzeichnisse sind nur Präfixe von Namen; die Erweiterung merkt sich das leere Verzeichnis, bis darin etwas angelegt wird |
| `watch` / Event `onDidChangeFile` | ausgelöst, wenn sich nach einem Abgleich Revisionen ändern |

- **Konflikte:** VS Code vergleicht `mtime` beim Speichern selbst und fragt nach, wenn die
  Datei inzwischen neuer ist. Zusätzlich lehnt der Hub ab, wenn die mitgeschickte Revision
  veraltet ist (siehe [`konzept.md`](konzept.md), „Zwei Arten von Eingaben“). Die
  Erweiterung meldet das als Fehler; nichts wird still überschrieben.
- **Offline wird gelesen, nicht geschrieben.** Ist der Hub nicht erreichbar, wirft
  `writeFile` `FileSystemError.Unavailable` mit der Meldung des Nodes. Lesen läuft weiter.
- **Rechte:** Collections ohne Schreibrecht meldet `stat` als schreibgeschützt
  (`FilePermission.Readonly`); VS Code öffnet sie dann nur zum Lesen.
- **Die Replica bleibt unberührt.** Die Erweiterung schreibt nie in sie; jeder
  Schreibvorgang geht über den Node zum Hub, die Replica zieht über den Abgleich nach — wie bei
  jedem anderen Client.

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
`workspace` hingehört (zu prüfen beim Bau). Cursor und VSCodium nehmen dieselbe Datei.

Was ohne Marketplace fehlt: automatische Updates. Die übernimmt das gemeinsame Release (unten).

## Ein Repository, ein Release

**Entschieden am 2026-09-26: Die Erweiterung liegt in diesem Repository und wird mit dem
Binary zusammen veröffentlicht, unter derselben Version.** Ein Release erneuert beide, auch
wenn sich nur eines geändert hat. Bei wenigen Nutzern ist das unkritisch; bei vielen Nutzern
lässt es sich später trennen.

- **Gleiche Version heißt: kein Abgleich von Versionen zwischen Erweiterung und Node.** Die
  Erweiterung fragt beim Start die Version des Nodes ab (`status`) und weist auf einen
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

## Offen

- Wie der Node-Dienst gestartet wird, wenn er nicht läuft — Hinweis in VS Code oder selbst
  starten (wie k-playbook beim Briefing).
- Wie oft `onDidChangeFile` feuert: ein Ereignis je Abgleich mit den geänderten Namen, und ob
  der Node dafür eine Benachrichtigung braucht oder die Erweiterung die Revision abfragt.
- Leere Verzeichnisse: nur in der Erweiterung gemerkt, oder als `SYSTEM:D:`-Zeile am Hub
  (vgl. „Persönliche Verzeichnisse“ im Konzept).
- Wie die Erweiterung mit `personal`-Verzeichnissen umgeht (nur Eigenes zeigen, Schalter für
  alles).
- Ob eine TreeView zusätzlich zum Dateisystem sinnvoll ist, etwa für Status und Abgleich.

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
