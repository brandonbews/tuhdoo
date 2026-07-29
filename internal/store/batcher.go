package store

import (
	"sync"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// DefaultQuiet is the commit debounce (T8): a fleet's burst of events
// becomes one commit, while `watch` still feels live.
const DefaultQuiet = 2 * time.Second

// Batcher accumulates events and commits them as one batch after a quiet
// interval with no new additions. Flush commits eagerly — the decision
// to do so for claims and escalations (T8) belongs to the daemon; the
// Batcher only supplies the mechanism.
//
// Error reporting: a background (timer-driven) flush that fails keeps
// its events pending and records the error; LastError returns it until a
// later flush attempt succeeds. Callers wanting synchronous errors use
// Flush, which returns them directly.
//
// One mutex guards everything; the only concurrency is the time.Timer's
// callback, which takes the same mutex. AppendBatch runs while holding
// the lock — Add blocks during a flush, which is the point: writes to
// the branch stay serialized.
type Batcher struct {
	store *Store
	quiet time.Duration

	mu      sync.Mutex
	pending []event.Event
	timer   *time.Timer
	lastErr error
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
	if b.timer == nil {
		b.timer = time.AfterFunc(b.quiet, b.background)
	} else {
		b.timer.Reset(b.quiet)
	}
}

// Flush commits everything pending now and returns the result. With
// nothing pending it does nothing and returns nil (it does not clear a
// recorded background error).
func (b *Batcher) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.timer != nil {
		b.timer.Stop()
	}
	return b.flushLocked()
}

// LastError returns the most recent background flush failure, or nil.
// It is cleared by the next flush attempt that succeeds (the failed
// events stay pending, so a later flush retries them).
func (b *Batcher) LastError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastErr
}

// background is the timer callback.
func (b *Batcher) background() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// flushLocked commits pending events; the caller holds b.mu. On failure
// the events remain pending for a later retry and the error is recorded
// for LastError.
func (b *Batcher) flushLocked() error {
	if len(b.pending) == 0 {
		return nil
	}
	err := b.store.AppendBatch(Batch{Events: b.pending})
	if err == nil {
		b.pending = nil
	}
	b.lastErr = err
	return err
}
