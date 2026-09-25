# inbox

Der Eingang: was ankommt, bevor jemand es gelesen hat. Chat-Mitschnitte, Notizen,
PDFs, Screenshots, HTML-Abzüge, Exporte — alles, was eine Person oder ein Connector
ablegt, in dem Format, in dem es kommt. Unterteilt allein nach Quelle: inbox/<quelle>/…,
die Quelle ist frei (confluence, chat, mail, scan).

Es gibt keine Namensregel, kein Frontmatter und keine Pflicht über das Ablegen hinaus.
Das ist die Bedingung dafür, dass überhaupt etwas abgelegt wird: ein Eingang, der
Vorbereitung verlangt, wird einmal benutzt. Abgelegt wird über `k-playbook knowledge
inbox put`, das MCP-Werkzeug k_playbook_knowledge_inbox_put oder schlicht mit dem
Dateimanager.

Der Eingang ist ein Archiv, keine Warteschlange. Etwas kann monatelang hier liegen,
ohne dass jemand hineinsieht, und nichts ist offen, nur weil es hier liegt. Er wird
nie indiziert und taucht in keiner Suche auf. Eine Verarbeitung verbraucht ihn nicht:
ein Rohstück bleibt, nachdem daraus Wissen gemacht wurde — nur so lässt sich ein
schlechter Extrakt aus derselben Quelle noch einmal ziehen. Gelöscht wird hier
ausschließlich von Hand, nie als Nebenwirkung eines Laufs. Was kein Markdown ist,
bleibt für immer hier; das Wissensdokument unter ../knowledge/ zeigt darauf.

Der Inhalt wird ganz normal mitversioniert. Rohmaterial enthält typischerweise
Tokens, Pfade und Namen; soll es nicht ins Repository, schaltet der Block
„Lokale Einstellungen" in der Oberfläche dieses Verzeichnis um — er legt die
.gitignore an und nimmt bereits versionierte Dateien aus dem Index.

Von Hand geht es genauso: eine .gitignore in diesem Verzeichnis mit diesem
Inhalt:

    *
    !.gitignore
    !README.md

Was bereits committet ist, nimmt erst ein `git rm --cached` wieder heraus. Und
was schon gepusht wurde, bleibt in der Historie.

Dieses Verzeichnis gehört dem Projekt und wird von einem Update nie angefasst.
