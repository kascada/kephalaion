package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/kascada/kephalaion/internal/config"
	hubstore "github.com/kascada/kephalaion/internal/hub/store"
	nodestore "github.com/kascada/kephalaion/internal/node/store"
)

// exportFormat ist die Fassung des Exportformats, die dieses Binary schreibt.
// Gelesen werden auch ältere Fassungen ab minExportFormat. Format 3 bringt
// hubs.node_name am Node; ein Hub-Eintrag ohne ihn scheitert beim Import an
// derselben Prüfung wie node hub add ohne --node.
const (
	exportFormat    = 3
	minExportFormat = 1
)

// exportFile ist der Inhalt einer Exportdatei: die config, die settings je
// Rolle und ab Format 2 die lokalen Tabellen je Rolle — keine Inhalte, kein
// db_info, kein Protokoll.
type exportFile struct {
	Format   int                               `yaml:"format"`
	Config   config.Config                     `yaml:"config"`
	Settings map[config.Role]map[string]string `yaml:"settings"`
	Tables   *exportTables                     `yaml:"tables,omitempty"`
}

// exportTables sind die lokalen Tabellen je Rolle; eine Rolle fehlt nur, wenn
// sie nicht eingerichtet ist.
type exportTables struct {
	Hub  *hubTablesYAML  `yaml:"hub,omitempty"`
	Node *nodeTablesYAML `yaml:"node,omitempty"`
}

type hubTablesYAML struct {
	Collections     []collectionYAML `yaml:"collections"`
	Nodes           []nodeYAML       `yaml:"nodes"`
	NodeCollections []grantYAML      `yaml:"node_collections"`
}

type collectionYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	CreatedAt   int64  `yaml:"created_at"`
	CreatedBy   string `yaml:"created_by"`
}

type nodeYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	TokenHash   string `yaml:"token_hash"`
	Locked      bool   `yaml:"locked"`
	CreatedAt   int64  `yaml:"created_at"`
	CreatedBy   string `yaml:"created_by"`
}

type grantYAML struct {
	Node       string `yaml:"node"`
	Collection string `yaml:"collection"`
}

type nodeTablesYAML struct {
	Hubs           []hubYAML    `yaml:"hubs"`
	HubCollections []wantedYAML `yaml:"hub_collections"`
}

type hubYAML struct {
	Name      string `yaml:"name"`
	NodeName  string `yaml:"node_name"`
	Transport string `yaml:"transport"`
	Address   string `yaml:"address"`
	Token     string `yaml:"token"`
	SSHKey    string `yaml:"ssh_key"`
	HubID     string `yaml:"hub_id"`
}

type wantedYAML struct {
	Hub        string `yaml:"hub"`
	Collection string `yaml:"collection"`
}

func hubTablesToYAML(t hubstore.Tables) *hubTablesYAML {
	out := &hubTablesYAML{
		Collections:     []collectionYAML{},
		Nodes:           []nodeYAML{},
		NodeCollections: []grantYAML{},
	}
	for _, c := range t.Collections {
		out.Collections = append(out.Collections, collectionYAML(c))
	}
	for _, n := range t.Nodes {
		out.Nodes = append(out.Nodes, nodeYAML{Name: n.Name, Description: n.Description, TokenHash: n.TokenHash,
			Locked: n.Locked, CreatedAt: n.CreatedAt, CreatedBy: n.CreatedBy})
	}
	for _, g := range t.Grants {
		out.NodeCollections = append(out.NodeCollections, grantYAML(g))
	}
	return out
}

func (y *hubTablesYAML) toStore() hubstore.Tables {
	t := hubstore.Tables{Collections: []hubstore.Collection{}, Nodes: []hubstore.Node{}, Grants: []hubstore.Grant{}}
	for _, c := range y.Collections {
		t.Collections = append(t.Collections, hubstore.Collection(c))
	}
	for _, n := range y.Nodes {
		t.Nodes = append(t.Nodes, hubstore.Node{Name: n.Name, Description: n.Description, TokenHash: n.TokenHash,
			Locked: n.Locked, CreatedAt: n.CreatedAt, CreatedBy: n.CreatedBy})
	}
	for _, g := range y.NodeCollections {
		t.Grants = append(t.Grants, hubstore.Grant(g))
	}
	return t
}

func nodeTablesToYAML(t nodestore.Tables) *nodeTablesYAML {
	out := &nodeTablesYAML{Hubs: []hubYAML{}, HubCollections: []wantedYAML{}}
	for _, h := range t.Hubs {
		out.Hubs = append(out.Hubs, hubYAML{Name: h.Name, NodeName: h.NodeName, Transport: h.Transport, Address: h.Address,
			Token: h.Token, SSHKey: h.SSHKey, HubID: h.HubID})
	}
	for _, w := range t.Wanted {
		out.HubCollections = append(out.HubCollections, wantedYAML(w))
	}
	return out
}

func (y *nodeTablesYAML) toStore() nodestore.Tables {
	t := nodestore.Tables{Hubs: []nodestore.Hub{}, Wanted: []nodestore.Wanted{}}
	for _, h := range y.Hubs {
		t.Hubs = append(t.Hubs, nodestore.Hub{Name: h.Name, NodeName: h.NodeName, Transport: h.Transport, Address: h.Address,
			Token: h.Token, SSHKey: h.SSHKey, HubID: h.HubID})
	}
	for _, w := range y.HubCollections {
		t.Wanted = append(t.Wanted, nodestore.Wanted(w))
	}
	return t
}

// roles liefert die Rollen eines Exports: die aus der config und die mit
// einem Teil.
func (e *exportFile) roles() ([]config.Role, error) {
	for r := range e.Settings {
		if r != config.Hub && r != config.Node {
			return nil, fmt.Errorf("unbekannte Rolle %q", r)
		}
	}
	var out []config.Role
	for _, r := range config.Roles {
		_, hasSettings := e.Settings[r]
		hasTables := e.Tables != nil && ((r == config.Hub && e.Tables.Hub != nil) || (r == config.Node && e.Tables.Node != nil))
		if e.Config.Section(r) != nil || hasSettings || hasTables {
			out = append(out, r)
		}
	}
	return out, nil
}

// tableKeys sind die Tabellen je Rolle, wie sie im Export heißen.
var tableKeys = map[config.Role][]string{
	config.Hub:  {"collections", "nodes", "node_collections"},
	config.Node: {"hubs", "hub_collections"},
}

// parseExport liest eine Exportdatei. Die Fassung wird zuerst geprüft, damit
// ein Export einer anderen Fassung klar abgelehnt wird, statt an unbekannten
// Feldern zu scheitern. Danach muss jede Rolle des Exports jeden Teil ihrer
// Fassung haben: Ein fehlender Teil oder null bricht ab — nur ein
// ausdrücklich leerer Teil ({} bzw. []) leert beim Import.
func parseExport(data []byte) (exportFile, error) {
	var head struct {
		Format int `yaml:"format"`
	}
	if err := yaml.Unmarshal(data, &head); err != nil {
		return exportFile{}, fmt.Errorf("kein gültiges YAML: %w", err)
	}
	if head.Format < minExportFormat || head.Format > exportFormat {
		if head.Format == 0 {
			return exportFile{}, errors.New("keine Fassung des Formats angegeben (format:) — kein Export von kephalaion?")
		}
		return exportFile{}, fmt.Errorf("unbekannte Fassung des Formats %d; dieses Binary kennt %d bis %d",
			head.Format, minExportFormat, exportFormat)
	}
	var exp exportFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&exp); err != nil {
		return exportFile{}, fmt.Errorf("Export nicht lesbar: %w", err)
	}
	if exp.Format < 2 && exp.Tables != nil {
		return exportFile{}, fmt.Errorf("Format %d kennt keine Tabellen (tables:)", exp.Format)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return exportFile{}, fmt.Errorf("kein gültiges YAML: %w", err)
	}
	roles, err := exp.roles()
	if err != nil {
		return exportFile{}, err
	}
	for _, r := range roles {
		if err := requirePart(&root, "settings", string(r)); err != nil {
			return exportFile{}, err
		}
		if exp.Format < 2 {
			continue
		}
		if err := requirePart(&root, "tables", string(r)); err != nil {
			return exportFile{}, err
		}
		for _, k := range tableKeys[r] {
			if err := requirePart(&root, "tables", string(r), k); err != nil {
				return exportFile{}, err
			}
		}
	}
	return exp, nil
}

// requirePart prüft, dass der Teil unter path im Export steht und nicht null
// ist.
func requirePart(root *yaml.Node, path ...string) error {
	name := strings.Join(path, ".")
	const hint = "; ein leerer Teil muss ausdrücklich dastehen ({} bzw. [])"
	n := root
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	for i, key := range path {
		if n.Kind != yaml.MappingNode {
			return errors.New(name + " fehlt" + hint)
		}
		var next *yaml.Node
		for j := 0; j+1 < len(n.Content); j += 2 {
			if n.Content[j].Value == key {
				next = n.Content[j+1]
				break
			}
		}
		if next == nil {
			return errors.New(name + " fehlt" + hint)
		}
		if next.Kind == yaml.AliasNode {
			next = next.Alias
		}
		if next.Kind == yaml.ScalarNode && next.ShortTag() == "!!null" {
			if i < len(path)-1 {
				return errors.New(name + " fehlt" + hint)
			}
			return errors.New(name + " ist null" + hint)
		}
		n = next
	}
	return nil
}
