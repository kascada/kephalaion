# cache

Was diese Maschine aus dem Projekt ableitet und jederzeit neu bauen kann.

Alles hier ist wiederherstellbar. Wer etwas Unwiederbringliches ablegen will, braucht
ein anderes Verzeichnis — dieses darf ohne Rückfrage gelöscht werden, und sein Inhalt
bleibt aus der Versionskontrolle.

k-playbook legt dafür beim erstmaligen Anlegen dieses Verzeichnisses eine .gitignore mit
diesem Inhalt an:

    *
    !.gitignore
    !README.md

Der Block „Lokale Einstellungen" in der Oberfläche zeigt den gemessenen Ist-Zustand und
schaltet ihn um — auch wieder zurück; einmal umgeschaltet, bleibt es dabei.

knowledge/index.json ist der Suchindex des Wissenstors über ../knowledge/ (`k-playbook
knowledge`, MCP-Werkzeuge k_playbook_knowledge_*). Fehlt er, baut ihn der nächste
Zugriff neu; Änderungen am Tor vorbei erkennt er über Datei-Hashes selbst.

Dieses Verzeichnis gehört dem Projekt und wird von einem Update nie angefasst.
