---
title: Kephalaion — Konzept
description: Entwurf für eine geteilte Wissensdatenbank mehrerer Nutzer und Projekte — ein Binary mit zwei Rollen, dem Hub für Store, Journal und Accounts und dem Node als lokalem MCP-Server mit Replica —, mit zentralem Schreiben, Collections und Rechten. Entstanden in k-playbook.
---

# Kephalaion — Konzept

**Stand: Entwurf, nichts davon ist gebaut.** Die Überlegungen entstanden in k-playbook und
sind am 2026-09-25 hierher umgezogen. Begriffe nach [`begriffe.md`](begriffe.md): Sie sind
englisch, die Dokumentation ist deutsch.

Ausgangspunkt ist die lokale Wissensablage von k-playbook (`k-playbook-local/knowledge/`,
beschrieben in `k-playbook/docs/knowledge-gate.md` und `knowledge-layout.md`). Sie bleibt
vorerst, wie sie ist; k-playbook gibt seine Suche schrittweise an Kephalaion ab.

## Der Name

**Kephalaion** (κεφάλαιον), von κεφαλή, Kopf: die Hauptsache, der Kernpunkt, die Summe einer
Sache. Im Geldwesen der Grundbetrag im Gegensatz zu den Zinsen — daraus wurde über das
lateinische *capitale* unser „Kapital“; im heutigen Griechisch heißt κεφάλαιο Kapitel und
Kapital zugleich.

Ausschlaggebend war die Gattung: **Kephalaia** sind gesammelte, durchnummerierte kurze
Lehrsätze, jeder für sich lesbar — die manichäischen „Kephalaia des Lehrers“, die „Kephalaia
Gnostika“ des Euagrios. Genau das ist dieser Store: verdichtete Stücke, einzeln
auffindbar.

Recherche am 2026-09-24: kein Softwareprojekt und keine Marke dieses Namens gefunden, auch
nicht in der Nische der KI-Gedächtnisse — anders als bei Mneme, Episteme, Pinakes, Scrinium,
Hypomnema, Anamnesis und Armarium, die dort alle mehrfach belegt sind. Belegt ist dagegen die
neugriechische Form **Kefalaio**: ein verbreitetes griechisches ERP und eine Zeitung. Andere
Schreibweise, anderer Markt, aber wer Griechisch spricht, hört zuerst „Kapital“.

**Benennung:** Produkt, Paket und Binary heißen `kephalaion`. Der Dienst ist
`kephalaion serve`; welche Rollen er trägt — hub, node oder beide —, steht in der
Konfiguration. Im Gespräch: Keph. Im Code und in der Konfiguration steht der volle Name —
er wird gelesen, nicht getippt.

**Repository:** `kascada/kephalaion` auf GitHub, öffentlich — entschieden am 2026-09-25.

**Vor der Veröffentlichung zu prüfen:** GitHub-Organisation, Domains in den Schreibweisen
Kephalaion, Kefalaion und Cephalaion, sowie DPMA und EUIPO.

## Wozu

Wissen, das nur in einem Projektverzeichnis liegt, stirbt mit dem Projekt und hilft keinem
zweiten Menschen. Gebraucht wird ein Store, den mehrere Nutzer und mehrere Projekte
gemeinsam benutzen — mit Rechten, ohne Clone, ohne Merge-Konflikte.

Was dabei nicht verhandelbar ist: **Fragen muss schneller sein als Suchen.** Das ist der
einzige Grund, warum ein Assistent die Ablage überhaupt benutzt, statt den Code zu greppen.
Gemessen sind heute rund 20 ms bei 506 Abschnitten; ein Netzweg kostet das Fünf- bis
Fünfundzwanzigfache.

## Die tragende Entscheidung: einer schreibt, viele lesen lokal

| Vorgang | Wo | Geschwindigkeit |
|---|---|---|
| Suchen, Lesen, Auflisten | Replica auf dem Node | wie heute, ohne Netz |
| Schreiben, Ablösen, Anhängen | Hub | darf langsam sein |
| Abgleich der Replica | Hub → Node | im Hintergrund |

Daraus folgt alles Weitere:

- **Es gibt genau einen Schreiber**, den Hub. Gleichzeitiges Schreiben mehrerer Accounts gibt
  es damit nicht, und ein Zusammenführen von Konflikten braucht es nicht.
- **Die Replica ist abgeleitet.** Sie wird nie selbst beschrieben. Ein Aufruf geht zum Hub,
  dessen Antwort sagt, was gespeichert wurde und wo, und der Node schreibt genau das. Zwei
  Wahrheiten wären der Anfang vom Auseinanderlaufen.
- **Offline wird gelesen, nicht geschrieben.** Ist der Hub nicht erreichbar, scheitert ein
  Schreibvorgang mit einer klaren Meldung. Lesen läuft weiter.

## Ein Programm, zwei Rollen

**Entschieden am 2026-09-25:** Kephalaion ist in Go geschrieben und ein einziges Binary.
Ein Dienst `kephalaion serve` trägt die Rollen, die in der Konfiguration stehen; der frühere
Gedanke, Hub und Node als zwei Programme in womöglich zwei Sprachen zu bauen, ist verworfen —
die Zerlegung in Abschnitte und der Vertrag gäbe es sonst zweimal.

```text
Client (lokal)    ──MCP über HTTP──────► node ──HTTPS / SSH / local──► hub
Client (entfernt) ──MCP über HTTPS─────► node
k-playbook        ──MCP über HTTP──────► node
```

| | node | hub |
|---|---|---|
| Gestartet durch | `kephalaion serve`, Abschnitt `node:` | `kephalaion serve`, Abschnitt `hub:` |
| Läuft | einmal je Rechner, als Dienst | einmal je Installation, als Dienst |
| Datenbank | eigene: die Replica, eine je Hub | eigene: der Store |
| Spricht mit | Clients (MCP über HTTP), Hubs | Nodes |
| Besitzt | Replica, Index, Suche, Revisionen | Store, Journal, ids, Accounts, Protokoll |
| Schreibt | nur die Replica, und nur was der Hub geantwortet hat | den Store |

**Der Hub ist kein MCP-Server.** Er spricht mit Nodes, nicht mit Clients, und er sucht
nicht.

**Hub und Node im selben Prozess — entschieden am 2026-09-25.** Das wird der häufige Fall
sein. Stehen in der Konfiguration beide Abschnitte, trägt ein Prozess beide Rollen:

```yaml
hub:                          # nur auf dem Rechner des Hubs
  listen: 0.0.0.0:8443        # extern erreichbar, wenn eingestellt
  data: ~/.local/share/kephalaion/hub.db
node:
  listen: 127.0.0.1:7433
  hubs:
    - name: privat
      transport: local        # derselbe Prozess: Funktionsaufruf
    - name: cloudeteer
      url: https://keph.intern.cloudeteer.de
      token_file: ~/.config/kephalaion/cloudeteer.token
```

- **Zwei Umsetzungen des Vertrags:** über HTTP — direkt per TLS oder durch SSH getunnelt, das
  ist derselbe Client mit anderem Verbindungsaufbau — und lokal als Funktionsaufruf. Der Node
  kennt nur die Schnittstelle, nicht die Umsetzung. `transport: local` ist ein Eintrag in der
  Liste der Hubs wie jeder andere; daneben kann derselbe Node entfernte Hubs bedienen.
- **Der lokale Weg prüft genauso.** Auch beim Funktionsaufruf trägt der Node sein eigenes
  Token und das des Accounts, und der Hub prüft beide. Sonst gäbe es einen Weg ohne Prüfung,
  den kein Test der HTTP-Seite findet. Die Tests des Vertrags laufen gegen beide Umsetzungen.
- **Getrennte Datenbanken.** Der Node behält seine eigene Replica, auch neben dem Hub, statt
  aus dessen Datenbank zu lesen: ein einziger Lesepfad im Node, kein Suchindex im Hub, keine
  Leser in der Datei des einzigen Schreibers. Die doppelten Daten sind wenige Megabyte. Der
  Abgleich ist lokal sofort da; statt eines Ereignisstroms genügt ein Signal im Prozess.
- **Der Hub bleibt extern erreichbar**, wenn eingestellt — für Nodes anderer Rechner —, und
  wird zugleich lokal direkt aufgerufen.
- **Die Kopplung ist gewollt.** Ein Absturz oder Update betrifft beide Rollen; im Code bleiben
  sie getrennt: Der Hub kennt den Node nicht, der Node kennt den Hub nur über die
  Schnittstelle.

**Die Suche liegt vollständig bei Kephalaion.** Während des Übergangs gibt k-playbook seine
Suche schrittweise ab; der Endzustand ist ein einziger Ort, an dem zerlegt, indiziert und
gesucht wird.

**Der Vertrag zwischen Node und Hub bleibt das wichtigste Stück Arbeit**, auch in einem
Binary. Er ist eine harte Schnittstelle mit zwei Umsetzungen — entfernt und im Prozess — und
wird zuerst geschrieben, nicht nebenbei: Vorgänge, Felder, Fehlercodes, die Bedeutung der
Revision, die ids, das Ersetzen eines ganzen Verzeichnisses. Er trägt eine Fassung, und der
Hub bedient auch ältere Nodes — sonst zwingt jedes Update des Hubs alle Rechner zum
Mitziehen.

## Kommunikation

**Entschieden am 2026-09-25: Der Node ist ein MCP-Server über HTTP (Streamable HTTP), lokal
wie entfernt, mit einem einzigen Eingang.** Er läuft einmal je Rechner als Dienst (systemd
`--user`, launchd); jede Anfrage bedient eine eigene Goroutine. HTTP auf `127.0.0.1` braucht
keinen Webserver auf dem Rechner. Zusätzlich prüft k-playbook beim Briefing, ob der Node
läuft, und startet ihn sonst. Nur der Node hält den Index warm, später ein
Einbettungsmodell, und nur er führt die Abgleichschleife und die Verbindungen zu den Hubs.

**Eine Bridge für stdio ist zurückgestellt.** Bei stdio startet der Client den MCP-Server als
eigenen Kindprozess und kann sich nicht an einen laufenden hängen; jede Sitzung bekäme ihren
eigenen Prozess. Eine Bridge wäre der Prozess, den der Client startet, und reichte stdio an den
Node weiter. Sie wird erst gebaut, wenn ein Client zwingend stdio braucht; am Node ändert sie
nichts.

**Zustandslos.** Jede Anfrage trägt Account-Name und Token als HTTP-Header, je Hub ein Paar; der Client trägt
sie aus seiner MCP-Konfiguration ein, die KI sieht sie nicht. Eine Sitzung (`Mcp-Session-Id`)
wird nicht geführt. Die Prüfung je Anfrage kostet Mikrosekunden: ein Nachschlagen der
Account-Zeilen in der Replica über einen Index, ein SHA-256 über das Token und ein Vergleich in
konstanter Zeit. Einen Cache im Speicher gibt es bewusst nicht — er müsste nach jedem
Abgleich, `rotate` und jeder Sperre nachgezogen werden, und eine vergessene Stelle ließe ein
gesperrtes Token still weiter gelten. Die Datenbank ist die einzige Wahrheit. Was sonst über
Aufrufe hinweg reichen müsste, etwa das Weiterblättern in Treffern, steht als Cursor in der
Antwort.

**Lokales HTTP absichern.** Ein lokaler Node bedient nicht nach außen. Er lauscht auf
`127.0.0.1` und prüft die Header `Host` und `Origin`, wie die MCP-Spezifikation es für lokale
Server verlangt — sonst könnte eine Webseite im Browser über DNS-Rebinding Anfragen an den
Node schicken.

**Devcontainer nutzen den Node des Hosts.** Entschieden am 2026-09-25: kein eigener Node je
Container. Weil `127.0.0.1` im Container der Container selbst ist, lauscht der Node zusätzlich
auf der Schnittstelle, über die Container den Host erreichen (Docker-Bridge,
`host.docker.internal`) — und nur dort, nicht im übrigen Netz.

**Anfragen werden beantwortet, während der Abgleich läuft.** Die Suche liest den zuletzt
bestätigten Stand der Replica (SQLite im WAL-Modus trennt Leser und Schreiber). Ausnahme ist
der eigene Schreibvorgang: Dessen Ergebnis steht in der Replica, bevor die Antwort an den
Client geht.

**Entfernt: MCP über HTTPS** mit Token, später OAuth, für Clients ohne eigenen Node. Es ist
derselbe Eingang.

**Node ↔ Hub: ein Protokoll, zwei Transportwege** — dazu der Funktionsaufruf im selben
Prozess (`local`, siehe oben). Das Protokoll ist HTTP mit JSON und der Fassung im Pfad, kein
MCP; es ist zustandslos, die Revision trägt der Node.

- **Direkt über TLS.** Neue Revisionen meldet der Hub über einen Ereignisstrom (Server-Sent
  Events) oder Long-Polling; das Delta holt der Node danach selbst.
- **Über SSH.** Der Node hält eine stehende SSH-Verbindung (Go-Bibliothek, kein externes
  `ssh`), mit Keepalive und Neuaufbau, und tunnelt dasselbe HTTP zum Hub, der dann nur auf
  `localhost` lauscht.
- **Die Identität ist immer das Token.** Der Transport verschlüsselt und bringt durch die
  Firewall, mehr nicht. Ein SSH-Schlüssel ist kein zweites Rechtemodell.

**k-playbook ↔ Kephalaion.** Zusammenlegen ist nicht sinnvoll: k-playbook wird je Projekt
installiert, Kephalaion je Rechner, und Kephalaion ist auch ohne k-playbook nützlich.
k-playbook steuert selbst Abläufe, startet Sitzungen und Werkzeuge und greift deshalb selbst
auf Kephalaion zu, **ebenfalls über MCP**, ist also selbst ein Client. Die KI-Sitzungen,
auch die von k-playbook gestarteten, sprechen für die Suche direkt mit Kephalaion. Welche MCP-Server eine KI hat und mit welchen
Zugangsdaten, sagt ihr das Briefing von k-playbook; das ist Sache der Gegenseite.

## Collections, Accounts, Rechte

**Collection** ist die Einheit, über die Rechte vergeben werden, und zugleich die Einheit, die
abgeglichen wird. Collections eines Hubs sind unabhängig und überschneiden sich nicht. Eine
Collection hat einen Namen. Was eine Collection ist — Team, Produkt, Thema — entscheidet der
Betrieb, nicht das Werkzeug. „Privat“ ist kein Sonderfall, sondern eine Collection mit genau
einem berechtigten Account.

**Account** ist, wer zugreift — Mensch, KI oder Programm: ein Name und ein Token. Das Token
liegt nie im Repository, sondern beim Nutzer auf dem Rechner.

**Rechte** je Account und Collection — entschieden am 2026-09-25:

| Recht | Bedeutung |
|---|---|
| `read` | Suchen, Lesen. Hat jeder Account, der in der Collection eingetragen ist. |
| `write` | Neues anlegen; **Eigenes** ändern und löschen (`created_by` ist der Account). Einen gelöschten Namen neu anlegen darf jeder mit `write`. |
| `supersede` | **Fremdes** ändern, ablösen und löschen. |
| `replicate` | Nur für Nodes: Inhalt und Account-Zeilen der Collection abgleichen. |

Später, falls gebraucht: `write` als Liste von Namenspräfixen statt `true` (etwa `["eins/",
"zwei/"]`) — Präfixe, keine Regex; bei Rechten ist „passt versehentlich mehr“ die gefährliche
Richtung. Faustregel: Unterscheidet sich, wer *lesen* darf, gehört es in eine eigene
Collection; unterscheidet sich nur, wer *schreiben* darf, genügt ein Präfix. Schnipsel bekommen,
wenn sie gebaut werden, ein eigenes Recht (`submit`).

**Lesen ist grob, Schreiben feiner.** Ein Store, der beim Lesen filtert, zwingt jede Suche zu
einer Rechteprüfung je Treffer und macht den Abgleich je Account verschieden. Wer eine
Collection nicht sehen soll, bekommt sie nicht — das ist die Grenze, und sie verläuft an der
Collection.

Fremdes zu ändern, abzulösen oder zu löschen ist ausdrücklich ein eigenes Recht: Es nimmt
etwas aus der Suche, und wer schreiben darf, darf deshalb nicht automatisch löschen, was ein
anderer beigetragen hat.

**Urheber.** Jedes Dokument trägt, wer es angelegt und zuletzt geändert hat: den Account
(`created_by`, `updated_by`). Die Herkunft (`origin`) bleibt davon unberührt — sie sagt,
woher der Inhalt stammt, der Urheber sagt, wer ihn abgelegt hat.

## Der Weg eines Eintrags

```text
Client ──► Node ──► Hub ──► Store
             ▲              │
             └── Antwort: id, Collection, Name, Revision
                    │
             Replica schreibt genau das
```

**Der Hub darf umsortieren.** Er kann ein Dokument anders einordnen, als der Aufrufer es
vorgeschlagen hat — anfangs nach Regeln, später selbsttätig. Deshalb ist seine Antwort die
Wahrheit und nicht der Wunsch des Aufrufers.

**Daraus folgt eine `id` je Dokument.** Ein Dokument hat eine stabile `id`, der Name ist nur
ein Merkmal. Sonst wäre jedes Umsortieren für die Replica „gelöscht und neu angelegt“, und
Verweise darauf würden brechen.

## Abgleich

Der Node merkt sich eine Revision je Collection und fragt „was hat sich seit dieser Revision
geändert“. Er bekommt geänderte Dokumente, Umzüge und Löschmarken, und zieht seinen Index
nach — genau der Vorgang, den der lokale Index heute beim Dateiwechsel schon macht.

Angestoßen wird das beim Start und danach regelmäßig, sowie unmittelbar nach einem eigenen
Schreibvorgang.

**Entschieden am 2026-09-25: Abgleich über die Revision, nicht über die Uhrzeit.** Der Hub
ist der einzige Schreiber und vergibt je Schreibvorgang innerhalb der Transaktion eine
fortlaufende Nummer. Ein Zeitstempel taugt nicht als Revision: Uhren springen zurück (NTP), und
zwei Änderungen in derselben Millisekunde machen „alles nach X“ mehrdeutig — beides führt zu
still verpassten Änderungen. Die Zeitstempel bleiben für Menschen. Einzelheiten unter
„Datenmodell“.

**Warum nicht rsync oder Git.** Beide kennen keine Rechte: Sie übertragen alles oder nichts.
Sobald ein Node nur einen Teil der Collections sehen darf, muss der Hub beim Ausliefern
filtern — und dann ist der Änderungsstrom ohnehin der kürzere Weg.

## Zwei Arten von Eingaben

**Reihenfolge, entschieden am 2026-09-25:** Im Kern ist Kephalaion eine Datenbank, die
Dateien deterministisch schreibt und ändert. Das kommt zuerst. Schnipsel, die erst
verarbeitet und dann eingepflegt werden, sind **zurückgestellt**; was unten über sie steht,
bleibt als Richtung stehen, wird aber vorerst nicht gebaut.

**Wissensschnipsel.** Ein Text mit so viel Zusammenhang wie möglich: wofür er gilt, woher er
stammt, was er bedeutet. Der Aufrufer schlägt nichts vor, er liefert. Was daraus wird —
welches Dokument, welche Art, ob mehrere Dokumente — entscheidet der Hub.

**Dateien.** Ganze Dokumente mit eigener Struktur, etwa Tasks. Sie werden nicht verdichtet,
sondern geführt: anlegen, ergänzen, einen Abschnitt ändern, abschließen. Dafür braucht es
keine KI, sondern verlässliche Vorgänge. Weil nur der Hub schreibt und seine Schreibvorgänge
nacheinander ausführt, braucht es keine eigene Sperre.

Der Unterschied ist nicht die Größe, sondern wer über die Form bestimmt. Beim Schnipsel der
Hub, bei der Datei der Aufrufer. Beide liegen im selben Store und werden gleich
indiziert; getrennt sind die **Werkzeuge**.

**Es sind zwei Familien, nicht ein Werkzeug mit zwei Antworten.** Ein Aufrufer, der einen
Schnipsel einliefert, tut etwas anderes als einer, der eine Datei schreibt, und bekommt etwas
anderes zurück.

| | Schnipsel einliefern | Datei schreiben |
|---|---|---|
| Eingabe | JSON mit festen Feldern: Text, wofür, Quelle, Bedeutung, Collection | Name, Inhalt oder Abschnitt |
| Der Hub | nimmt entgegen, ordnet später ein | speichert sofort, deterministisch |
| Antwort | „angenommen“, mit Vorgangsnummer | id, Collection, Name, Revision |
| Danach lokal | **noch nicht verfügbar** | nach dem Abgleich vorhanden |

**Die Einlieferung ist keine Speicherung.** Wer einen Schnipsel abgibt, weiß danach nur, dass
er angekommen ist. Ob daraus ein Dokument wird, mehrere, oder eine Ergänzung an einem
bestehenden, entscheidet der Hub, und sichtbar wird es beim nächsten Abgleich. Die
Vorgangsnummer bleibt auffindbar, damit „was wurde aus meinem Schnipsel“ beantwortbar ist.

**Ein Arbeitsprozess auf dem Hub** nimmt die Einlieferungen der Reihe nach vor. Was er
nicht entscheiden kann, legt er in eine eigene Warteschlange, die ein Mensch bearbeitet. Der
Reihe nach heißt: nachvollziehbar, und ohne zwei gleichzeitige Einordnungen am selben Thema.

**Der Schnipsel bleibt erhalten.** Was hereinkommt, wird archiviert, auch nachdem daraus ein
Dokument wurde — so wie der Eingang der lokalen Ablage. Sonst ist eine maschinelle
Einordnung nicht nachprüfbar und nicht wiederholbar.

**Dateien brauchen keine KI**, sondern verlässliche Vorgänge: anlegen, ergänzen, einen
Abschnitt ersetzen, abschließen — und ein ganzes Verzeichnis in einem Schritt ersetzen, wie es
ein Generator braucht (in k-playbook heute `publish`). Das ist der Teil, der sofort nutzbar
ist.

## Löschen

Drei Stufen:

- **Ablösen** ist der Normalfall: Überholtes bekommt einen Nachfolger und fällt aus der Suche,
  bleibt aber lesbar.
- **Löschen** über einen Node (Eigenes mit `write`, Fremdes mit `supersede`) setzt eine
  Löschmarke: Der Inhalt verschwindet aus Store und Replicas, die Zeile bleibt als Marke. Den
  Namen darf danach jeder mit `write` neu anlegen.
- **Endgültiges Löschen gehört allein dem Admin.** Es läuft direkt am Hub, nicht über einen
  Node, und ist der Weg für etwas Vertrauliches, das versehentlich eingeliefert wurde. Entfernt
  wird auch, was sonst bliebe — Einlieferung, ältere Fassungen, sobald es sie gibt. Im
  Protokoll bleibt, dass gelöscht wurde, von wem und warum — nicht aber der Inhalt.

**Was ein Löschen nicht kann:** Es holt nichts zurück, was schon abgeglichen wurde. Die
Replicas erfahren beim nächsten Abgleich davon und entfernen es; was jemand daraus kopiert oder
gesichert hat, bleibt. Das ist keine Schwäche dieses Entwurfs, sondern die Eigenschaft
verteilter Replicas — die Antwort darauf ist, beim Einliefern vorsichtig zu sein, nicht beim
Löschen gründlich.

## Indizierung

**Indiziert wird auf dem Node, über genau den Ausschnitt, den er replizieren darf.** Das ist
die Replica, und die enthält nur die Collections, für die sein Scope `replicate` gilt. Welche
davon eine einzelne Anfrage sehen darf, entscheidet der Node über die Rechte des Accounts je
Collection — grob, vor der Suche, nicht je Treffer.

Daraus folgen drei Dinge:

- **Die Zerlegung in Abschnitte gibt es nur im Node** und trägt eine Fassung. Ändert sie
  sich, wird der Index neu gebaut; die lokale Ablage von k-playbook macht das schon so.
- **Der Abgleich zieht den Index nach**, Dokument für Dokument: geänderte neu zerlegen,
  gelöschte entfernen. Ein vollständiger Neubau ist nur nötig, wenn sich die Fassung der
  Zerlegung ändert.
- **Einbettungen liefert der Hub mit**, wenn es sie gibt. Sie gehören zu einem Modell, und
  die Replica hält fest, zu welchem — ein Index, der mit einem Modell gebaut und mit einem
  anderen befragt wird, antwortet Unsinn, und das muss sichtbar sein statt still.

**Ein Client ohne eigenen Node** wird nicht vom Hub bedient, sondern von einem Node, der
neben ihm läuft und eine Replica hält wie jeder andere. Sonst gäbe es die Zerlegung in
Abschnitte zweimal. Der Hub sucht nicht.

## Wenn der Hub selbst einordnet (zurückgestellt)

Der Hub soll Schnipsel einordnen, zusammenführen und Überholtes erkennen. Das heißt, dass
im Hub eine KI arbeitet. Zwei Leitplanken, ohne die daraus ein Store wird, dem
niemand mehr traut:

- **Jede maschinelle Entscheidung wird protokolliert:** Eingabe, betroffene Dokumente,
  Begründung, Zeitpunkt, Modell. Das Protokoll ist selbst Teil des Stores und wird von Zeit
  zu Zeit durchgesehen.
- **Unklares wandert in eine Warteschlange**, statt geraten zu werden. Ein Mensch entscheidet
  dort. Das ist dieselbe Zone, die die lokale Ablage schon kennt: Was offen ist, ist sichtbar
  offen.

**Folge für die Antwort.** Eine Einordnung durch eine KI dauert und kann mehrere Dokumente
berühren. Der Schreibvorgang kann dann nicht mehr sagen „liegt unter diesem Pfad“. Zwei
Antwortarten:

| Fall | Antwort |
|---|---|
| deterministisch gespeichert (Datei, einfacher Schnipsel) | id, Collection, Name, Revision |
| zur Einordnung angenommen | Vorgangsnummer und die Aufforderung, abzugleichen |

Nach dem Abgleich sieht der Node, was daraus geworden ist. Die Vorgangsnummer bleibt
auffindbar, damit „was wurde aus meinem Schnipsel“ beantwortbar ist.

Ob der Hub wirklich eine KI enthalten soll, wird **getrennt verfolgt**: Es bringt
Kosten, Abhängigkeit von einem Anbieter und eine neue Fehlerquelle in einen Dienst, der sonst
nur verwaltet. Die erste Stufe kommt ohne aus.

## Transport, Token und Fehlschläge

Zwischen Node und Hub gibt es nur wenige Vorgänge: einliefern, schreiben, abgleichen,
verwalten. Die Wege dafür stehen unter „Kommunikation“.

**Token.** Jeder Zugriff trägt Account-Name und Token (siehe „Authentifizierung“). Der Hub
verwaltet sie: ausstellen, Rechte je Collection hinterlegen, zurückziehen. Lokal liegt das
Token in der Nutzerkonfiguration mit engen Rechten, nie im Repository.

**Ein Schreibvorgang scheitert auf zwei Arten, und beide werden gemeldet:**

- **Abgelehnt** — fehlendes Recht, unbrauchbare Eingabe, unbekannte Collection. Das ist
  endgültig; der Aufrufer erfährt den Grund und kann korrigieren. Kein erneuter Versuch.
- **Nicht erreichbar** — der Node versucht es mit wachsendem Abstand erneut und meldet nach
  einer Frist, dass nichts gespeichert wurde. Ausdrücklich: *nichts gespeichert*, nicht
  „vielleicht“.

## Authentifizierung

**Account-Name und Token, wie Benutzername und Passwort.** Jede Anfrage trägt beide. Hub und
Node speichern nur `sha256(token)`; geprüft wird, indem der Account über seinen Namen
nachgeschlagen, das vorgelegte Token gehasht und in konstanter Zeit verglichen wird. Ein Hash
wird nirgends als Ausweis angenommen, und kein Vorgang antwortet auf einen Hash allein. Der
Hash schützt, was liegt — eine kompromittierte Datenbank enthält keine brauchbaren
Zugangsdaten. Was unterwegs ist, schützt der Transport. Token sind Zufallswerte mit 256 Bit;
ein einfacher SHA-256 genügt, langsame Verfahren braucht es nur für Passwörter. Format:
`keph_<geheimnis>` — der Präfix lässt Scanner wie gitleaks ein eingechecktes Token erkennen.

Der Name bringt kryptographisch nichts dazu, praktisch aber: Fehlversuche lassen sich einem
Account zuordnen und begrenzen, ein vertauschtes Token fällt auf, und Logs nennen Namen statt
Geheimnisse. Unbekannter Name und falsches Token bekommen dieselbe Antwort.

**Wo das Token unterwegs ist:**

| Strecke | Wie oft | Schutz |
|---|---|---|
| MCP-Konfiguration des Clients → Node | je Anfrage, als Header | nur `127.0.0.1`, Datei mit `0600`; die KI sieht es nicht |
| Node → Hub | je Vorgang in fremdem Namen | TLS oder SSH |

Das Token steht nie in einem Tool-Call.

**Zwei Identitäten je Anfrage an den Hub.** Der Node meldet sich immer mit seinem eigenen
Token an — er ist der Träger. Das Token des Accounts steht in der einzelnen Anfrage und sagt,
in wessen Namen. Ein Node, der im eigenen Namen für andere schreiben dürfte, wäre ein
Generalschlüssel.

**Wer wann prüft:**

| Vorgang | Node | Hub |
|---|---|---|
| Lesen | prüft gegen die Account-Zeilen seiner Replica | — |
| Schreiben | reicht durch | prüft |
| `rotate` | reicht durch, muss den Account nicht kennen | prüft, liefert den Account-Eintrag |

**Einrichtung.** Collections und Accounts legt der Admin am Hub per Kommandozeile an; sie
liegen in der Datenbank des Hubs, die Konfigurationsdatei enthält nur, was der Dienst zum
Starten braucht. Ein Account — für einen Node oder für einen Menschen, eine KI oder ein
Programm — bekommt Name, Kurzbeschreibung und Scopes
(`kephalaion hub account add <name> --scope team-x:write …`). Der Hub
erzeugt ein Token, zeigt es einmal an und speichert nur den Hash. Das Token wird vorerst von
Hand übergeben. Ein Node trägt es in seine Konfiguration ein (`0600`).

**Der erste Vorgang jedes Accounts ist ein `rotate`.** Einrichtung und Rotation sind derselbe
Vorgang:

1. Der Account erzeugt ein neues Token und speichert es als „ausstehend“ neben dem alten,
   bevor er etwas abschickt.
2. Er sendet das **alte Token** zur Anmeldung und den **Hash des neuen**.
3. Der Hub prüft das alte Token, ersetzt den Hash und liefert dem Node den Eintrag des
   Accounts — beschränkt auf die Collections, die dieser Node repliziert. Hat der Account
   keine davon, ist das ein Fehler.
4. Der Node schreibt den Eintrag in seine Replica und antwortet mit „ok“. Der Account
   weiß damit: Der Hub ist erreichbar, das neue Token gilt, und der Node kennt ihn.
5. Erst dann löscht der Account das alte Token.

Das Einrichtungs-Token ist danach wertlos. Nebeneffekt: Ein neuer Account ist sofort auf dem
Node bekannt, ohne auf einen Abgleich zu warten.

**Wenn das „ok“ ausbleibt,** meldet sich der Account mit dem neuen Token an (`whoami`).
Gelingt das, war die Rotation erfolgreich; sonst wiederholt er sie mit dem alten. Der Hub
kennt keinen Wiederholungsfall, `rotate` verlangt immer ein gültiges altes Token. Der
Mechanismus wird so definiert; ein Account implementiert die Wiederholung, wenn er will.

**Sperren** wirkt beim Schreiben sofort, denn geschrieben wird nur über den Hub. Beim Lesen
wirkt es auf einem Node erst mit dem nächsten Abgleich; der Hub meldet Sperren deshalb sofort
über den Ereignisstrom.

**Die Grenze beim Lesen ist der Rechner.** Die Replica liegt unverschlüsselt beim Node. Auf
dem Rechner haben nur root und der Node Zugriff auf sie; das genügt. Die Token-Prüfung im
Node ordnet Anfragen ihren Collections zu. Die harte Grenze ist, was der Hub auf den Rechner
lässt — das bestimmt der Scope `replicate` des Node-Accounts.

**Was der Hub über einen Node weiß:** nur seinen Account und dessen Scopes. `replicate`
umfasst den Inhalt der Collection und ihre Account-Zeilen (`SYSTEM:A:<account>`, siehe
„Datenmodell“). `read` allein bekommt die Account-Zeilen nicht. Welche Collections ein Node tatsächlich
hält, verfolgt der Hub nicht.

## Mehrere Hubs

Ein Node bedient mehrere Hubs — etwa den eines Arbeitgebers, dessen Daten dessen Netz nicht
verlassen dürfen, und einen privaten. Mehrere Nodes auf einem Rechner wären schlechter: mehrere
Dienste, mehrere Ports, mehrere MCP-Einträge je Client.

- **Hubs wissen nichts voneinander.** Jeder hat eigene Collections, Accounts und Token. Sie
  treffen sich nur im Node.
- **Adressen sind zweistufig.** Auf dem Hub bleibt alles hub-lokal (`team-x:write`). Im Node
  und für Clients gilt `<hub>:<collection>`; den Hub-Namen vergibt der Node als Alias in
  seiner Konfiguration.
- **Der Node ist auf jedem Hub ein eigener Account**, mit eigenem Token und eigener Rotation.
  Die Konfiguration führt im Abschnitt `node:` eine Liste von Hubs: Name, Adresse, Transport
  (`https`, `ssh`, `local`), SSH-Schlüssel, Token-Datei.
- **Abgleich je Hub.** Ist ein Hub nicht erreichbar, laufen die anderen weiter.
- **Ein Client trägt mehrere Paare aus Name und Token**, je Hub höchstens eins. Die Suche
  geht über alle Collections, die diese Accounts lesen dürfen.

**Kein Schutz gegen Abfluss zwischen Hubs.** Ein Client mit Token für zwei Hubs kann aus
dem einen lesen und in den anderen schreiben. Das zu verhindern ist nicht Aufgabe von
Kephalaion; wer Daten übertragen will, kann das auch anders. Geschützt wird, dass die
Datenbank eines Hubs nicht anderswo läuft und kein Fremder auf sie zugreift.

## Speicherung

**Entschieden am 2026-09-25: Das Journal ist eine Datenbank.** Der Abgleich fragt „was hat
sich seit Revision X geändert“; dafür braucht es eine fortlaufende Folge von Änderungen,
Umzügen und Löschmarken. Git als interner Speicher scheidet aus, weil das echte
Löschen durch den Admin dann ein Umschreiben der Historie wäre.

**Entschieden am 2026-09-25: Auch die Inhalte liegen in der Datenbank**, nicht in
Markdown-Dateien daneben; Markdown gibt es als Export. Die Entscheidung steht: Ist die Suche
zu langsam, wird am Index oder an der Suche gearbeitet, nicht an der Datenbank. Die Gründe:

- **Index nachziehen.** Der lokale Index von k-playbook lädt heute bei jedem Schreiben den
  ganzen Index und schreibt ihn zurück; gemessen 48 ms je Dokument bei 640 Dokumenten, ein
  Import wächst quadratisch (`k-playbook/docs/knowledge-gate.md`, Abschnitt zu den
  Messungen). Ein Node zieht aber laufend Deltas nach. Ein Volltextindex in der Datenbank
  (SQLite FTS5) ändert je Dokument nur dessen Zeilen.
- **Lesen während des Abgleichs.** Der Node beantwortet viele Anfragen parallel, während er
  abgleicht. SQLite im WAL-Modus erlaubt viele Leser und einen Schreiber, ohne selbst gebaute
  Sperren.
- **Delta atomar anwenden.** Replica, Index und Revision bewegen sich in einer Transaktion —
  oder gar nicht. Mit Dateien entsteht ein Fenster, in dem sie auseinanderlaufen. Im Hub gilt
  dasselbe für Journaleintrag und Inhalt.
- **Die Replica soll nicht editiert werden.** Dateien laden dazu ein; eine Datenbank nicht.

Lesen eines einzelnen Dokuments ist dagegen kein Unterscheidungsmerkmal: aus Datei und aus
SQLite gleichermaßen unter einer Millisekunde.

Auf dem Node ist das SQLite über einen reinen Go-Treiber (`modernc.org/sqlite`), weil ohne C
gebaut wird. Im Hub anfangs ebenfalls SQLite — ein einziger Schreiber passt
dazu —, PostgreSQL erst, wenn Nutzerverwaltung, Volltext und Vektoren aus einer Hand kommen sollen. Der Wechsel ist
möglich, ohne den Vertrag zu ändern.

- **Ein Ausweg nach Markdown bleibt — als Export.** Ein Store, den man nur über einen
  laufenden Dienst lesen kann, ist ein Store, den man verlieren kann. Markdown mit
  Frontmatter wird bei Bedarf exportiert, ist aber nicht der Speicher.

**Was dabei zu beachten ist:** FTS5 rechnet BM25 anders als der Index von k-playbook. Die
Ranking-Korrektur aus Task 056 (Zeiger-Dokumente vor ihren Zielen) ist neu nachzuweisen. Die
Zerlegung in Abschnitte bleibt eigener Code; FTS5 indiziert nur, was sie liefert.

**Wo die Replica liegt: nicht im Projekt.** Fremdes Wissen hat in der Versionierung eines
Projekts nichts verloren. Ein Ort je Rechner, eine Datenbank je Hub, etwa unter
`~/.local/share/kephalaion/`, gemeinsam für alle Projekte dieses Rechners.

## Datenmodell

**Entschieden am 2026-09-25: eine Datenbank je Hub**, alle Collections in denselben Tabellen,
die Collection ist eine Spalte. Collections werden alle gleich behandelt; unterschieden wird
nur beim Zugriff über Token und Scope. Mehrere Tabellen oder Datenbanken je Collection würden
den Abgleich zu mehreren Abfragen gegen mehrere Dateien machen. Auf dem Node gilt dasselbe je
Hub.

**Erster Entwurf:**

```sql
CREATE TABLE documents (
  id          TEXT PRIMARY KEY,        -- ULID, vom Hub vergeben
  collection  TEXT NOT NULL,
  name        TEXT NOT NULL,           -- Pfad oder Bezeichner, frei
  content     TEXT,                    -- NULL, wenn gelöscht
  meta        TEXT,                    -- freies JSON, der Hub deutet es nicht
  deleted     INTEGER NOT NULL DEFAULT 0,
  revision    INTEGER NOT NULL,        -- Abgleich: alles mit revision > X
  created_at  INTEGER NOT NULL,        -- ms seit Epoche
  created_by  TEXT NOT NULL,           -- Account
  updated_at  INTEGER NOT NULL,
  updated_by  TEXT NOT NULL
);
CREATE UNIQUE INDEX documents_name ON documents(collection, name) WHERE deleted = 0;
CREATE INDEX documents_revision ON documents(collection, revision);

CREATE TABLE actions (                 -- Protokoll, befristet
  at          INTEGER NOT NULL,
  account     TEXT NOT NULL,
  carrier     TEXT,                    -- welcher Node es gebracht hat
  action      TEXT NOT NULL,           -- create, update, rename, delete, rotate …
  document_id TEXT,
  revision    INTEGER
);
```

- **Nur Text, kein Typ.** Solange nur Texte gespeichert werden, braucht es keine Typspalte.
- **Metadaten: ein freies Feld `meta` (JSON), vom Hub nicht gedeutet**, nur gespeichert und
  mit abgeglichen. Was hineingehört, entscheidet der nutzende Dienst. Das Frontmatter bleibt
  vorerst im Text; dieselbe Angabe steht nicht an beiden Stellen. SQLite kann JSON-Felder
  abfragen (`json_extract`) und über Ausdrücke indizieren, falls später gefiltert werden soll.
- **Name** ist frei — Pfad oder anderer Bezeichner, je nach Collection und nutzendem Dienst —,
  eindeutig je Collection. Die Logik schaut nicht hinein.
- **Die `id` bleibt**, obwohl Collection und Name eindeutig sind: Ohne sie wäre Umbenennen für
  jeden Node „gelöscht und neu“, Verweise brächen, der Index würde neu gebaut. Mit ihr ist
  Umbenennen eine gewöhnliche Änderung — und der Hub darf später umsortieren.
- **Gelöschtes bleibt als Löschmarke** stehen, ohne Inhalt, mit neuer Revision. Eine
  verschwundene Zeile erführe kein Node. Der eindeutige Index gilt nur für nicht Gelöschtes,
  damit ein Name wieder vergeben werden kann. Ablösen kommt später auf demselben Weg.
- **Auf dem Node zählt allein die `id`.** Die Eindeutigkeit der Namen sichert der Hub; die
  Replica hat keinen eindeutigen Index auf `(collection, name)`. Ein Delta liefert je Dokument
  nur den letzten Stand, und dabei kann ein Name vorübergehend doppelt vergeben sein: A wird
  von `x` in `y` umbenannt, B neu als `x` angelegt, danach A geändert — das Delta bringt B vor
  A, und B hieße `x`, solange A dort noch `x` heißt. Ein eindeutiger Index ließe den Abgleich
  scheitern.
- **Wer erzeugt und geändert hat** steht zweifach: `created_by`/`updated_by` in der Tabelle
  beantworten „wer war zuletzt dran“ ohne Umweg; das Protokoll `actions` den Rest, solange es
  zurückreicht. Es ist befristet; Einträge über echtes Löschen durch den Admin bleiben
  dauerhaft.
- **Schemaänderungen:** SQLite kann `ADD COLUMN`, `RENAME COLUMN` (seit 3.25) und `DROP
  COLUMN` (seit 3.35). Typ oder Bedingung einer Spalte ändert man durch Neubau der Tabelle in
  einer Transaktion. Es braucht eine Schemafassung und Migrationen.
- **History (angedacht, zurückgestellt):** eine zweite Tabelle `document_versions (id,
  revision, content, …)`, in die vor jeder Änderung die alte Fassung kopiert wird, begrenzt
  auf einige Versionen.

**Abgleich — entschieden am 2026-09-25, so einfach wie möglich:**

- **Eine Folge je Hub, eine Revision je Collection.** Die Revision zählt über alle
  Collections des Hubs fort. Der Node merkt sich je Collection die letzte — meist sind
  alle gleich, aber eine neu hinzugekommene Collection beginnt bei 0.
- **Eine Abfrage für alle Collections.** Der Node schickt eine Liste von Paaren (Collection,
  seit); der Hub fragt einmal ab, nach Revision sortiert, jede Collection über den Index
  `(collection, revision)`:
  `WHERE (collection='a' AND revision > 1200) OR (collection='b' AND revision > 0)`.
- **In Seiten, nicht in einem Rutsch.** Die Abfrage liefert eine begrenzte Anzahl und wird
  wiederholt, bis nichts mehr kommt. Jede Seite endet bei einer Revision R. Der Node wendet jede
  Seite in einer Transaktion an und setzt die Revisionen auf R. Ein abgebrochener Abgleich
  setzt dort fort, wo er stand; der erste vollständige Abgleich ist nur eine lange Folge von
  Seiten.
- **Format:** JSON über HTTP, komprimiert (gzip). Ein Datenstrom zeilenweiser JSON-Objekte
  wäre die nächste Stufe, falls Seiten zu groß werden.
- **Accounts sind Zeilen in `documents`**, keine eigene Tabelle — damit gleichen sie sich
  ohne eigenen Mechanismus ab. Je Account und Collection eine Zeile:
  - `name` = `SYSTEM:A:<account>`, also der Account-Name; der eindeutige Index auf
    `(collection, name)` sichert die Eindeutigkeit.
  - `content` = Hash des Tokens und die Rechte in *dieser* Collection, etwa
    `{"hash": "…", "rights": {"write": true, "supersede": false}}`; `read` ergibt sich aus
    der Zeile selbst.
  - Ein Node bekommt mit dem Abgleich genau die Zeilen seiner Collections — der beschränkte
    Auszug fällt von selbst heraus; kein Node erfährt, dass ein Account noch andere
    Collections hat.
  - Sperren und Entziehen laufen über Löschmarken. `rotate` ändert den Content aller Zeilen
    eines Accounts in einer Transaktion.
- **Der Präfix `SYSTEM:` ist reserviert** und die einzige Ausnahme von „der Name ist frei“.
  Am Hub darf kein Account einen solchen Namen anlegen, ändern oder löschen — nur der Hub
  selbst. Am Node kommen diese Zeilen nie in den Suchindex und nie in eine Antwort von
  `read` oder `list`.
- **Ein Index für die Account-Zeilen.** Für eine Suche über alle erlaubten Collections
  braucht der Node alle Zeilen eines Accounts, quer über die Collections; der Index auf
  `(collection, name)` hilft dabei nicht. Dafür:
  `CREATE INDEX documents_system ON documents(name) WHERE name LIKE 'SYSTEM:%';`
  Der Node fragt je Anfrage die Datenbank, ohne Cache.

## Suche über mehrere Collections

Collections unterscheiden sich in Frische, in Zuständigkeit und darin, was ein Treffer
überhaupt bedeutet. Ein gemeinsamer Rang wäre deshalb keine Aussage.

Die Werkzeuge bekommen ein Feld für die Collections (eine Adresse `<hub>:<collection>`, oder
alle erlaubten) und liefern getrennte Trefferlisten. Der Rang zählt je Liste ab 1, wie heute. Ein Punktwert
bleibt aus dem Vertrag heraus.

## Stufen

1. **Nur lesen.** Node liefert Suchen, Lesen, Abgleich; eine Collection, ein Token, ein Recht.
2. **Schreiben mit Rechten, ohne KI.** Dateien werden deterministisch abgelegt, der Hub prüft
   Recht und Form. Accounts, `rotate`, Ablehnungen, Fehlschläge.
3. **Vorgänge auf Dateien.** Anhängen, Abschnitt ändern, abschließen, Verzeichnis ersetzen.
4. **Schnipsel und Einordnen durch den Hub** (zurückgestellt). Zuerst Regeln. Erst danach, und
   getrennt entschieden, eine KI im Hub, mit Protokoll und Warteschlange.
5. **Semantische Suche.** Einbettungen rechnet der Hub einmal für den Store und liefert
   sie mit. Offen bleibt die Frage, die schon einmal zum Verzicht geführt hat: Auch die
   *Frage* braucht eine Einbettung. Entweder ein kleines Modell lokal, oder BM25 zuerst und
   semantisch nur bei schlechter Antwort nachfragen.

## Offene Punkte

- **Wer darf schreiben:** entschieden — allein Token und Rechte bestimmen das, nicht die
  Frage, ob ein Mensch oder eine Sitzung aufruft. Offen bleibt, ob eine Collection zusätzlich
  „nur nach Bestätigung“ verlangen darf.
- **Was ist eine Collection** im Betrieb? Angelegt wird sie vom Admin am Hub.
- **Erreichbarkeit:** entschieden — Verschlüsselung ist Pflicht, der Transport ist wählbar
  (HTTPS, SSH, lokal im selben Prozess). Offen ist, welcher entfernte zuerst gebaut wird.
- **Node als Dienst:** entschieden — er läuft ständig (Benutzerdienst), k-playbook prüft
  beim Briefing zusätzlich. Offen ist nur die Einrichtung je Betriebssystem.
- **k-playbook ↔ Kephalaion im Einzelnen:** Welche Werkzeuge braucht k-playbook, die eine
  KI-Sitzung nicht sehen soll, und hängt das am Token? Das Ersetzen eines ganzen Verzeichnisses
  (heute `publish`) muss in den Vertrag. Die Projektablage und das Briefing regelt k-playbook.
- **Token-Rotation (später):** Accounts von Menschen und KIs rotieren am Hub mit einer Frist,
  in der altes und neues Token gelten; der neue Hash gleicht sich zu den Nodes ab. Ein Node
  rotiert sein Token selbst: Er erzeugt ein neues und meldet dem Hub nur den Hash, ab da gilt
  das neue. Wie das neue Token bis zu k-playbook gelangt, ist offen.
- **Vormerken bei Nichterreichbarkeit:** derzeit nein, es wird gemeldet. Ob ein Ausgangskorb
  später kommt, ist offen; er bringt Reihenfolge- und Doppelschreibfragen mit.
- **Anhängen bei mehreren Accounts:** Der Hub serialisiert. Für die lokale Ablage steht
  dasselbe als Task 078 (`append`) bereit; beide sollten denselben Vertrag haben.
- **Dopplungen:** Zwei Collections oder zwei Accounts legen dasselbe Wissen ab. Erkennt das
  der Hub, und was tut er dann?
- **Rückfragen an den Menschen:** Wer sieht die Warteschlange des Arbeitsprozesses, und was
  geschieht mit einem Schnipsel, den wochenlang niemand beantwortet?
- **KI im Hub:** getrennt zu entscheiden. Kosten, Anbieterbindung und eine neue
  Fehlerquelle in einem Dienst, der sonst nur verwaltet.
- **Protokoll der Entscheidungen:** wo es liegt, wer es durchsieht, und was beim Widerspruch
  geschieht.
