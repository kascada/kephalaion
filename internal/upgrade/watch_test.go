package upgrade

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

// clock steuert die Zeit des Watchers: Jedes After meldet die verlangte Dauer
// und feuert erst, wenn der Test es sagt.
type clock struct {
	waits chan time.Duration
	fire  chan time.Time
}

func newClock() *clock {
	return &clock{waits: make(chan time.Duration, 10), fire: make(chan time.Time)}
}

func (c *clock) after(d time.Duration) <-chan time.Time {
	c.waits <- d
	return c.fire
}

func (c *clock) nextWait(t *testing.T) time.Duration {
	t.Helper()
	select {
	case d := <-c.waits:
		return d
	case <-time.After(5 * time.Second):
		t.Fatal("der Watcher wartet nicht")
	}
	return 0
}

// Beim Start eine Frage an GitHub, danach die Antwort im Speicher; die
// nächste erst nach einem Tag, nach einem Fehler nach einer Stunde.
func TestWatcherTiming(t *testing.T) {
	f := newFakeGitHub(t)
	f.addRelease("v0.2.0", []byte("zwei"))
	f.latest = "v0.2.0"
	fail := false
	var mu sync.Mutex
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return true
		}
		return false
	}
	u, _ := f.upgrader("v0.1.0", installed(t))
	u.Now = func() time.Time { return fixedNow }
	c := newClock()
	w := NewWatcher(u.Report, u.Local())
	w.After = c.after
	var changes []Report
	var cmu sync.Mutex
	w.Changed = func(r Report) {
		cmu.Lock()
		changes = append(changes, r)
		cmu.Unlock()
	}
	if r := w.Report(); r.State != StateUnchecked || f.requests.Load() != 0 {
		t.Fatalf("vor dem Start: %+v, %d Anfragen", r, f.requests.Load())
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()

	if d := c.nextWait(t); d != 24*time.Hour {
		t.Errorf("nach Erfolg %s, erwartet 24h", d)
	}
	if r := w.Report(); r.State != StateOK || r.Latest != "v0.2.0" || !r.UpdateAvailable {
		t.Errorf("nach der ersten Prüfung: %+v", r)
	}
	// Beliebig viele Abfragen, keine weitere Anfrage.
	for i := 0; i < 100; i++ {
		w.Report()
	}
	if n := f.requests.Load(); n != 1 {
		t.Errorf("%d Anfragen, erwartet 1", n)
	}

	// Nach einem Tag: GitHub scheitert — Fehler im Speicher, nächste Frage
	// nach einer Stunde.
	mu.Lock()
	fail = true
	mu.Unlock()
	c.fire <- time.Now()
	if d := c.nextWait(t); d != time.Hour {
		t.Errorf("nach Fehler %s, erwartet 1h", d)
	}
	if r := w.Report(); r.State != StateFailed || r.Latest != "" || r.Error == "" {
		t.Errorf("nach Fehler: %+v", r)
	}
	mu.Lock()
	fail = false
	mu.Unlock()
	c.fire <- time.Now()
	if d := c.nextWait(t); d != 24*time.Hour {
		t.Errorf("nach Erholung %s, erwartet 24h", d)
	}
	if n := f.requests.Load(); n != 3 {
		t.Errorf("%d Anfragen, erwartet 3", n)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run endet nicht mit ctx")
	}
	cmu.Lock()
	defer cmu.Unlock()
	if len(changes) != 3 || changes[0].State != StateOK || changes[1].State != StateFailed || changes[2].State != StateOK {
		t.Errorf("Änderungen: %+v", changes)
	}
}

// Eine Prüfung, die das Ende von serve abbricht, zählt nicht: Report bleibt,
// wie er war.
func TestWatcherCancel(t *testing.T) {
	started := make(chan struct{})
	w := NewWatcher(func(ctx context.Context) Report {
		close(started)
		<-ctx.Done()
		return Report{State: StateFailed, Error: "abgebrochen"}
	}, Report{State: StateUnchecked, Error: "noch nicht geprüft"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	<-started
	cancel()
	<-done
	if r := w.Report(); r.State != StateUnchecked {
		t.Errorf("nach Abbruch: %+v", r)
	}
}

// Eine hängende Prüfung endet nach Timeout und gilt als Fehler.
func TestWatcherTimeout(t *testing.T) {
	c := newClock()
	w := NewWatcher(func(ctx context.Context) Report {
		<-ctx.Done()
		return Report{State: StateFailed, Error: ctx.Err().Error()}
	}, Report{State: StateUnchecked})
	w.Timeout = 10 * time.Millisecond
	w.After = c.after
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	if d := c.nextWait(t); d != time.Hour {
		t.Errorf("nach Zeitüberschreitung %s, erwartet 1h", d)
	}
	if r := w.Report(); r.State != StateFailed {
		t.Errorf("Report %+v", r)
	}
}
