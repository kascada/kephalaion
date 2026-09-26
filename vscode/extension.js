// Versuch: ein FileSystemProvider für das Schema keph://, nur lesen, mit festem Inhalt.
// Keine Verbindung zum Node; es geht nur darum, ob der Weg in VS Code trägt.
// Siehe docs/vscode.md.

const vscode = require('vscode');

const SCHEME = 'keph';
const started = Date.now();

// Fester Baum: Verzeichnis -> Einträge; Dateien als Text.
const files = {
  '/dummy/hallo.md':
    '# Hallo aus Kephalaion\n\n' +
    'Diese Datei kommt nicht von der Platte, sondern aus einem FileSystemProvider.\n\n' +
    '- Schema: `keph://`\n' +
    '- Nur lesen\n' +
    `- Erweiterung gestartet: ${new Date(started).toISOString()}\n`,
};
const dirs = {
  '/': ['dummy'],
  '/dummy': ['hallo.md'],
};

const enc = new TextEncoder();

function norm(uri) {
  // keph://<authority>/<pfad>: die Authority (später der Hub-Alias) spielt im Dummy keine Rolle.
  const p = uri.path.replace(/\/+$/, '');
  return p === '' ? '/' : p;
}

class DummyFs {
  constructor() {
    this._emitter = new vscode.EventEmitter();
    this.onDidChangeFile = this._emitter.event;
  }

  watch() {
    return new vscode.Disposable(() => {});
  }

  stat(uri) {
    const p = norm(uri);
    if (dirs[p]) {
      return { type: vscode.FileType.Directory, ctime: started, mtime: started, size: 0,
        permissions: vscode.FilePermission.Readonly };
    }
    if (files[p] !== undefined) {
      return { type: vscode.FileType.File, ctime: started, mtime: started,
        size: enc.encode(files[p]).length, permissions: vscode.FilePermission.Readonly };
    }
    throw vscode.FileSystemError.FileNotFound(uri);
  }

  readDirectory(uri) {
    const p = norm(uri);
    const names = dirs[p];
    if (!names) throw vscode.FileSystemError.FileNotFound(uri);
    const base = p === '/' ? '' : p;
    return names.map((n) => [n, dirs[`${base}/${n}`] ? vscode.FileType.Directory : vscode.FileType.File]);
  }

  readFile(uri) {
    const p = norm(uri);
    if (files[p] === undefined) throw vscode.FileSystemError.FileNotFound(uri);
    return enc.encode(files[p]);
  }

  createDirectory(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  writeFile(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  delete(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
  rename(uri) { throw vscode.FileSystemError.NoPermissions(uri); }
}

function activate(context) {
  context.subscriptions.push(
    vscode.workspace.registerFileSystemProvider(SCHEME, new DummyFs(), {
      isCaseSensitive: true,
      isReadonly: true,
    }),
    vscode.commands.registerCommand('kephalaion.openDummyFile', () =>
      vscode.window.showTextDocument(vscode.Uri.parse(`${SCHEME}://dummy/dummy/hallo.md`)),
    ),
    vscode.commands.registerCommand('kephalaion.addDummyFolder', () => {
      const n = vscode.workspace.workspaceFolders ? vscode.workspace.workspaceFolders.length : 0;
      vscode.workspace.updateWorkspaceFolders(n, 0, {
        uri: vscode.Uri.parse(`${SCHEME}://dummy/dummy`),
        name: 'Kephalaion (Dummy)',
      });
    }),
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
