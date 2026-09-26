package upgrade

import (
	"context"
	"sync"
	"time"
)

// Die Abstände der Prüfung im Hintergrund: GitHub erlaubt ohne Anmeldung 60
// Anfragen je Stunde und Adresse, geteilt von allen Usern eines Rechners.
const (
	// CheckInterval ist der Abstand nach einer gelungenen Prüfung.
	CheckInterval = 24 * time.Hour
	// RetryInterval ist der Abstand nach einer gescheiterten.
	RetryInterval = time.Hour
	// CheckTimeout begrenzt eine einzelne Prüfung.
	CheckTimeout = time.Minute
)

// Watcher fragt im Hintergrund nach dem neuesten Release — beim Start, danach
// höchstens einmal je CheckInterval, nach einem Fehler frühestens nach
// RetryInterval — und hält die letzte Antwort im Speicher. Report liefert sie
// ohne Anfrage an GitHub; so beantwortet serve whoami beliebig oft.
type Watcher struct {
	check func(ctx context.Context) Report
	// Interval, Retry und Timeout sind für Tests überschreibbar; NewWatcher
	// setzt die Werte oben.
	Interval, Retry, Timeout time.Duration
	// After liefert den Kanal, der nach d feuert; Tests steuern damit die Zeit.
	After func(d time.Duration) <-chan time.Time
	// Changed wird nach einer Prüfung aufgerufen, wenn sich Zustand, neueste
	// Version oder Fehler geändert haben — fürs Log; darf nil sein.
	Changed func(Report)

	mu   sync.Mutex
	last Report
}

// NewWatcher liefert einen Watcher, der mit check fragt; bis zur ersten
// Antwort liefert Report initial (State unchecked).
func NewWatcher(check func(ctx context.Context) Report, initial Report) *Watcher {
	return &Watcher{check: check, Interval: CheckInterval, Retry: RetryInterval, Timeout: CheckTimeout,
		After: time.After, last: initial}
}

// Report liefert die letzte Antwort.
func (w *Watcher) Report() Report {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.last
}

// Run fragt, bis ctx endet. Eine Prüfung, die durch das Ende von ctx
// abbricht, zählt nicht.
func (w *Watcher) Run(ctx context.Context) {
	for {
		cctx, cancel := context.WithTimeout(ctx, w.Timeout)
		r := w.check(cctx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		w.mu.Lock()
		prev := w.last
		w.last = r
		w.mu.Unlock()
		if w.Changed != nil && (prev.State != r.State || prev.Latest != r.Latest || prev.Error != r.Error) {
			w.Changed(r)
		}
		wait := w.Interval
		if r.State != StateOK {
			wait = w.Retry
		}
		select {
		case <-ctx.Done():
			return
		case <-w.After(wait):
		}
	}
}
