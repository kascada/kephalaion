package replica

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/oklog/ulid/v2"

	"github.com/kascada/kephalaion/internal/contract"
	"github.com/kascada/kephalaion/internal/node/store"
	"github.com/kascada/kephalaion/internal/sqlitedb"
)

// Connect liefert zu einem Hub-Eintrag die Umsetzung des Vertrags, über die
// der Node ihn erreicht — bei local den Hub der eigenen config. Ein Fehler
// (Transport noch nicht unterstützt, kein Hub in der config) betrifft nur
// diesen Eintrag; die übrigen laufen weiter. Die Umsetzung wählt
// cmd/kephalaion: Der Node kennt den Hub nur über den Vertrag.
type Connect func(h store.Hub) (contract.Hub, error)

// Syncer gleicht die Replicas eines Nodes ab.
type Syncer struct {
	// Nodes ist die Datenbank des Nodes: Hub-Einträge, gewünschte
	// Collections, Ort der Replicas.
	Nodes store.Store
	// PageSize ist die Seitengröße der Anfragen; 0 heißt
	// contract.DefaultPageSize. Keine Nutzeroption, Tests stellen sie
	// kleiner.
	PageSize int
	// now liefert die Zeit für synced_at; nil heißt sqlitedb.NowMillis.
	now func() int64
}

// Status sagt, was der Abgleich mit einer Collection getan hat.
type Status int

// Mögliche Ergebnisse je Collection.
const (
	// Synced: abgeglichen; Rows Zeilen kamen an, der Stand ist Revision.
	Synced Status = iota
	// NotAllowed: der Hub erlaubt sie nicht (mehr), oder er kennt sie
	// nicht; aus der Replica entfernt.
	NotAllowed
	// NotWanted: der Node will sie nicht mehr; aus der Replica entfernt.
	NotWanted
)

func (s Status) String() string {
	switch s {
	case NotAllowed:
		return "nicht erlaubt"
	case NotWanted:
		return "nicht mehr gewünscht"
	}
	return "abgeglichen"
}

// CollectionResult ist das Ergebnis des Abgleichs für eine Collection.
type CollectionResult struct {
	Collection string
	Status     Status
	// Rows zählt die angekommenen Zeilen (Synced), Löschmarken und
	// SYSTEM:-Zeilen eingeschlossen.
	Rows int
	// Revision ist der Stand nach dem Abgleich (Synced).
	Revision int64
	// Removed zählt die Zeilen, die aus der Replica entfernt wurden
	// (NotAllowed, NotWanted).
	Removed int64
}

// HubResult ist das Ergebnis des Abgleichs für einen Hub-Eintrag. Err ist
// gesetzt, wenn der Abgleich dieses Eintrags scheiterte; was bis dahin
// ankam, ist geschrieben und steht in Collections. Fehler des Vertrags
// (contract.ErrUnauthenticated usw.) lassen sich mit errors.Is erkennen.
type HubResult struct {
	// Hub ist der Alias des Eintrags.
	Hub string
	// HubID ist die hub_id des Hubs, sobald er geantwortet hat.
	HubID string
	// Reset nennt den Grund, wenn die Replica geleert und von vorn
	// abgeglichen wurde, sonst leer.
	Reset string
	// Allowed sind alle Collections, die der Hub diesem Node erlaubt, auch
	// nicht gewünschte.
	Allowed []string
	// Pages zählt die angewandten Seiten.
	Pages int
	// Collections nennt erst die gewünschten Collections in der Reihenfolge
	// des Eintrags, dann die nicht mehr gewünschten, die entfernt wurden.
	Collections []CollectionResult
	Err         error
}

// Sync gleicht alle Hub-Einträge ab, oder nur den mit dem Alias only. Der
// Fehler betrifft nur das Lesen der Einträge (oder einen unbekannten
// Alias); was je Eintrag scheitert, steht in HubResult.Err.
func (s *Syncer) Sync(ctx context.Context, only string, connect Connect) ([]HubResult, error) {
	var hubs []store.Hub
	if only != "" {
		h, err := s.Nodes.Hub(ctx, only)
		if err != nil {
			return nil, err
		}
		hubs = []store.Hub{h}
	} else {
		var err error
		if hubs, err = s.Nodes.Hubs(ctx); err != nil {
			return nil, err
		}
	}
	out := make([]HubResult, 0, len(hubs))
	for _, h := range hubs {
		hub, err := connect(h)
		if err != nil {
			out = append(out, HubResult{Hub: h.Name, HubID: h.HubID, Err: err})
			continue
		}
		out = append(out, s.SyncHub(ctx, h, hub))
	}
	return out, nil
}

// openForSync öffnet die Replica eines Eintrags. Fehlt sie, ist sie nil. Ist
// ihre Schemafassung eine andere, wird sie verworfen — sie ist abgeleitet —,
// und reason sagt es.
func openForSync(ctx context.Context, path string) (r *Replica, reason string, err error) {
	r, err = Open(ctx, path)
	var sv *sqlitedb.SchemaVersionError
	switch {
	case err == nil:
		return r, "", nil
	case errors.Is(err, sqlitedb.ErrNotFound):
		return nil, "", nil
	case errors.As(err, &sv):
		if err := Remove(path); err != nil {
			return nil, "", fmt.Errorf("Replica %s verwerfen: %w", path, err)
		}
		return nil, fmt.Sprintf("Schemafassung %s der Replica passt nicht zu diesem Binary (erwartet %s); neu angelegt",
			sv.Got, sv.Want), nil
	}
	return nil, "", err
}

// checkResponse prüft eine Antwort gegen ihre Anfrage, soweit der Node es
// kann, bevor er etwas schreibt.
func checkResponse(req contract.SyncRequest, resp contract.SyncResponse) error {
	if resp.Version != contract.Version {
		return fmt.Errorf("der Hub antwortet in Fassung %d, erwartet %d", resp.Version, contract.Version)
	}
	if _, err := ulid.ParseStrict(resp.HubID); err != nil {
		return fmt.Errorf("der Hub nennt als hub_id %q, keine ULID", resp.HubID)
	}
	if len(resp.Collections) != len(req.Collections) {
		return fmt.Errorf("der Hub nennt %d Collections, angefragt waren %d", len(resp.Collections), len(req.Collections))
	}
	for i, c := range resp.Collections {
		if c.Collection != req.Collections[i].Collection {
			return fmt.Errorf("der Hub nennt Collection %q, angefragt war %q", c.Collection, req.Collections[i].Collection)
		}
	}
	return nil
}

// SyncHub gleicht einen Hub-Eintrag über hub ab (docs/vertrag.md, „Regeln
// für den Node“):
//
//   - Nicht mehr gewünschte Collections verlassen die Replica zuerst.
//   - Je gewünschter Collection fragt der Node ab ihrem Stand (neu: 0).
//   - Die erste Antwort legt die Replica an, wenn es sie nicht gibt. Nennt
//     sie eine andere hub_id als db_info der Replica, oder liegt ein Stand
//     über der Revision des Hubs (Hub aus einer Sicherung), wird die Replica
//     geleert und von vorn abgeglichen — höchstens einmal je Aufruf.
//   - Danach schreibt der Node die Kopie der hub_id in node.db.
//   - Jede Seite ist eine Transaktion: nicht erlaubte Collections entfernen,
//     Zeilen per id einfügen oder ersetzen, je Collection den Stand auf
//     max(Stand, until). Dann die nächste Seite, solange more gilt.
func (s *Syncer) SyncHub(ctx context.Context, h store.Hub, hub contract.Hub) HubResult {
	res := HubResult{Hub: h.Name, HubID: h.HubID}
	if err := s.syncHub(ctx, h, hub, &res); err != nil {
		res.Err = err
	}
	return res
}

// progress hält fest, was der Abgleich eines Eintrags je Collection getan
// hat.
type progress struct {
	order  []string
	byName map[string]*CollectionResult
}

func (p *progress) get(c string, st Status) *CollectionResult {
	if r, ok := p.byName[c]; ok {
		r.Status = st
		return r
	}
	r := &CollectionResult{Collection: c, Status: st}
	p.byName[c] = r
	p.order = append(p.order, c)
	return r
}

func (p *progress) results() []CollectionResult {
	out := make([]CollectionResult, 0, len(p.order))
	for _, c := range p.order {
		out = append(out, *p.byName[c])
	}
	return out
}

func (s *Syncer) syncHub(ctx context.Context, h store.Hub, hub contract.Hub, res *HubResult) error {
	pageSize := s.PageSize
	if pageSize <= 0 {
		pageSize = contract.DefaultPageSize
	}
	now := s.now
	if now == nil {
		now = sqlitedb.NowMillis
	}
	prog := &progress{byName: map[string]*CollectionResult{}}
	for _, c := range h.Collections {
		prog.get(c, Synced)
	}
	defer func() { res.Collections = prog.results() }()

	path := s.Nodes.ReplicaPath(h.Name)
	rep, reason, err := openForSync(ctx, path)
	if err != nil {
		return err
	}
	res.Reset = reason
	defer func() {
		if rep != nil {
			_ = rep.Close()
		}
	}()

	state := map[string]int64{}
	if rep != nil {
		if err := s.dropUnwanted(ctx, rep, h.Collections, prog, now()); err != nil {
			return err
		}
		states, err := rep.States(ctx)
		if err != nil {
			return err
		}
		for _, st := range states {
			state[st.Collection] = st.Revision
		}
	}

	pending := slices.Clone(h.Collections)
	reset := false
	for {
		req := contract.SyncRequest{
			Version:     contract.Version,
			Auth:        contract.NodeAuth{Node: h.NodeName, Token: h.Token},
			Collections: make([]contract.Since, 0, len(pending)),
			PageSize:    pageSize,
		}
		for _, c := range pending {
			req.Collections = append(req.Collections, contract.Since{Collection: c, Since: state[c]})
		}
		resp, err := hub.Sync(ctx, req)
		if err != nil {
			return err
		}
		if err := checkResponse(req, resp); err != nil {
			return err
		}

		if rep == nil {
			if rep, err = Create(ctx, path, resp.HubID); err != nil {
				return err
			}
		} else if why := mismatch(rep, state, resp); why != "" {
			// Die Seite gehört zu Ständen, die nicht mehr gelten: verwerfen,
			// Replica leeren, von vorn fragen.
			if reset {
				return fmt.Errorf("der Hub wechselt während des Abgleichs erneut: %s", why)
			}
			if err := rep.reset(ctx, resp.HubID); err != nil {
				return err
			}
			reset = true
			res.Reset = why
			state = map[string]int64{}
			for _, r := range prog.byName {
				if r.Status == Synced {
					r.Rows, r.Revision = 0, 0
				}
			}
			continue
		}
		if h.HubID != resp.HubID {
			// Die Kopie folgt der maßgeblichen hub_id der Replica.
			if err := s.Nodes.SetHubID(ctx, h.Name, resp.HubID); err != nil {
				return err
			}
			h.HubID = resp.HubID
		}
		res.HubID = resp.HubID
		res.Allowed = resp.Allowed

		p := page{rows: resp.Rows, advance: map[string]int64{}, until: resp.Until, now: now()}
		next := make([]string, 0, len(pending))
		for _, cs := range resp.Collections {
			if cs.Allowed {
				p.advance[cs.Collection] = state[cs.Collection]
				next = append(next, cs.Collection)
			} else {
				p.drop = append(p.drop, cs.Collection)
			}
		}
		for _, row := range resp.Rows {
			if _, ok := p.advance[row.Collection]; !ok {
				return fmt.Errorf("der Hub liefert eine Zeile aus Collection %q, die nicht angefragt oder nicht erlaubt ist",
					row.Collection)
			}
		}
		minSince := int64(-1)
		for _, c := range next {
			if minSince < 0 || state[c] < minSince {
				minSince = state[c]
			}
		}
		drops, err := rep.apply(ctx, p)
		if err != nil {
			return err
		}
		res.Pages++
		for c, d := range drops {
			prog.get(c, NotAllowed).Removed += d.rows
		}
		for _, row := range resp.Rows {
			prog.get(row.Collection, Synced).Rows++
		}
		for c := range p.advance {
			state[c] = max(state[c], resp.Until)
			prog.get(c, Synced).Revision = state[c]
		}
		pending = next
		if !resp.More || len(pending) == 0 {
			return nil
		}
		// Mit more muss until über dem kleinsten Stand liegen, sonst fragte
		// der Node ewig dasselbe.
		if resp.Until <= minSince {
			return fmt.Errorf("der Hub meldet weitere Zeilen, kommt aber nicht über Revision %d hinaus", resp.Until)
		}
	}
}

// mismatch sagt, warum die Replica nicht zur Antwort passt: andere hub_id,
// oder ein Stand über der Revision des Hubs. Leer heißt: passt.
func mismatch(rep *Replica, state map[string]int64, resp contract.SyncResponse) string {
	if rep.HubID() != resp.HubID {
		return fmt.Sprintf("hub_id gewechselt (%s → %s); Replica geleert und von vorn abgeglichen", rep.HubID(), resp.HubID)
	}
	var top int64
	for _, rev := range state {
		top = max(top, rev)
	}
	if top > resp.HubRevision {
		return fmt.Sprintf("Stand %d der Replica liegt über der Revision %d des Hubs (aus einer Sicherung "+
			"zurückgespielt?); Replica geleert und von vorn abgeglichen", top, resp.HubRevision)
	}
	return ""
}

// dropUnwanted entfernt aus der Replica, was der Node nicht mehr will, in
// einer Transaktion.
func (s *Syncer) dropUnwanted(ctx context.Context, rep *Replica, wanted []string, prog *progress, now int64) error {
	have, err := rep.Collections(ctx)
	if err != nil {
		return err
	}
	var drop []string
	for _, c := range have {
		if !slices.Contains(wanted, c) {
			drop = append(drop, c)
		}
	}
	if len(drop) == 0 {
		return nil
	}
	drops, err := rep.apply(ctx, page{drop: drop, now: now})
	if err != nil {
		return err
	}
	for _, c := range drop {
		prog.get(c, NotWanted).Removed = drops[c].rows
	}
	return nil
}
