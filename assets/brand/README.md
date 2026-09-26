# Markenzeichen

Logo und Bildchen von Kephalaion. Die SVG-Dateien sind die Vorlage, alles andere wird daraus
exportiert.

| Datei | Wofür |
|---|---|
| `logo.svg` | Logo ohne Untertitel |
| `logo-claim.svg` | Logo mit dem Claim „Knowledge, distilled.“ |
| `mark.svg` | nur das Zeichen — Favicon, GitHub-Avatar, Icon der VS-Code-Erweiterung |
| `png/` | Exporte in festen Größen (16 bis 512 px), Social Preview für GitHub (1280×640) |

**Untertitel** (siehe `docs/konzept.md`, „Der Name“): „Knowledge, distilled.“ ist der Claim
und steht am Logo. „Shared memory for humans and agents“ ist die Beschreibung und steht, wo
Platz ist. Im kleinen Logo steht keiner, in Headern und Bannern beide, der Claim oben.

**Farben und Schrift:** noch offen.

**Verwendung:** Die VS-Code-Erweiterung packt nur `vscode/`; ihr Icon wird beim Bauen dorthin
kopiert. Eine spätere Oberfläche bettet die Dateien über ein Go-Paket `assets` ein
(`//go:embed brand`), das erst mit ihr angelegt wird.
