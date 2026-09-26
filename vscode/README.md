# Kephalaion für VS Code

Versuch: ein `FileSystemProvider` für `keph://`, nur lesen, mit festem Inhalt — ohne
Verbindung zum Node. Idee und Weg: [`../docs/vscode.md`](../docs/vscode.md).

Bauen und installieren (unter WSL aus einem WSL-Terminal, dann landet die Erweiterung im
VS-Code-Server der WSL):

```sh
cd vscode && npx --yes @vscode/vsce package --skip-license
code --install-extension kephalaion-0.0.1.vsix
```

Danach „Developer: Reload Window“ und über die Befehlspalette:

- „Kephalaion: Dummy-Datei öffnen“
- „Kephalaion: Dummy-Ordner zum Workspace hinzufügen“
