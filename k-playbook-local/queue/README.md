# queue

Die Warteschlange: was noch aussteht. Ein Eintrag ist ein Stück Arbeit — dieses
Rohstück soll Wissen werden. Er ist eine kleine Markdown-Datei, die auf ihre Quelle
im Eingang verweist statt sie zu kopieren, und das Zielverzeichnis unter ../knowledge/
und den Grund nennt. Angelegt wird er über `k-playbook knowledge queue add` oder das
MCP-Werkzeug k_playbook_knowledge_queue_add, das die Kennung selbst vergibt.

Nach der Übernahme in die Ablage wird der Eintrag gelöscht — nicht verschoben, nicht
archiviert, nicht abgehakt. Das erledigt `k-playbook knowledge write` selbst, wenn
ihm der Eintrag genannt wird: gelöscht wird, nachdem das Dokument steht, und nur dann.
Eine leere Warteschlange heißt deshalb: nichts offen. Das ist der ganze Zweck des
Verzeichnisses. Ein Rückstand, der sich nur durch Zählen oder Filtern feststellen
lässt, ist einer, den niemand liest.

Ein Lauf, der scheitert, lässt seinen Eintrag liegen. Das Verzeichnis hält Arbeit,
nicht Geschichte; was aus einer Quelle geworden ist, steht im origin des fertigen
Dokuments. Wer einen Eintrag ohne Übernahme loswerden will, nimmt `k-playbook
knowledge queue drop`. Indiziert wird hier nichts.

Dieses Verzeichnis gehört dem Projekt und wird von einem Update nie angefasst.
