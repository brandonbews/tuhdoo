// Package syncer is the background sync loop (002 T2/T8, 001 D2): fetch
// the remote data branch, merge divergence at the application level,
// push. It is the network half of tuhdoo's convergence story; the local
// half is the daemon's serialized writer. The syncer never moves the
// local ref itself (T2, 2026-09-10: the store is the single mover): a
// fast-forward and a union merge both land through the store, so the
// daemon's replica and the ref advance together, and its replays read
// decoded events and leases from the store's cache by OID.
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
	OnMerged func()        // called after the local head moves (daemon refresh)
	Log      *log.Logger
	Now      func() time.Time // test hook; nil means time.Now
}

// Syncer runs the loop for one repository.
type Syncer struct {
	git      gitx.Git
	store    *store.Store
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

// New returns a Syncer over g that commits through st. The ref it
// syncs is st's data branch; opts.Ref must name the same one (empty
// means store.DefaultRef, as for the store).
func New(g gitx.Git, st *store.Store, opts Options) *Syncer {
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
		store:    st,
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

// Run loops until Stop. Blocking; callers put it in a goroutine. Started
// or stopped is decided under the mutex: a Run that begins after Stop
// returns at once, before any cycle and without marking itself started,
// so Stop never waits on a loop that will not run — and never misses one
// that will.
func (s *Syncer) Run() {
	s.mu.Lock()
	select {
	case <-s.stop:
		s.mu.Unlock()
		return
	default:
	}
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
// Run — nothing to wait for; a Run that begins afterwards sees the
// closed stop channel and exits before any cycle).
func (s *Syncer) Stop() {
	s.mu.Lock()
	s.stopOnce.Do(func() { close(s.stop) })
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

// Cycle runs one ref check → fetch → merge → push pass. Exported for
// tests and for a future `tuhdoo sync` command. The ref check runs
// first, remote or not: a remoteless daemon must notice a `branch -f`
// within a cycle just as a syncing one does (001 D2, 2026-09-10).
func (s *Syncer) Cycle() error {
	if err := s.noticeExternalMove(); err != nil {
		return err
	}
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

		local := s.store.Head()

		if remoteHead != "" && remoteHead != local {
			advanced, err := s.reconcile(local, remoteHead)
			if err != nil {
				return err
			}
			if advanced {
				local = s.store.Head()
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

// noticeExternalMove is the one place the ref is compared against git
// outside the store's own commit path (001 D2, 2026-09-10, accepted):
// one rev-parse per cycle. The store moves the ref and its replica
// together, so a ref that differs from the replica's head was moved
// from outside — a manual fetch into it, a `branch -f` — and the
// replica reloads from git (which reseeds the private index) and the
// daemon refreshes. A store that has not loaded yet simply loads. A
// commit landing between the two reads makes the ref look moved for
// one cycle; the reload is then a no-op that costs three processes.
func (s *Syncer) noticeExternalMove() error {
	head := s.store.Head()
	ref, err := s.git.ReadRef(s.ref)
	if err != nil {
		return fmt.Errorf("syncer: %w", err)
	}
	if ref == head {
		return nil
	}
	if head != "" {
		s.logf("sync: %s moved outside the daemon (%s -> %s); reloading", s.ref, head, ref)
	}
	if err := s.store.Load(); err != nil {
		return fmt.Errorf("syncer: reload after external move: %w", err)
	}
	if head != "" && s.onMerged != nil {
		s.onMerged()
	}
	return nil
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

// reconcile brings remote work into the local ref through the store:
// fast-forward when possible, app-level merge when divergent. Returns
// whether the local head moved. A lost compare-and-swap (the ref moved
// from outside mid-reconcile) is not an error — the store has reloaded,
// and the next cycle attempt re-reads and tries again.
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

	if weBehind {
		err = s.store.FastForward(local, remote)
	} else {
		// True divergence: the app-level merge, committed through the
		// store with both heads as parents.
		err = s.merge(remote)
	}
	if err != nil {
		if errors.Is(err, gitx.ErrRefCASFailed) {
			s.logf("sync: local ref moved during reconcile; retrying next pass")
			return false, nil
		}
		return false, fmt.Errorf("syncer: %w", err)
	}
	if !weBehind {
		s.bumpMerges()
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
