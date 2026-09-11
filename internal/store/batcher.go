package store

import (
	"log"
	"sync"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// DefaultQuiet is the commit debounce (T8): a fleet's burst of events
// becomes one commit, while the TUI still feels live.
const DefaultQuiet = 2 * time.Second

// Batcher accumulates events and commits them as one batch after a quiet
// interval with no new additions. Flush commits eagerly — the decision
// to do so for claims and escalations (T8) belongs to the daemon; the
// Batcher only supplies the mechanism.
//
// Error reporting: a background (timer-driven) flush that fails keeps
// its events and files pending for the next attempt and logs the error
// at failure time — through Log when set, the standard logger
// otherwise — so a failed timer flush is never silent (Go-sweep audit
// finding, decided 2026-08-27: the earlier LastError accessor had no
// reader on the failure path, and a failure surfaced only if a later
// synchronous Flush happened to run). Callers wanting synchronous
// errors use Flush, which returns them directly.
//
// One mutex guards everything; the only concurrency is the time.Timer's
// callback, which takes the same mutex. AppendBatch runs while holding
// the lock — Add blocks during a flush, which is the point: writes to
// the branch stay serialized.
type Batcher struct {
	store *Store
	quiet time.Duration
	// Log, when set, gets one line per commit with its size and cost —
	// the daemon's evidence stream for what a write costs (001 D2 note).
	Log *log.Logger

	mu      sync.Mutex
	pending []event.Event
	files   map[string][]byte
	timer   *time.Timer
}

// NewBatcher returns a Batcher committing through s. quiet <= 0 means
// DefaultQuiet.
func NewBatcher(s *Store, quiet time.Duration) *Batcher {
	if quiet <= 0 {
		quiet = DefaultQuiet
	}
	return &Batcher{store: s, quiet: quiet}
}

// Add queues e and restarts the quiet-interval timer.
func (b *Batcher) Add(e event.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = append(b.pending, e)
	b.armLocked()
}

// SetFiles replaces the file blobs (views, T6) staged to ride the next
// commit, event or timer driven. Replaces, not merges: the daemon
// stages the full diff of a fresh render against the head tree on
// every state-version bump (T6, 2026-09-10), so whatever was staged
// before is stale by construction — a page rendered before a merge
// landed would otherwise ride a later commit and overwrite the merged
// one. An empty set clears the staging (the guard refusing to write
// under a newer peer's stamp). Files restart the quiet timer exactly
// as events do (D9): a lease lapsing changes the views with no event
// to carry them — the view-only commit ("0 events, N files") rides the
// same quiet period.
func (b *Batcher) SetFiles(files map[string][]byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(files) == 0 {
		b.files = nil
		return
	}
	b.files = make(map[string][]byte, len(files))
	for path, data := range files {
		b.files[path] = data
	}
	b.armLocked()
}

// armLocked starts or restarts the quiet-interval timer. Caller holds
// b.mu.
func (b *Batcher) armLocked() {
	if b.timer == nil {
		b.timer = time.AfterFunc(b.quiet, b.background)
	} else {
		b.timer.Reset(b.quiet)
	}
}

// Flush commits everything pending now and returns the result. With
// nothing pending it does nothing and returns nil.
func (b *Batcher) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.timer != nil {
		b.timer.Stop()
	}
	return b.flushLocked()
}

// background is the timer callback. Nobody is waiting on its error, so
// a failure is logged here, at failure time, with what stays pending
// (in memory only — lost if the process dies before a later flush
// succeeds). The timer is not re-armed: the next Add or SetFiles arms
// it, and Flush retries on demand.
func (b *Batcher) background() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.flushLocked(); err != nil {
		b.logf("store: background flush failed, %d events and %d files still pending: %v",
			len(b.pending), len(b.files), err)
	}
}

// logf writes through Log when set and the standard logger otherwise.
func (b *Batcher) logf(format string, args ...any) {
	if b.Log != nil {
		b.Log.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// flushLocked commits pending events and files; the caller holds b.mu.
// On failure everything remains pending for a later retry.
func (b *Batcher) flushLocked() error {
	if len(b.pending) == 0 && len(b.files) == 0 {
		return nil
	}
	start := time.Now()
	err := b.store.AppendBatch(Batch{Events: b.pending, Files: b.files})
	if err == nil {
		if b.Log != nil {
			b.Log.Printf("store: commit: %d events, %d files in %s",
				len(b.pending), len(b.files), time.Since(start).Round(10*time.Microsecond))
		}
		b.pending = nil
		b.files = nil
	}
	return err
}
