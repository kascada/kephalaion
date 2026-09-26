# Kephalaion für VS Code

Zeigt Collections von Kephalaion als Ordner (`keph://<hub>/<collection>/…`) und den Stand des
Nodes in der Statusleiste. Idee und Weg: [`../docs/vscode.md`](../docs/vscode.md).

Stand: Anbindung an den Node über `whoami` — Statusleiste, Menü, Collections einbinden.
Inhalte der Collections kommen mit `list` und `read` (Task 009).

- **Node:** Adresse aus `listen` im Abschnitt `node:` der config
  (`~/.config/kephalaion/config.yaml`, abweichend `KEPHALAION_CONFIG`, `XDG_CONFIG_HOME`);
  überschreibbar mit der Einstellung `kephalaion.nodeUrl`.
- **Anmeldung:** aus `~/.config/kephalaion/tokens/<hub>/<account>.token`; ohne Einrichtung.

Bauen und installieren (unter WSL aus einem WSL-Terminal, dann landet die Erweiterung im
VS-Code-Server der WSL):

```sh
cd vscode && npx --yes @vscode/vsce package --skip-license
code --install-extension kephalaion-0.0.2.vsix
```

Danach „Developer: Reload Window“. Die Statusleiste zeigt `Keph <hub>`; ein Klick öffnet das
Menü (neu verbinden, Collection einbinden, Log).
