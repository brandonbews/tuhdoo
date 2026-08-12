// Package syncer is the background sync loop (002 T2/T8, 001 D2): fetch
// the remote data branch, merge divergence at the application level,
// push. It is the network half of tuhdoo's convergence story; the local
// half is the daemon's serialized writer.
package syncer

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
)

// TrackingRef is where fetches of the remote data branch land: a ref
// tuhdoo owns outright, so no remote-tracking config is assumed.
const TrackingRef = "refs/tuhdoo/remote"

// DefaultInterval is the T8 fetch cadence.
const DefaultInterval = 60 * time.Second

// maxCycleRetries bounds one cycle's fetch-merge-push loop when the
// remote keeps moving underneath it.
const maxCycleRetries = 4

// Status is a snapshot of the loop for status surfaces. Mode is one of
// "local-only", "syncing", "error".
type Status struct {
	Mode       string
	Remote     string
	LastFetch  time.Time
	LastPush   time.Time
	LastError  string
	Collisions int // non-fast-forward pushes encountered (T8 evidence)
	Merges     int
}

// Options tune a Syncer. Zero values mean production defaults.
type Options struct {
	Ref      string        // data branch; empty means store.DefaultRef
	Remote   string        // remote name; empty means "origin"
	Interval time.Duration // fetch cadence; <= 0 means DefaultInterval
	Ident    gitx.Identity // merge-commit identity
	OnMerged func()        // called after the local ref moves (daemon refresh)
	Log      *log.Logger
	Now      func() time.Time // test hook; nil means time.Now
}

// Syncer runs the loop for one repository.
type Syncer struct {
	git      gitx.Git
	ref      string
	remote   string
	interval time.Duration
	ident    gitx.Identity
	onMerged func()
	log      *log.Logger
	now      func() time.Time

	poke     chan struct{}
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	mu         sync.Mutex
	started    bool
	status     Status
	lastPushed string // last local OID successfully pushed
	replay     *core.Replayer
}

func New(g gitx.Git, opts Options) *Syncer {
	if opts.Ref == "" {
		opts.Ref = store.DefaultRef
	}
	if opts.Remote == "" {
		opts.Remote = "origin"
	}
	if opts.Interval <= 0 {
		opts.Interval = DefaultInterval
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Syncer{
		git:      g,
		ref:      opts.Ref,
		remote:   opts.Remote,
		interval: opts.Interval,
		ident:    opts.Ident,
		onMerged: opts.OnMerged,
		log:      opts.Log,
		now:      opts.Now,
		poke:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		replay:   core.NewReplayer(),
	}
}

func (s *Syncer) logf(format string, args ...any) {
	if s.log != nil {
		s.log.Printf(format, args...)
	}
}

// Run loops until Stop. Blocking; callers put it in a goroutine.
func (s *Syncer) Run() {
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	defer close(s.done)
	for {
		s.cycleAndRecord()
		select {
		case <-s.stop:
			return
		case <-s.poke:
		case <-time.After(s.interval):
		}
	}
}

// Poke requests an immediate cycle — the daemon calls this after eager
// flushes (claims, escalations) so they hit the wire without waiting for
// the interval (T8).
func (s *Syncer) Poke() {
	select {
	case s.poke <- struct{}{}:
	default: // a cycle is already requested
	}
}

// Stop ends the loop and waits for it to finish. Safe to call more than
// once, and safe when Run was never started (a daemon shut down before
// Run — nothing to wait for; if Run starts afterwards it sees the closed
// stop channel and exits after one cycle).
func (s *Syncer) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if started {
		<-s.done
	}
}

// Status returns a snapshot of the loop's health.
func (s *Syncer) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Syncer) cycleAndRecord() {
	err := s.Cycle()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		// Unreachable remotes are routine (offline is a degradation,
		// not an error state worth screaming about — D10); record and
		// carry on.
		s.status.Mode = "error"
		s.status.LastError = err.Error()
		return
	}
	s.status.LastError = ""
}

// Cycle runs one fetch → merge → push pass. Exported for tests and for
// a future `tuhdoo sync` command.
func (s *Syncer) Cycle() error {
	if _, err := s.git.RemoteURL(s.remote); err != nil {
		if errors.Is(err, gitx.ErrNoRemote) {
			s.setMode("local-only")
			return nil
		}
		return fmt.Errorf("syncer: %w", err)
	}

	for attempt := 0; attempt < maxCycleRetries; attempt++ {
		remoteHead, err := s.fetch()
		if err != nil {
			return err
		}

		local, err := s.git.ReadRef(s.ref)
		if err != nil {
			return fmt.Errorf("syncer: %w", err)
		}

		if remoteHead != "" && remoteHead != local {
			advanced, err := s.reconcile(local, remoteHead)
			if err != nil {
				return err
			}
			if advanced {
				local, err = s.git.ReadRef(s.ref)
				if err != nil {
					return fmt.Errorf("syncer: %w", err)
				}
			}
		}

		if local == s.lastPushedOID() && local == remoteHead {
			s.setMode("syncing")
			return nil
		}
		err = s.git.Push(s.remote, s.ref+":"+s.ref)
		if err == nil {
			s.recordPush(local)
			s.setMode("syncing")
			return nil
		}
		if errors.Is(err, gitx.ErrNonFastForward) {
			// Someone pushed between our fetch and push: go around
			// again — fetch their work, merge, retry (D6 in action).
			s.bumpCollisions()
			continue
		}
		return err
	}
	return fmt.Errorf("syncer: remote %s kept moving for %d attempts", s.remote, maxCycleRetries)
}

// fetch updates TrackingRef and returns the remote head, or "" when the
// remote has no data branch yet (first push still pending).
func (s *Syncer) fetch() (string, error) {
	err := s.git.Fetch(s.remote, s.ref+":"+TrackingRef)
	if err != nil {
		if errors.Is(err, gitx.ErrRemoteRefMissing) {
			s.recordFetch()
			return "", nil
		}
		return "", err
	}
	s.recordFetch()
	oid, err := s.git.ReadRef(TrackingRef)
	if err != nil {
		return "", fmt.Errorf("syncer: %w", err)
	}
	return oid, nil
}

// reconcile brings remote work into the local ref: fast-forward when
// possible, app-level merge when divergent. Returns whether the local
// ref moved. A CAS loss (the daemon committed mid-reconcile) is not an
// error — the next cycle attempt re-reads and tries again.
func (s *Syncer) reconcile(local, remote string) (bool, error) {
	theirsBehind, err := s.git.IsAncestor(remote, local)
	if err != nil {
		return false, err
	}
	if theirsBehind {
		return false, nil // we are strictly ahead; nothing to bring in
	}
	weBehind, err := s.git.IsAncestor(local, remote)
	if err != nil {
		return false, err
	}

	target := remote
	if !weBehind {
		// True divergence: build the app-level merge commit.
		target, err = s.merge(local, remote)
		if err != nil {
			return false, err
		}
		s.bumpMerges()
	}

	err = s.git.UpdateRef(s.ref, target, local)
	if err != nil {
		if errors.Is(err, gitx.ErrRefCASFailed) {
			s.logf("sync: local ref moved during reconcile; retrying next pass")
			return false, nil
		}
		return false, fmt.Errorf("syncer: %w", err)
	}
	if s.onMerged != nil {
		s.onMerged()
	}
	return true, nil
}

func (s *Syncer) setMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Mode = mode
	s.status.Remote = s.remote
}

func (s *Syncer) recordFetch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastFetch = s.now()
}

func (s *Syncer) recordPush(oid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastPush = s.now()
	s.lastPushed = oid
}

func (s *Syncer) lastPushedOID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPushed
}

func (s *Syncer) bumpCollisions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Collisions++
}

func (s *Syncer) bumpMerges() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Merges++
}
