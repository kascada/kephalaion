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
    return (await this.request(tool, args)).structuredContent;
  }

  // Wie call, dazu der Text des Ergebnisses — bei read der Inhalt des Dokuments.
  async callWithText(tool, args) {
    const r = await this.request(tool, args);
    return { data: r.structuredContent, text: (r.content || []).map((c) => c.text || '').join('') };
  }

  // Tokens je Aufruf frisch, aber höchstens alle 5 s von der Platte (stat kommt oft).
  credentials() {
    const now = Date.now();
    if (!this._creds || now - this._credsAt > 5000) {
      this._creds = readCredentials(this.log);
      this._credsAt = now;
    }
    return this._creds;
  }

  async request(tool, args) {
    const url = nodeUrl();
    if (!url) throw new Error(`keine Adresse des Nodes: listen fehlt in ${configFile()}`);
    const headers = {
      'Content-Type': 'application/json',
      Accept: 'application/json, text/event-stream',
    };
    for (const [hub, c] of Object.entries(this.credentials())) {
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
      const err = new Error(`${tool}: ${t}`);
      err.toolError = true; // Antwort des Werkzeugs, kein Fehler der Verbindung
      throw err;
    }
    return msg.result;
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

  // Dasselbe wie der Tooltip, als Text — für „Kephalaion: Status anzeigen“.
  summary() {
    const lines = [`Kephalaion — Node ${nodeUrl() || '(keine Adresse)'}`];
    if (!this.who) {
      lines.push(`nicht erreichbar: ${this.error}`);
      return lines;
    }
    lines.push(`Version Node ${this.who.version}, Erweiterung ${ext.packageJSON.version}`);
    for (const h of this.hubs()) {
      let l = `Hub ${h.hub} (Node ${h.node}): Anmeldung ${h.login}`;
      if (h.login === 'ok') l += `, Account ${h.account}, User ${h.user}`;
      lines.push(l);
      for (const c of h.collections || []) lines.push(`  ${c.address}: ${c.rights.join(', ')}`);
      const sy = h.sync;
      lines.push(sy.never_synced ? '  noch nie abgeglichen' : `  abgeglichen ${sy.last_success}, Revision ${sy.revision}`);
      if (sy.last_error) lines.push(`  letzter Fehler ${sy.last_error_at}: ${sy.last_error}`);
    }
    if ((this.who.unknown_hubs || []).length) lines.push(`Token-Verzeichnisse ohne Hub am Node: ${this.who.unknown_hubs.join(', ')}`);
    return lines;
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
// Die Authority ist der Hub-Alias, das erste Segment die Collection, der Rest der Name.
// Namen kommen vom Node immer als voller Pfad ab der Collection.

const CHANGES_MS = 3000;

function toFsError(e, uri) {
  if (e.toolError) return vscode.FileSystemError.FileNotFound(uri); // „nicht lesbar“ u. ä.
  return vscode.FileSystemError.Unavailable(`Kephalaion: ${e.message}`);
}

function time(t) {
  const n = t && Date.parse(t.at);
  return Number.isFinite(n) ? n : 0;
}

class KephFs {
  constructor(node, status, log) {
    this.node = node;
    this.log = log;
    this.cursor = undefined;
    this._emitter = new vscode.EventEmitter();
    this.onDidChangeFile = this._emitter.event;
    // Neue Anmeldung oder andere Collections: die Wurzeln neu lesen lassen — nur dann,
    // nicht bei jedem whoami.
    this.shape = '';
    status.onChanged(() => {
      const shape = JSON.stringify(status.hubs().map((h) => [h.hub, h.login, status.collections(h.hub).map((c) => c.collection)]));
      if (shape === this.shape) return;
      this.shape = shape;
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

  split(uri) {
    const [col, ...rest] = uri.path.split('/').filter(Boolean);
    return { hub: uri.authority, col, name: rest.join('/') };
  }

  dir() {
    return { type: vscode.FileType.Directory, ctime: 0, mtime: 0, size: 0, permissions: vscode.FilePermission.Readonly };
  }

  async stat(uri) {
    const { hub, col, name } = this.split(uri);
    if (!col) return this.dir();
    let d;
    try {
      d = await this.node.call('read', { collection: `${hub}:${col}`, name, content: false });
    } catch (e) {
      throw toFsError(e, uri);
    }
    if (d.kind === 'directory') return this.dir();
    if (d.kind !== 'document') throw vscode.FileSystemError.FileNotFound(uri);
    // Schreiben kommt später; bis dahin alles schreibgeschützt, unabhängig von writable.
    return {
      type: vscode.FileType.File,
      ctime: time(d.created),
      mtime: time(d.updated),
      size: d.size || 0,
      permissions: vscode.FilePermission.Readonly,
    };
  }

  async readDirectory(uri) {
    const { hub, col, name } = this.split(uri);
    const args = col ? { collection: `${hub}:${col}`, path: name, limit: 1000 } : { collection: `${hub}:`, limit: 1000 };
    const out = [];
    try {
      for (;;) {
        const r = await this.node.call('list', args);
        for (const e of r.entries || []) {
          const last = e.name.split('/').pop();
          if (e.kind === 'document') out.push([last, vscode.FileType.File]);
          else out.push([last, vscode.FileType.Directory]); // directory, collection
        }
        if (!r.more || !r.cursor) break;
        args.cursor = r.cursor;
      }
    } catch (e) {
      throw toFsError(e, uri);
    }
    return out;
  }

  async readFile(uri) {
    const { hub, col, name } = this.split(uri);
    if (!col || !name) throw vscode.FileSystemError.FileIsADirectory(uri);
    let r;
    try {
      r = await this.node.callWithText('read', { collection: `${hub}:${col}`, name });
    } catch (e) {
      throw toFsError(e, uri);
    }
    if (r.data.kind === 'directory') throw vscode.FileSystemError.FileIsADirectory(uri);
    if (r.data.kind !== 'document') throw vscode.FileSystemError.FileNotFound(uri);
    return new TextEncoder().encode(r.text);
  }

  createDirectory(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  writeFile(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  delete(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  rename(uri) { throw vscode.FileSystemError.NoPermissions(uri); }

  // changes lückenlos weiterfragen und daraus onDidChangeFile auslösen. Ohne cursor liefert
  // der erste Aufruf nur den Ausgangspunkt „ab jetzt“.
  async pollChanges() {
    let r;
    try {
      r = await this.node.call('changes', this.cursor ? { cursor: this.cursor } : {});
    } catch (e) {
      if (e.toolError) this.cursor = undefined; // z. B. ungültiger cursor: neu ansetzen
      return;
    }
    const first = this.cursor === undefined;
    this.cursor = r.cursor;
    if (first) return;
    const events = [];
    const seen = new Set();
    const add = (type, uri) => {
      const k = `${type} ${uri}`;
      if (!seen.has(k)) {
        seen.add(k);
        events.push({ type, uri });
      }
    };
    for (const c of r.changes || []) {
      const [hub, col] = c.address.split(':');
      const uri = vscode.Uri.parse(`${SCHEME}://${hub}/${col}/${c.name}`);
      add(c.deleted ? vscode.FileChangeType.Deleted : vscode.FileChangeType.Changed, uri);
      // Der Explorer liest ein Verzeichnis neu, wenn es selbst geändert gemeldet wird —
      // auch neu entstandene oder leer gewordene Verzeichnisse darüber.
      const segs = c.name.split('/');
      for (let i = segs.length - 1; i >= 0; i--) {
        const dir = [col, ...segs.slice(0, i)].join('/');
        add(vscode.FileChangeType.Changed, vscode.Uri.parse(`${SCHEME}://${hub}/${dir}`));
      }
      this.log(`changes: ${c.address} ${c.name}${c.deleted ? ' gelöscht' : ''} (Revision ${c.revision})`);
    }
    for (const hub of r.reset || []) add(vscode.FileChangeType.Changed, vscode.Uri.parse(`${SCHEME}://${hub}/`));
    for (const d of r.dropped || []) {
      const [hub] = String(d.address || d).split(':');
      add(vscode.FileChangeType.Changed, vscode.Uri.parse(`${SCHEME}://${hub}/`));
    }
    if (events.length) this._emitter.fire(events);
    if (r.more) return this.pollChanges();
  }
}

// --- Aktivierung ---

let ext;

function activate(context) {
  ext = context.extension;
  const out = vscode.window.createOutputChannel('Kephalaion');
  const log = (s) => out.appendLine(`${new Date().toISOString()} ${s}`);
  const node = new Node(log);
  const status = new Status(node, log);
  const kfs = new KephFs(node, status, log);

  context.subscriptions.push(
    out,
    status.item,
    vscode.workspace.registerFileSystemProvider(SCHEME, kfs, {
      isCaseSensitive: true,
      isReadonly: true,
    }),
    vscode.commands.registerCommand('kephalaion.refresh', () => status.refresh()),
    vscode.commands.registerCommand('kephalaion.showLog', () => out.show()),
    vscode.commands.registerCommand('kephalaion.showStatus', async () => {
      await status.refresh();
      out.appendLine('');
      for (const l of status.summary()) out.appendLine(l);
      out.show();
    }),
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
        { label: '$(info) Status anzeigen', cmd: 'kephalaion.showStatus' },
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
  let polling = false;
  const pollChanges = async () => {
    if (polling) return;
    polling = true;
    try {
      await kfs.pollChanges();
    } finally {
      polling = false;
    }
  };
  pollChanges();
  const changesTimer = setInterval(pollChanges, CHANGES_MS);
  context.subscriptions.push(new vscode.Disposable(() => {
    clearInterval(timer);
    clearInterval(changesTimer);
  }));
}

function deactivate() {}

module.exports = { activate, deactivate };
