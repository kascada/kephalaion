# data

Maschinendateien, die k-playbook selbst besitzt und die zum Projektstand gehören.
Sie werden ganz normal mitversioniert; dieses Verzeichnis bekommt keine .gitignore.

Erste Datei ist todos.json mit den Todos des Projekts. Geschrieben wird sie über
/k-todo, über das Subkommando `k-playbook todo` oder über die Oberfläche — nie von
Hand. Einzige Ausnahme ist die Auflösung eines Merge-Konflikts: treffen zwei Branches
aufeinander, die je ein Todo angelegt haben, kollidieren sie an nextId und am Ende des
Arrays. Die Auflösung ist mechanisch — beide Einträge behalten, nextId auf max(id)+1
setzen.

Dieses Verzeichnis gehört dem Projekt und wird von einem Update nie angefasst.
