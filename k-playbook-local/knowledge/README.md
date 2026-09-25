---
title: Wissensablage
subject: Index der Wissensablage
origin: k-playbook, Einrichtung der Struktur
state: condensed
format: markdown
updated: 2026-09-25
---

# knowledge

Die Wissensablage: was gilt. Nur was hier liegt, wird indiziert, durchsucht, gelesen
und in einer Antwort zitiert. Immer Markdown mit YAML-Frontmatter; was in einem anderen
Format ankommt, wird am Eingang umgewandelt oder bleibt unter ../inbox/ liegen, mit
einem Markdown-Stub hier, der es beschreibt. Ein Dokument je Thema, fortgeschrieben —
nicht eines je Ereignis.

Der Pfad trägt den Eigentümer, das Frontmatter das Thema: code/ gehört /k-docs-code,
libs/ gehört /k-docs-tools, versions/ gehört /k-doc-inventory, extracted/ gehört
/k-docs-extract, external/<system>/ je einem Connector, findings/ der Sitzung,
pitfalls/ und manual/ einer Person, README.md dem Index-Command /k-docs-index. Genau ein
Eigentümer je Verzeichnis, und nur der schreibt dort: die drei Generatoren code/, libs/
und versions/ schreiben ihr Verzeichnis bei jedem Lauf als Ganzes neu, und was ein
anderer dort abgelegt hätte, wäre danach spurlos weg.

Geschrieben wird ausschließlich über das Werkzeug — `k-playbook knowledge write`,
`publish` und `supersede` oder die MCP-Werkzeuge k_playbook_knowledge_*. Jede
Schreibung nennt ihren Erzeuger, und ein Ziel außerhalb seines Verzeichnisses wird
abgewiesen. Das Frontmatter baut das Werkzeug aus den übergebenen Feldern; niemand
reicht einen fertigen Dateikopf durch. Gelöscht wird nichts: Überholtes bekommt
`state: superseded`, fällt aus der Suche und bleibt lesbar.

Der Suchindex darüber liegt unter ../cache/knowledge/ und ist jederzeit verwerfbar;
Änderungen am Werkzeug vorbei erkennt er selbst über Datei-Hashes.

Dieses Verzeichnis gehört dem Projekt und wird von einem Update nie angefasst.
