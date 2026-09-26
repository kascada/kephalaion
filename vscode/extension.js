// Kephalaion in VS Code: Status aus whoami und ein FileSystemProvider für keph://.
// Stufe 1: nur whoami — Statusleiste, Collections als Wurzel von keph://<hub>/.
// Inhalte der Collections kommen mit list/read (Task 009). Siehe docs/vscode.md.

const vscode = require('vscode');
const fs = require('fs');
const os = require('os');
const path = require('path');

const SCHEME = 'keph';
const POLL_MS = 30000;

// --- Orte (docs/konzept.md, „Orte nach XDG“) ---

function configDir() {
  const xdg = process.env.XDG_CONFIG_HOME || path.join(os.homedir(), '.config');
  return path.join(xdg, 'kephalaion');
}

function configFile() {
  return process.env.KEPHALAION_CONFIG || path.join(configDir(), 'config.yaml');
}

// listen aus dem Abschnitt node: der config — ohne YAML-Bibliothek, die Datei ist flach.
function nodeListen(file) {
  let text;
  try {
    text = fs.readFileSync(file, 'utf8');
  } catch {
    return undefined;
  }
  let section = '';
  for (const line of text.split('\n')) {
    const top = line.match(/^([A-Za-z_]+):\s*$/);
    if (top) {
      section = top[1];
      continue;
    }
    const m = line.match(/^\s+listen:\s*["']?([^"'\s#]+)/);
    if (m && section === 'node') return m[1];
  }
  return undefined;
}

function nodeUrl() {
  const set = vscode.workspace.getConfiguration('kephalaion').get('nodeUrl');
  if (set) return set.replace(/\/+$/, '') + '/mcp';
  let listen = nodeListen(configFile());
  if (!listen) return undefined;
  if (listen.startsWith(':')) listen = '127.0.0.1' + listen;
  return `http://${listen}/mcp`;
}

// Token-Dateien: tokens/<hub>/<account>.token; *.pending wird übergangen.
// Mehrere Accounts an einem Hub: vorerst der erste nach Namen (Auswahl kommt später).
function readCredentials(log) {
  const base = path.join(configDir(), 'tokens');
  const creds = {};
  let hubs = [];
  try {
    hubs = fs.readdirSync(base, { withFileTypes: true }).filter((d) => d.isDirectory());
  } catch {
    return creds;
  }
  for (const hub of hubs) {
    const files = fs.readdirSync(path.join(base, hub.name))
      .filter((f) => f.endsWith('.token'))
      .sort();
    if (files.length === 0) continue;
    if (files.length > 1) log(`Hub ${hub.name}: mehrere Accounts (${files.join(', ')}), nehme ${files[0]}`);
    const token = fs.readFileSync(path.join(base, hub.name, files[0]), 'utf8').split('\n')[0].trim();
    if (token) creds[hub.name] = { account: files[0].slice(0, -'.token'.length), token };
  }
  return creds;
}

// --- MCP über HTTP, zustandslos (docs/konzept.md, „Kommunikation“) ---

class Node {
  constructor(log) {
    this.log = log;
    this.id = 0;
  }

  async call(tool, args) {
    const url = nodeUrl();
    if (!url) throw new Error(`keine Adresse des Nodes: listen fehlt in ${configFile()}`);
    const headers = {
      'Content-Type': 'application/json',
      Accept: 'application/json, text/event-stream',
    };
    for (const [hub, c] of Object.entries(readCredentials(this.log))) {
      headers[`X-Keph-Account-${hub}`] = c.account;
      headers[`X-Keph-Token-${hub}`] = c.token;
    }
    const body = { jsonrpc: '2.0', id: ++this.id, method: 'tools/call', params: { name: tool, arguments: args || {} } };
    const res = await fetch(url, { method: 'POST', headers, body: JSON.stringify(body) });
    if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
    const msg = parseResponse(await res.text(), res.headers.get('content-type') || '');
    if (msg.error) throw new Error(`${tool}: ${msg.error.message}`);
    if (msg.result && msg.result.isError) {
      const t = (msg.result.content || []).map((c) => c.text).join('\n');
      throw new Error(`${tool}: ${t}`);
    }
    return msg.result.structuredContent;
  }
}

// Antwort als JSON oder als SSE mit einem data:-Block.
function parseResponse(text, type) {
  if (type.includes('text/event-stream')) {
    const data = text.split('\n').filter((l) => l.startsWith('data:')).map((l) => l.slice(5).trim());
    return JSON.parse(data[data.length - 1]);
  }
  return JSON.parse(text);
}

// --- Status ---

class Status {
  constructor(node, log) {
    this.node = node;
    this.log = log;
    this.who = undefined;
    this.error = undefined;
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 50);
    this.item.command = 'kephalaion.menu';
    this.item.show();
    this._changed = new vscode.EventEmitter();
    this.onChanged = this._changed.event;
  }

  async refresh() {
    try {
      this.who = await this.node.call('whoami');
      this.error = undefined;
    } catch (e) {
      this.who = undefined;
      this.error = e.cause ? `${e.message} (${e.cause.code || e.cause.message})` : e.message;
      this.log(`whoami: ${this.error}`);
    }
    this.render();
    this._changed.fire();
  }

  hubs() {
    return (this.who && this.who.hubs) || [];
  }

  // Lesbare Collections eines Hubs; leer, wenn die Anmeldung nicht gilt.
  collections(hub) {
    const h = this.hubs().find((x) => x.hub === hub);
    if (!h || h.login !== 'ok') return [];
    return (h.collections || []).filter((c) => (c.rights || []).includes('read'));
  }

  render() {
    const it = this.item;
    if (!this.who) {
      it.text = '$(warning) Keph: Node nicht erreichbar';
      it.tooltip = this.error;
      it.backgroundColor = new vscode.ThemeColor('statusBarItem.warningBackground');
      return;
    }
    const hubs = this.hubs();
    const bad = hubs.filter((h) => h.login !== 'ok' || h.sync.last_error);
    it.text = `${bad.length ? '$(warning)' : '$(database)'} Keph ${hubs.map((h) => h.hub).join(' ')}`;
    it.backgroundColor = bad.length ? new vscode.ThemeColor('statusBarItem.warningBackground') : undefined;

    const md = new vscode.MarkdownString();
    md.appendMarkdown(`**Kephalaion** — Node ${this.who.version}, Erweiterung ${ext.packageJSON.version}\n\n`);
    for (const h of hubs) {
      md.appendMarkdown(`**${h.hub}** (Node ${h.node}): Anmeldung \`${h.login}\``);
      if (h.login === 'ok') md.appendMarkdown(`, Account ${h.account}, User ${h.user}`);
      md.appendMarkdown('\n\n');
      for (const c of h.collections || []) md.appendMarkdown(`- \`${c.address}\` — ${c.rights.join(', ')}\n`);
      const s = h.sync;
      if (s.never_synced) md.appendMarkdown('\nnoch nie abgeglichen\n\n');
      else md.appendMarkdown(`\nabgeglichen ${s.last_success}, Revision ${s.revision}\n\n`);
      if (s.last_error) md.appendMarkdown(`letzter Fehler ${s.last_error_at}: ${s.last_error}\n\n`);
    }
    if ((this.who.unknown_hubs || []).length) {
      md.appendMarkdown(`Token-Verzeichnisse ohne Hub am Node: ${this.who.unknown_hubs.join(', ')}\n`);
    }
    it.tooltip = md;
  }
}

// --- Dateisystem: keph://<hub>/<collection>/… ---

class KephFs {
  constructor(status) {
    this.status = status;
    this._emitter = new vscode.EventEmitter();
    this.onDidChangeFile = this._emitter.event;
    status.onChanged(() => {
      const events = status.hubs().map((h) => ({
        type: vscode.FileChangeType.Changed,
        uri: vscode.Uri.parse(`${SCHEME}://${h.hub}/`),
      }));
      if (events.length) this._emitter.fire(events);
    });
  }

  watch() {
    return new vscode.Disposable(() => {});
  }

  parts(uri) {
    return uri.path.split('/').filter(Boolean);
  }

  dir(permissions) {
    return { type: vscode.FileType.Directory, ctime: 0, mtime: 0, size: 0, permissions };
  }

  async stat(uri) {
    const [col, ...rest] = this.parts(uri);
    if (!col) return this.dir(vscode.FilePermission.Readonly);
    const c = this.status.collections(uri.authority).find((x) => x.collection === col);
    if (!c) throw vscode.FileSystemError.FileNotFound(uri);
    if (rest.length === 0) return this.dir(vscode.FilePermission.Readonly);
    // Inhalte der Collections: kommt mit read (Task 009).
    throw vscode.FileSystemError.FileNotFound(uri);
  }

  async readDirectory(uri) {
    const [col, ...rest] = this.parts(uri);
    if (!col) return this.status.collections(uri.authority).map((c) => [c.collection, vscode.FileType.Directory]);
    if (rest.length === 0 && this.status.collections(uri.authority).some((x) => x.collection === col)) {
      return []; // kommt mit list (Task 009)
    }
    throw vscode.FileSystemError.FileNotFound(uri);
  }

  readFile(uri) { throw vscode.FileSystemError.FileNotFound(uri); }
  createDirectory(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  writeFile(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  delete(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  rename(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
}

// --- Aktivierung ---

let ext;

function activate(context) {
  ext = context.extension;
  const out = vscode.window.createOutputChannel('Kephalaion');
  const log = (s) => out.appendLine(`${new Date().toISOString()} ${s}`);
  const node = new Node(log);
  const status = new Status(node, log);

  context.subscriptions.push(
    out,
    status.item,
    vscode.workspace.registerFileSystemProvider(SCHEME, new KephFs(status), {
      isCaseSensitive: true,
      isReadonly: true,
    }),
    vscode.commands.registerCommand('kephalaion.refresh', () => status.refresh()),
    vscode.commands.registerCommand('kephalaion.showLog', () => out.show()),
    vscode.commands.registerCommand('kephalaion.addCollection', async () => {
      await status.refresh();
      const items = status.hubs().flatMap((h) => status.collections(h.hub).map((c) => ({
        label: c.address,
        description: c.rights.join(', '),
        hub: h.hub,
        collection: c.collection,
      })));
      if (items.length === 0) {
        vscode.window.showWarningMessage('Kephalaion: keine lesbare Collection — Anmeldung prüfen (Statusleiste).');
        return;
      }
      const pick = await vscode.window.showQuickPick(items, { placeHolder: 'Collection als Ordner einbinden' });
      if (!pick) return;
      const n = vscode.workspace.workspaceFolders ? vscode.workspace.workspaceFolders.length : 0;
      vscode.workspace.updateWorkspaceFolders(n, 0, {
        uri: vscode.Uri.parse(`${SCHEME}://${pick.hub}/${pick.collection}`),
        name: `Keph ${pick.label}`,
      });
    }),
    vscode.commands.registerCommand('kephalaion.menu', async () => {
      const pick = await vscode.window.showQuickPick([
        { label: '$(refresh) Neu verbinden', cmd: 'kephalaion.refresh' },
        { label: '$(folder-library) Collection einbinden', cmd: 'kephalaion.addCollection' },
        { label: '$(output) Log anzeigen', cmd: 'kephalaion.showLog' },
      ]);
      if (pick) vscode.commands.executeCommand(pick.cmd);
    }),
  );

  log(`Node: ${nodeUrl() || '(keine Adresse)'}, config ${configFile()}`);
  status.refresh();
  const timer = setInterval(() => status.refresh(), POLL_MS);
  context.subscriptions.push(new vscode.Disposable(() => clearInterval(timer)));
}

function deactivate() {}

module.exports = { activate, deactivate };
