// Die Trennregel: Hub und Node bleiben im Code getrennt. Kein Paket unter
// internal/hub importiert — auch nicht über Umwege oder in Tests — eines unter
// internal/node, und umgekehrt. Gemeinsames steht in neutralen Paketen wie
// internal/sqlitedb.
//
// Der Test liest die Importe selbst aus den Quelldateien statt `go list`
// aufzurufen: So kennt der Testcache die gelesenen Dateien, und ein neuer
// Import macht ein zwischengespeichertes Ergebnis ungültig.
package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// modulePath liest den Modulpfad aus go.mod im Verzeichnis darüber.
func modulePath(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			return f[1]
		}
	}
	t.Fatal("go.mod ohne module-Zeile")
	return ""
}

// importGraph liefert je Paket unter root (Importpfad) die Importe aller
// seiner .go-Dateien, Tests eingeschlossen.
func importGraph(t *testing.T, root, importRoot string) map[string][]string {
	t.Helper()
	graph := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(p))
		if err != nil {
			return err
		}
		pkg := path.Join(importRoot, filepath.ToSlash(rel))
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if _, ok := graph[pkg]; !ok {
			graph[pkg] = nil
		}
		for _, imp := range f.Imports {
			ip, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			graph[pkg] = append(graph[pkg], ip)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// under sagt, ob pkg gleich prefix ist oder darunter liegt.
func under(pkg, prefix string) bool {
	return pkg == prefix || strings.HasPrefix(pkg, prefix+"/")
}

// violations liefert die Pakete unter forbidden, die start direkt oder über
// Umwege importiert.
func violations(graph map[string][]string, start, forbidden string) []string {
	seen := map[string]bool{start: true}
	queue := []string{start}
	var bad []string
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, imp := range graph[p] {
			if seen[imp] {
				continue
			}
			seen[imp] = true
			if under(imp, forbidden) {
				bad = append(bad, imp)
			}
			queue = append(queue, imp)
		}
	}
	sort.Strings(bad)
	return bad
}

func TestHubAndNodeSeparated(t *testing.T) {
	module := modulePath(t)
	// Das Arbeitsverzeichnis des Tests ist internal/.
	graph := importGraph(t, ".", module+"/internal")
	hub := module + "/internal/hub"
	node := module + "/internal/node"
	checked := 0
	for pkg := range graph {
		var forbidden string
		switch {
		case under(pkg, hub):
			forbidden = node
		case under(pkg, node):
			forbidden = hub
		default:
			continue
		}
		checked++
		for _, v := range violations(graph, pkg, forbidden) {
			t.Errorf("%s importiert %s — Hub und Node bleiben getrennt", pkg, v)
		}
	}
	if checked < 2 {
		t.Fatalf("nur %d Pakete von Hub und Node gefunden", checked)
	}
}

func TestViolations(t *testing.T) {
	graph := map[string][]string{
		"m/internal/hub/store": {"m/internal/sqlitedb", "m/internal/hubx"},
		"m/internal/sqlitedb":  {"m/internal/node/store"},
	}
	got := violations(graph, "m/internal/hub/store", "m/internal/node")
	if len(got) != 1 || got[0] != "m/internal/node/store" {
		t.Errorf("violations = %v", got)
	}
	if got := violations(graph, "m/internal/node/store", "m/internal/hub"); len(got) != 0 {
		t.Errorf("violations = %v", got)
	}
}

// TestContractNeutral prüft, dass der Vertrag weder Hub noch Node kennt: Der
// Node benutzt ihn, ohne über ihn an den Hub zu kommen. Das gilt auch für
// seine Umsetzung über HTTP (internal/contract/httpapi) und das Log der
// Anfragen (internal/reqlog), die beide Rollen teilen.
func TestContractNeutral(t *testing.T) {
	module := modulePath(t)
	graph := importGraph(t, ".", module+"/internal")
	var neutral []string
	for pkg := range graph {
		if under(pkg, module+"/internal/contract") || under(pkg, module+"/internal/reqlog") {
			neutral = append(neutral, pkg)
		}
	}
	if len(neutral) < 3 {
		t.Fatalf("nur %v gefunden", neutral)
	}
	for _, pkg := range neutral {
		for _, forbidden := range []string{module + "/internal/hub", module + "/internal/node"} {
			for _, v := range violations(graph, pkg, forbidden) {
				t.Errorf("%s importiert %s — der Vertrag bleibt neutral", pkg, v)
			}
		}
	}
}
