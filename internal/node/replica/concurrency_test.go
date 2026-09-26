package replica

// Nebenläufigkeit: node sync, serve, node hub rm|add und config import laufen
// in eigenen Prozessen neben- und gegeneinander. Die Tests stellen die
// Prozesse durch getrennte Syncer über getrennt geöffnete node.db nach; die
// Abfolge steuert gateHub.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kephalaion/kephalaion/internal/config"
	"github.com/kephalaion/kephalaion/internal/contract"
	"github.com/kephalaion/kephalaion/internal/node/store"
)

// gateHub reicht Sync an inner durch, einen Aufruf nach dem anderen. Den
// Aufruf mit der Nummer holdAt (ab 1) hält es an, bis release geschlossen
// wird; entered meldet, dass er angekommen ist.
type gateHub struct {
	contract.Hub
	mu      *sync.Mutex
	calls   int
	holdAt  int
	entered chan struct{}
	release chan struct{}
}

func newGate(inner contract.Hub, mu *sync.Mutex, holdAt int) *gateHub {
	return &gateHub{Hub: inner, mu: mu, holdAt: holdAt, entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *gateHub) Sync(ctx context.Context, req contract.SyncRequest) (contract.SyncResponse, error) {
	g.mu.Lock()
	g.calls++
	n := g.calls
	g.mu.Unlock()
	if n == g.holdAt {
		close(g.entered)
		select {
		case <-g.release:
		case <-ctx.Done():
			return contract.SyncResponse{}, ctx.Err()
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.Hub.Sync(ctx, req)
}

// otherProcess öffnet node.db ein zweites Mal, wie ein zweiter Prozess.
func (e *env) otherProcess() store.Store {
	e.t.Helper()
	nodeDB := filepath.Join(filepath.Dir(filepath.Dir(e.nodes.ReplicaPath("privat"))), "node.db")
	s, err := store.Open(context.Background(), config.SQLiteDB(nodeDB))
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = s.Close() })
	return s
}

func (e *env) entry() store.Hub {
	e.t.Helper()
	h, err := e.nodes.Hub(context.Background(), "privat")
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

// Zwei Abgleiche derselben Replica gleichzeitig, in kleinen Seiten, während
// der Hub schreibt: keiner scheitert, keiner schreibt einen Stand zurück,
// und danach stimmt die Replica.
func TestConcurrentSyncSameReplica(t *testing.T) {
	e := newEnv(t, "a", "b")
	for i := range 20 {
		e.hub.put([]string{"a", "b"}[i%2], fmt.Sprintf("d%02d.md", i), "x")
	}
	var mu sync.Mutex
	hub := newGate(e.hub, &mu, 0)
	syncers := []*Syncer{{Nodes: e.nodes, PageSize: 3}, {Nodes: e.otherProcess(), PageSize: 2}}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for round := range 5 {
		for i, s := range syncers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res := s.SyncEntry(context.Background(), e.entry(), func(store.Hub) (contract.Hub, error) { return hub, nil })
				if res.Err != nil {
					errs <- fmt.Errorf("Runde %d, Syncer %d: %w", round, i, res.Err)
				}
			}()
		}
		mu.Lock()
		e.hub.put("a", fmt.Sprintf("neu%d.md", round), "y")
		mu.Unlock()
		wg.Wait()
	}
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	e.ok()
	e.checkMirror("a", "b")
	if st := e.states(); st["a"] != e.hub.rev || st["b"] != e.hub.rev {
		t.Errorf("Stände %v, Hub bei %d", st, e.hub.rev)
	}
}

// Ein Abgleich leert die Replica (hub_id gewechselt), während ein anderer
// mitten darin steckt: Der andere schreibt seine Seite nicht auf die geleerte
// Replica, sondern setzt neu auf; danach stimmt alles.
func TestConcurrentReset(t *testing.T) {
	e := newEnv(t, "a")
	for i := range 6 {
		e.hub.put("a", fmt.Sprintf("d%d.md", i), "x")
	}
	e.ok()
	e.hub.put("a", "spaet1.md", "y")
	e.hub.put("a", "spaet2.md", "y")
	var mu sync.Mutex
	gate := newGate(e.hub, &mu, 2)
	slow := &Syncer{Nodes: e.nodes, PageSize: 1}
	done := make(chan HubResult, 1)
	go func() {
		done <- slow.SyncEntry(context.Background(), e.entry(), func(store.Hub) (contract.Hub, error) { return gate, nil })
	}()
	<-gate.entered
	// Der Hub wird neu aufgesetzt, mit anderen Dokumenten.
	mu.Lock()
	e.hub.id = newFakeHub("").id
	e.hub.docs = map[string]contract.Row{}
	e.hub.rev = 0
	e.hub.put("a", "anders.md", "z")
	mu.Unlock()
	other := &Syncer{Nodes: e.otherProcess()}
	if res := other.SyncEntry(context.Background(), e.entry(),
		func(store.Hub) (contract.Hub, error) { return newGate(e.hub, &mu, 0), nil }); res.Err != nil || res.Reset == "" {
		t.Fatalf("zweiter Abgleich: %+v", res)
	}
	close(gate.release)
	if res := <-done; res.Err != nil {
		t.Errorf("erster Abgleich: %v", res.Err)
	}
	e.checkMirror("a")
	if st := e.states(); st["a"] != e.hub.rev {
		t.Errorf("Stand %v, Hub bei %d", st, e.hub.rev)
	}
}

// node hub rm und add unter demselben Alias, während ein Abgleich des alten
// Eintrags auf den Hub wartet, bevor es eine Replica gibt: Der alte schreibt
// weder hub_id noch Stand in den neuen Eintrag; seine Replica verwirft der
// nächste Abgleich des neuen.
func TestRemoveAddBeforeFirstPage(t *testing.T) {
	e := newEnv(t, "a")
	ctx := context.Background()
	e.hub.put("a", "x.md", "x")
	old := e.entry()
	var mu sync.Mutex
	gate := newGate(e.hub, &mu, 1)
	done := make(chan HubResult, 1)
	go func() {
		done <- (&Syncer{Nodes: e.nodes}).SyncEntry(ctx, old, func(store.Hub) (contract.Hub, error) { return gate, nil })
	}()
	<-gate.entered

	cli := e.otherProcess()
	if err := cli.RemoveHub(ctx, "privat"); err != nil {
		t.Fatal(err)
	}
	if err := cli.AddHub(ctx, store.Hub{Name: "privat", NodeName: "laptop", Transport: store.TransportHTTPS,
		Address: "https://hub.example.org", Token: e.hub.token}, false); err != nil {
		t.Fatal(err)
	}
	e.want("a")
	close(gate.release)
	res := <-done
	if !res.Gone {
		t.Errorf("alter Abgleich: %+v", res)
	}
	cur := e.entry()
	if cur.EntryID == old.EntryID || cur.HubID != "" {
		t.Errorf("neuer Eintrag: %+v", cur)
	}
	if st, err := e.nodes.SyncStatus(ctx); err != nil || len(st) != 0 {
		t.Errorf("hub_sync: %+v, %v", st, err)
	}
	// Der neue Eintrag verwirft die Replica des alten und gleicht neu ab.
	res = e.ok()
	if res.Reset == "" {
		t.Errorf("kein Reset für die Replica des alten Eintrags: %+v", res)
	}
	if r := e.replica(); r.EntryID() != cur.EntryID {
		t.Errorf("Replica gehört zu %s, erwartet %s", r.EntryID(), cur.EntryID)
	}
	e.checkMirror("a")
}

// Dasselbe, während der alte Abgleich mitten in den Seiten steckt und der
// neue Eintrag schon seine eigene Replica hat: Der alte schreibt nichts
// hinein — weder Zeilen noch Stand noch hub_sync.
func TestRemoveAddDuringPages(t *testing.T) {
	e := newEnv(t, "a")
	ctx := context.Background()
	for i := range 4 {
		e.hub.put("a", fmt.Sprintf("d%d.md", i), "x")
	}
	old := e.entry()
	var mu sync.Mutex
	gate := newGate(e.hub, &mu, 2)
	oldSyncer := &Syncer{Nodes: e.nodes, PageSize: 1, now: func() int64 { return 999 }}
	done := make(chan HubResult, 1)
	go func() {
		done <- oldSyncer.SyncEntry(ctx, old, func(store.Hub) (contract.Hub, error) { return gate, nil })
	}()
	<-gate.entered

	cli := e.otherProcess()
	if err := cli.RemoveHub(ctx, "privat"); err != nil {
		t.Fatal(err)
	}
	if err := cli.AddHub(ctx, store.Hub{Name: "privat", NodeName: "laptop", Transport: store.TransportHTTPS,
		Address: "https://hub.example.org", Token: e.hub.token}, false); err != nil {
		t.Fatal(err)
	}
	if err := cli.AddCollection(ctx, "privat", "a"); err != nil {
		t.Fatal(err)
	}
	// Der neue Eintrag will nur, was der Hub unter spaet.md schreibt, nicht
	// mehr: Die Seiten des alten Abgleichs dürfen nicht hinein.
	mu.Lock()
	e.hub.put("a", "spaet.md", "y")
	mu.Unlock()
	newSyncer := &Syncer{Nodes: cli, now: func() int64 { return 111 }}
	if res := newSyncer.SyncEntry(ctx, e.entry(), func(store.Hub) (contract.Hub, error) {
		return newGate(e.hub, &mu, 0), nil
	}); res.Err != nil {
		t.Fatal(res.Err)
	}
	before := e.allRows()
	statesBefore := e.states()

	close(gate.release)
	res := <-done
	if !res.Gone {
		t.Errorf("alter Abgleich: %+v", res)
	}
	if got := e.allRows(); len(got) != len(before) {
		t.Errorf("Zeilen %d, vorher %d", len(got), len(before))
	}
	if got := e.states(); fmt.Sprint(got) != fmt.Sprint(statesBefore) {
		t.Errorf("Stände %v, vorher %v", got, statesBefore)
	}
	st, err := e.nodes.SyncStatus(ctx)
	if err != nil || st["privat"].OKAt != 111 {
		t.Errorf("hub_sync: %+v, %v", st, err)
	}
	e.checkMirror("a")
}

// Ein Abgleich, der abgebrochen wird (serve endet), hält nichts fest.
func TestCanceledSyncNotRecorded(t *testing.T) {
	e := newEnv(t, "a")
	e.hub.put("a", "x.md", "x")
	var mu sync.Mutex
	gate := newGate(e.hub, &mu, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan HubResult, 1)
	go func() {
		done <- e.sync.SyncEntry(ctx, e.entry(), func(store.Hub) (contract.Hub, error) { return gate, nil })
	}()
	<-gate.entered
	cancel()
	res := <-done
	if !errors.Is(res.Err, context.Canceled) {
		t.Errorf("Ergebnis: %+v", res)
	}
	if st, _ := e.nodes.SyncStatus(context.Background()); len(st) != 0 {
		t.Errorf("hub_sync nach Abbruch: %+v", st)
	}
}

// Art des Fehlers und Festhalten: Erfolg, dann Fehler des Hubs mit Art.
func TestSyncEntryRecordsKind(t *testing.T) {
	e := newEnv(t, "a")
	ctx := context.Background()
	e.hub.put("a", "x.md", "x")
	e.ok()
	e.hub.token = "keph_anders"
	res := e.run()
	if res.Kind != KindUnauthenticated {
		t.Errorf("Art %q", res.Kind)
	}
	st, _ := e.nodes.SyncStatus(ctx)
	if st["privat"].ErrKind != string(KindUnauthenticated) || st["privat"].OKAt == 0 {
		t.Errorf("hub_sync: %+v", st)
	}
	e.hub.failAt = e.hub.calls + 1
	if res := e.run(); res.Kind != KindHub {
		t.Errorf("Transportfehler der Attrappe: %q", res.Kind)
	}
	res = (&Syncer{Nodes: e.nodes}).SyncEntry(ctx, e.entry(), func(store.Hub) (contract.Hub, error) {
		return nil, errors.New("Transport https wird noch nicht unterstützt")
	})
	if res.Kind != KindConnect {
		t.Errorf("connect: %q", res.Kind)
	}
	for _, k := range []ErrorKind{KindConnect, KindUnreachable, KindUnauthenticated, KindVersion, KindHub, KindProtocol,
		KindReplica} {
		if k.Text() == "Abgleich gescheitert" {
			t.Errorf("ohne Text: %s", k)
		}
	}
}
