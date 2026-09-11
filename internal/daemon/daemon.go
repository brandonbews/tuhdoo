// Package daemon is the per-repo tuhdoo daemon (002 T4): single
// instance per repository, a Unix-socket JSON HTTP API, and the one
// mutex through which every write is serialized — that mutex is D2's
// machine-local serialization, not an implementation detail.
package daemon

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
	"github.com/brandonbews/tuhdoo/internal/syncer"
	"github.com/brandonbews/tuhdoo/internal/views"
)

// DefaultLeaseTTL is the claim lease lifetime (T8).
const DefaultLeaseTTL = 15 * time.Minute

// DefaultMCPKeepAlive is the MCP session ping interval. Session
// liveness is what keeps leases renewed (T5: session-bound leases), so
// the daemon pings well inside the lease TTL.
const DefaultMCPKeepAlive = 30 * time.Second

// maxSocketPath is the longest unix socket path we bind (sun_path is
// 104 bytes on macOS including the NUL).
const maxSocketPath = 103

// socketPath picks where the daemon binds its unix socket: beside
// daemon.json in the runtime dir when that fits sun_path, else a short
// path under tmpDir. The fallback is a hash of the runtime dir rather
// than a random name so that a daemon restarting after a crash computes
// the same path, keeping it distinct per repo and reachable by the
// stale-socket removal in New. Discovery is unaffected either way:
// clients dial whatever daemon.json says. Callers pass os.TempDir().
func socketPath(dir, tmpDir string) (string, error) {
	sock := filepath.Join(dir, "daemon.sock")
	if len(sock) <= maxSocketPath {
		return sock, nil
	}
	sum := sha256.Sum256([]byte(dir))
	fallback := filepath.Join(tmpDir, "tuhdoo-"+hex.EncodeToString(sum[:6])+".sock")
	if len(fallback) > maxSocketPath {
		return "", fmt.Errorf("daemon: socket path %s exceeds the %d-byte unix socket limit, and so does the fallback %s", sock, maxSocketPath, fallback)
	}
	return fallback, nil
}

// defaultIdent commits on behalf of the daemon; the true author of each
// change is the actor stamped on the events themselves.
var defaultIdent = gitx.Identity{Name: "tuhdoo daemon", Email: "daemon@tuhdoo.invalid"}

// ErrAlreadyRunning is New's failure when the repository's single-
// instance lock is held by a live daemon (errors.Is matches it through
// the wrapping). The CLI that spawned this process reads it as "the
// flock loser: the winner's socket is coming", not as a death.
var ErrAlreadyRunning = errors.New("daemon: another daemon is already running for this repo")

// Options tune a Daemon. The zero value is production defaults.
type Options struct {
	Ref          string        // data branch ref; empty means store.DefaultRef
	Ident        gitx.Identity // commit identity; zero means defaultIdent
	Quiet        time.Duration // commit debounce; <= 0 means store.DefaultQuiet
	LeaseTTL     time.Duration // claim lease TTL; <= 0 means DefaultLeaseTTL
	SyncInterval time.Duration // fetch cadence; <= 0 means syncer.DefaultInterval
	MCPKeepAlive time.Duration // MCP session ping interval; <= 0 means DefaultMCPKeepAlive
	Version      string        // binary version reported to MCP clients; empty means "dev"
	Log          *log.Logger   // nil means stderr

	// git, when set, stands in for gitx.New(root): a test hook for
	// wrapping the real git (a load held open, a remote that moves).
	// Unexported on purpose — production always runs the real thing.
	git gitx.Git
	// now and afterFunc, when set, stand in for time.Now and
	// time.AfterFunc: the clock every lease verdict and the one
	// transition timer read. Test hooks, so a lease lapsing at T can be
	// observed at exactly T instead of after a real wait.
	now       func() time.Time
	afterFunc func(time.Duration, func()) timer
}

// timer is the slice of *time.Timer the transition timer uses, so a
// test can stand in a fake that fires on demand.
type timer interface {
	Stop() bool
	Reset(time.Duration) bool
}

// discovery is the daemon.json contents: how CLIs and shims find the
// live daemon.
type discovery struct {
	PID     int    `json:"pid"`
	Socket  string `json:"socket"`
	Started string `json:"started"` // RFC3339
}

// Daemon owns one repository's tuhdoo runtime: the flock, the socket,
// the store, and the cached replayed state.
type Daemon struct {
	root         string
	dir          string // <git-dir>/tuhdoo runtime dir
	machine      string
	leaseTTL     time.Duration
	mcpKeepAlive time.Duration
	version      string
	log          *log.Logger

	store   *store.Store
	batcher *store.Batcher
	replay  *core.Replayer
	sync    *syncer.Syncer
	// now is the daemon's clock (Options.now, or time.Now); afterFunc
	// makes the transition timer (Options.afterFunc, or time.AfterFunc).
	now       func() time.Time
	afterFunc func(time.Duration, func()) timer
	// poke asks the sync loop for an immediate pass (d.sync.Poke); a
	// seam, so a test can count the pokes eager commits owe (T8).
	poke func()

	// mu serializes every write and guards all fields below. Reads take
	// it too — boring wins over a RWMutex at v0 volumes.
	mu     sync.Mutex
	state  *core.State
	leases map[string]time.Time
	// The versioned replica (001 D2/D3, 002 T4, 2026-09-10). version
	// counts state changes: it moves on every replay — a staged write,
	// a merge, a lease transition, the first load — and a read at an
	// unchanged version is answered from state as memoized. validUntil
	// is the instant the memo stops being right on its own: the next
	// lease expiry (core.NextTransition), zero when no live lease can
	// lapse. A read arriving past it replays once before answering;
	// the transition timer, armed to it and re-armed on every bump,
	// replays at that instant so parked snapshot requests wake without
	// a read. changed is the broadcast: closed and replaced on every
	// bump, so a parked request waits on the channel it took under mu
	// and wakes when that channel closes. replays counts replays, the
	// evidence for the memo tests.
	stateVersion uint64
	validUntil   time.Time
	changed      chan struct{}
	transition   timer
	replays      int
	// viewsGuardLogged keeps the "newer stamp" refusal to one log line:
	// it would otherwise repeat on every bump.
	viewsGuardLogged bool
	// loaded flips true once the first replay has landed (T4 startup
	// order, 2026-09-10: the socket is bound and daemon.json written
	// before the ledger is loaded). Until then every read answers sync
	// mode "starting" and every write a retryable 503 — the daemon
	// promises nothing it has not yet replayed.
	loaded bool
	// stopping is set the moment Shutdown begins, so the load half of
	// startup never starts the sync loop into a daemon that is already
	// tearing down.
	stopping bool
	// startErr is why the first load failed; Run returns it so the
	// process exits non-zero with the reason already in daemon.log.
	startErr error
	// degraded is non-nil after a fail-safe replay error (T3): reads
	// keep serving the last good state, writes are rejected with 503.
	degraded error
	// written holds events this process has produced that are not yet
	// visible on the branch, overlaid on the loaded events at refresh so
	// debounced (not-yet-committed) writes are visible immediately.
	// Replay dedupes by ID, so a brief overlap with already-committed
	// events is harmless; refreshLocked trims events off the overlay as
	// soon as a load sees them on the branch, so it stays bounded by the
	// debounce window, not the process lifetime.
	written []event.Event
	// lastLoggedEvents is the branch event count of the last refresh
	// timing line, so refresh logging fires on change, not on every poll.
	lastLoggedEvents int
	// entropy is monotonic so events minted in the same millisecond get
	// ULIDs in mint order — replay order must not invert, e.g. a claim
	// sorting before the task it claims. Guarded by mu.
	entropy *ulid.MonotonicEntropy

	// agentMu guards agentSeq, the per-client-name counters behind
	// auto-minted session principals (agentNameHeader in mcp.go).
	agentMu  sync.Mutex
	agentSeq map[string]int

	lockFile *os.File
	ln       net.Listener
	srv      *http.Server
	sockPath string
	jsonPath string

	shutdownOnce sync.Once
	cleanupOnce  sync.Once
	// loadDone closes when the first load has finished, either way —
	// Shutdown joins it, so the daemon is never torn down under a
	// running adopt, init, or replay.
	loadDone chan struct{}
	done     chan struct{}
}

// New prepares a daemon for the git repository rooted at root: acquires
// the single-instance lock, loads (or mints) the machine id, opens the
// store, binds the socket, and writes daemon.json. Run serves it and
// loads the ledger — in that order (T4, 2026-09-10: lock, socket,
// discovery file, then load), so a client finds the socket while a
// cold load is still running instead of timing out on it.
func New(root string, opts Options) (*Daemon, error) {
	logger := opts.Log
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	ident := opts.Ident
	if ident == (gitx.Identity{}) {
		ident = defaultIdent
	}
	ttl := opts.LeaseTTL
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	keepAlive := opts.MCPKeepAlive
	if keepAlive <= 0 {
		keepAlive = DefaultMCPKeepAlive
	}
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	gd, err := gitDirOf(root)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(gd, "tuhdoo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("daemon: create runtime dir: %w", err)
	}

	lockFile, err := acquireLock(dir)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			lockFile.Close()
		}
	}()

	machine, err := machineID(dir)
	if err != nil {
		return nil, err
	}

	g := opts.git
	if g == nil {
		g, err = gitx.New(root)
		if err != nil {
			return nil, fmt.Errorf("daemon: %w", err)
		}
	}
	st := store.New(g, opts.Ref, ident)
	now := opts.now
	if now == nil {
		now = time.Now
	}
	afterFunc := opts.afterFunc
	if afterFunc == nil {
		afterFunc = func(dur time.Duration, f func()) timer { return time.AfterFunc(dur, f) }
	}

	d := &Daemon{
		root:         root,
		dir:          dir,
		machine:      machine,
		leaseTTL:     ttl,
		mcpKeepAlive: keepAlive,
		version:      version,
		log:          logger,
		store:        st,
		batcher:      store.NewBatcher(st, opts.Quiet),
		replay:       core.NewReplayer(),
		now:          now,
		afterFunc:    afterFunc,
		changed:      make(chan struct{}),
		lockFile:     lockFile,
		entropy:      ulid.Monotonic(rand.Reader, 0),
		agentSeq:     make(map[string]int),
		loadDone:     make(chan struct{}),
		done:         make(chan struct{}),
	}
	d.batcher.Log = logger
	d.sync = syncer.New(g, st, syncer.Options{
		Ref:      opts.Ref,
		Interval: opts.SyncInterval,
		Ident:    ident,
		OnMerged: func() {
			if err := d.Refresh(); err != nil {
				logger.Printf("daemon: refresh after sync: %v", err)
			}
		},
		Log: logger,
	})
	d.poke = d.sync.Poke
	d.state = emptyState()

	sock, err := socketPath(dir, os.TempDir())
	if err != nil {
		return nil, err
	}
	// A leftover socket file from a crashed daemon is safe to remove:
	// the flock above proves no live daemon owns it.
	if err := os.Remove(sock); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("daemon: remove stale socket: %w", err)
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("daemon: listen: %w", err)
	}
	d.ln = ln
	d.sockPath = sock
	d.jsonPath = filepath.Join(dir, "daemon.json")

	disc, err := json.Marshal(discovery{
		PID:     os.Getpid(),
		Socket:  sock,
		Started: now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("daemon: %w", err)
	}
	if err := os.WriteFile(d.jsonPath, append(disc, '\n'), 0o644); err != nil {
		ln.Close()
		return nil, fmt.Errorf("daemon: write daemon.json: %w", err)
	}

	d.srv = &http.Server{Handler: d.handler()}
	ok = true
	return d, nil
}

// Run serves the API until Shutdown is called; it returns after cleanup
// completes. Serving starts first and the ledger loads behind it (T4
// startup order): until the first replay lands, reads answer sync mode
// "starting" and writes a retryable 503. A first load that fails ends
// the daemon — socket and discovery file torn down, the reason logged,
// the error returned — so no client can loop on "starting" forever.
// Every exit path logs its reason.
func (d *Daemon) Run() error {
	d.log.Printf("daemon: pid %d serving %s", os.Getpid(), d.sockPath)
	go d.start()
	err := d.srv.Serve(d.ln)
	if !errors.Is(err, http.ErrServerClosed) {
		d.log.Printf("daemon: exiting: listener failed: %v", err)
		d.cleanup()
		return err
	}
	<-d.done // wait for Shutdown's cleanup
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.startErr
}

// start is the load half of startup, run in the background once the
// socket is serving: load the ledger, then run the sync loop for the
// life of the daemon. A first load that fails ends the daemon through
// the ordinary Shutdown — the reason is logged here, outside any
// once, so it is never skipped — and Run returns it.
func (d *Daemon) start() {
	d.mu.Lock()
	stopping := d.stopping
	d.mu.Unlock()
	if stopping {
		// Shutdown began before the load could start: there is nothing
		// to load for, and Shutdown is waiting on loadDone.
		close(d.loadDone)
		return
	}
	err := d.load()
	d.mu.Lock()
	d.startErr = err
	d.mu.Unlock()
	close(d.loadDone)
	if err != nil {
		d.log.Printf("daemon: initial load failed: %v", err)
		d.Shutdown("initial load failed")
		return
	}
	d.mu.Lock()
	stopping = d.stopping
	d.mu.Unlock()
	if stopping {
		return // Shutdown began during the load; it owns the final sync
	}
	d.sync.Run()
}

// load brings the ledger into memory for the first time: adopt a
// remote data branch if one exists, mint the branch if none does, load
// the store's replica (head, tree, every event and lease blob in one
// batch — T2; the private index is reseeded by the first commit), and
// replay. The first
// replay is installed and the daemon marked loaded under one critical
// section, so a request parked on d.mu during the load is answered
// with state, never with the placeholder after the state was
// installed. Loaded covers the T3 fail-safe case too — the daemon then
// serves reads of the last comprehensible state (here: empty) with
// writes rejected; any other failure is returned for start to end the
// daemon with. The long part of startup — the adopt's fetch, the init,
// the store load — runs outside the mutex; only the replay (memory,
// milliseconds) holds it.
func (d *Daemon) load() error {
	// Clone-join before Init: a fresh clone whose remote already carries
	// the data branch adopts that history instead of minting a second
	// orphan root. Best-effort — on any failure (no remote, unreachable,
	// branch absent) Init mints exactly as before, and the app-level
	// union merge remains the correctness backstop for two-root histories.
	d.sync.AdoptRemoteBranch()
	if err := d.store.Init(); err != nil {
		return fmt.Errorf("daemon: initial load: %w", err)
	}
	loadStart := time.Now()
	if err := d.store.Load(); err != nil {
		return fmt.Errorf("daemon: initial load: %w", err)
	}
	d.log.Printf("daemon: load: replica at %s in %s", d.store.Head(), time.Since(loadStart).Round(10*time.Microsecond))
	d.mu.Lock()
	// The first replay is the first version bump (0 → 1): a snapshot
	// request parked on version 0 during the load wakes here.
	err := d.replayLocked(d.now())
	if err == nil || isFailSafe(err) {
		d.loaded = true
	}
	d.mu.Unlock()
	if err != nil && !isFailSafe(err) {
		return fmt.Errorf("daemon: initial load: %w", err)
	}
	if err != nil {
		d.log.Printf("daemon: starting in fail-safe read-only mode: %v", err)
	}
	return nil
}

// Shutdown stops serving, joins the first load, flushes pending events,
// runs a final sync, and removes the socket and discovery file. Safe to
// call more than once. It waits for the load Run started — a daemon is
// never torn down under a running adopt, init, or replay (the adopt's
// own fetch timeout bounds the wait) — so Run must have been called;
// when the load did not land (it failed, or the daemon is going down
// before it finished), nothing was staged and the sync loop never ran,
// so the flush and final sync are skipped. Every parked snapshot
// request is woken first (T4: shutdown must wake every parked request
// before the HTTP server drains, or the drain waits on them).
func (d *Daemon) Shutdown(reason string) {
	d.shutdownOnce.Do(func() {
		d.log.Printf("daemon: exiting: %s", reason)
		d.mu.Lock()
		d.stopping = true
		if d.transition != nil {
			d.transition.Stop()
		}
		d.wakeLocked()
		d.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = d.srv.Shutdown(ctx)
		<-d.loadDone
		d.mu.Lock()
		loaded := d.loaded
		d.mu.Unlock()
		if loaded {
			if err := d.batcher.Flush(); err != nil {
				d.log.Printf("daemon: final flush failed, events lost: %v", err)
			}
			d.sync.Stop()
			// Best-effort final push so a laptop closing its lid doesn't
			// strand the last few commits locally.
			if err := d.sync.Cycle(); err != nil {
				d.log.Printf("daemon: final sync: %v", err)
			}
		}
		d.cleanup()
		close(d.done)
	})
}

func (d *Daemon) cleanup() {
	d.cleanupOnce.Do(func() {
		os.Remove(d.sockPath)
		os.Remove(d.jsonPath)
		d.lockFile.Close() // releases the flock
	})
}

// SocketPath returns the bound unix socket path.
func (d *Daemon) SocketPath() string { return d.sockPath }

// Refresh recomputes cached state from the store's replica and bumps
// the state version. The sync loop calls this after every head move it
// causes or notices (wired via OnMerged in New): a merge is a version
// bump exactly like a local write (002 T2, 2026-09-10).
func (d *Daemon) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.replayLocked(d.now())
}

// slowRefresh is the refresh duration above which a timing line is
// always logged, whether or not the event count moved.
const slowRefresh = 250 * time.Millisecond

// replayLocked takes the replay input from the store's replica,
// replays at now, installs the result as the memoized state, and bumps
// the version — every caller is a state change: a staged write, a
// merge, a lease transition, the first load. It spawns no git (001 D2,
// 2026-09-10: reads never spawn git): the store's head tree and decode
// caches are complete after every load and commit, so the "load" here
// is a walk of the in-memory tree. On a fail-safe replay error the
// daemon degrades: the last good state keeps serving reads and every
// write path starts rejecting — and that is a version bump too, since
// the snapshot's degraded field just changed. Caller holds d.mu.
func (d *Daemon) replayLocked(now time.Time) error {
	d.replays++
	loadStart := time.Now()
	events, leases, err := d.store.ReplayInput()
	if err != nil {
		return err
	}
	loadDur := time.Since(loadStart)

	// Events the load saw on the branch no longer need the overlay.
	d.written = trimOverlay(d.written, events)
	combined := make([]event.Event, 0, len(events)+len(d.written))
	combined = append(combined, events...)
	combined = append(combined, d.written...)

	replayStart := time.Now()
	st, err := d.replay.Replay(core.Input{
		Events: combined,
		Leases: leases,
		Now:    now,
	})
	replayDur := time.Since(replayStart)
	if err != nil {
		if isFailSafe(err) {
			wasDegraded := d.degraded != nil
			d.degraded = err
			if !wasDegraded {
				d.bumpLocked(now)
			}
		}
		return err
	}
	d.degraded = nil
	d.state = st
	d.leases = leases
	d.validUntil, _ = core.NextTransition(transitionLeases(st, leases), now)

	// One timing line per change in branch event count (≈ one per landed
	// commit), plus any refresh slow enough to worry about. This is the
	// live evidence stream for the replay-scaling question.
	if len(events) != d.lastLoggedEvents || loadDur+replayDur >= slowRefresh {
		d.lastLoggedEvents = len(events)
		d.log.Printf("daemon: refresh: %d events (+%d overlay), %d leases; load %s, replay %s",
			len(events), len(d.written), len(leases),
			loadDur.Round(10*time.Microsecond), replayDur.Round(10*time.Microsecond))
	}
	d.bumpLocked(now)
	return nil
}

// transitionLeases narrows the lease map to the leases whose lapse
// would change replayed state: those of claims that are active (the
// holder expires, the task returns to the pool) or voided (the loser's
// unreported attempt closes as superseded). A finished or released
// claim's lease lapsing changes nothing, so it must not wake anyone.
func transitionLeases(st *core.State, leases map[string]time.Time) map[string]time.Time {
	out := make(map[string]time.Time)
	for id, c := range st.Claims {
		if c.Status != core.ClaimActive && c.Status != core.ClaimVoided {
			continue
		}
		if exp, ok := leases[id]; ok {
			out[id] = exp
		}
	}
	return out
}

// freshenLocked replays only when the memo has aged past validUntil —
// a lease has lapsed since the last replay and nothing re-evaluated it
// yet (the transition timer runs on the same clock, but a machine
// waking from sleep can arrive here first). Otherwise the memo stands:
// this is the read gate's whole cost at an unchanged version. Caller
// holds d.mu.
func (d *Daemon) freshenLocked(now time.Time) error {
	if d.validUntil.IsZero() || now.Before(d.validUntil) {
		return nil
	}
	return d.replayLocked(now)
}

// bumpLocked is the one place the state version moves: increment,
// wake every parked snapshot request, re-render the views against the
// new state (T6: views follow the version), and re-arm the transition
// timer to the memo's new validUntil. Caller holds d.mu.
func (d *Daemon) bumpLocked(now time.Time) {
	d.stateVersion++
	d.wakeLocked()
	d.renderViewsLocked()
	d.armLocked(now)
}

// wakeLocked closes the broadcast channel and replaces it: every
// request parked on the old channel returns. Shutdown calls it without
// a bump, so parked requests answer with the unchanged version and the
// client re-polls into a closed socket instead of hanging the drain.
// Caller holds d.mu.
func (d *Daemon) wakeLocked() {
	close(d.changed)
	d.changed = make(chan struct{})
}

// armLocked points the transition timer at validUntil, or stops it
// when no live lease can lapse. Caller holds d.mu.
func (d *Daemon) armLocked(now time.Time) {
	if d.validUntil.IsZero() {
		if d.transition != nil {
			d.transition.Stop()
		}
		return
	}
	dur := d.validUntil.Sub(now)
	if dur < 0 {
		dur = 0
	}
	if d.transition == nil {
		d.transition = d.afterFunc(dur, d.onTransition)
		return
	}
	d.transition.Reset(dur)
}

// onTransition is the timer callback: a lease is due to lapse. A
// replay at the current instant moves the verdict, bumps the version
// (waking parked requests, re-rendering views — a lapse is a view-only
// commit, 001 D9), and re-arms for the next lapse. Fired early (clock
// skew, or a fake clock that has not advanced), it re-arms and waits.
func (d *Daemon) onTransition() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopping || !d.loaded || d.degraded != nil || d.validUntil.IsZero() {
		return
	}
	now := d.now()
	if now.Before(d.validUntil) {
		d.armLocked(now)
		return
	}
	if err := d.replayLocked(now); err != nil {
		d.log.Printf("daemon: lease transition replay: %v", err)
	}
}

// trimOverlay drops overlay events that have landed on the branch: once
// an event is in the committed set the overlay copy is redundant (replay
// dedupes by ID), and dropping it here is what keeps the overlay from
// growing for the process lifetime.
func trimOverlay(written, committed []event.Event) []event.Event {
	if len(written) == 0 {
		return written
	}
	onBranch := make(map[string]bool, len(committed))
	for _, e := range committed {
		onBranch[e.ID] = true
	}
	var kept []event.Event
	for _, e := range written {
		if !onBranch[e.ID] {
			kept = append(kept, e)
		}
	}
	return kept
}

// stageLocked records events in the overlay and the batcher. Caller
// holds d.mu.
func (d *Daemon) stageLocked(evs ...event.Event) {
	for _, e := range evs {
		d.written = append(d.written, e)
		d.batcher.Add(e)
	}
}

// commitLocked stages events, replays (the version bump stages the
// views these events produce, so they ride the same batch — an eager
// commit carries its own views), and flushes immediately when eager
// (claim and escalation writes, T8 — everything else rides the
// debounce). A fail-safe replay must not strand the events themselves,
// so the flush runs regardless. Caller holds d.mu.
func (d *Daemon) commitLocked(eager bool, evs ...event.Event) error {
	d.stageLocked(evs...)
	replayErr := d.replayLocked(d.now())
	if eager {
		if err := d.batcher.Flush(); err != nil {
			return fmt.Errorf("daemon: flush: %w", err)
		}
		// Eager writes deserve eager wire time too (T8): ask the sync
		// loop for an immediate pass.
		d.poke()
	}
	return replayErr
}

// renderViewsLocked renders the views from the memoized state and
// stages the ones whose bytes differ from the head tree, to ride the
// next batch commit (T6, 2026-09-10: views are a projection of the
// versioned state, rendered on every bump, staged only when they
// change). It runs from bumpLocked, never from a write path — staging
// is no longer a call a write path can forget. Highest version wins
// (B8): views stamped by a newer generator are never overwritten —
// events still flow, and the newer peer keeps regenerating. Degraded,
// there is no trustworthy state to render. Caller holds d.mu.
func (d *Daemon) renderViewsLocked() {
	if d.degraded != nil || d.state == nil {
		return
	}
	meta, err := d.store.ReadFile(views.MetaPath)
	if err != nil {
		d.log.Printf("daemon: views: reading stamp: %v", err)
		return
	}
	if !views.CanWrite(meta) {
		if !d.viewsGuardLogged {
			d.viewsGuardLogged = true
			d.log.Printf("daemon: views stamped by a newer tuhdoo (format %d > %d); writing events only",
				views.Format(meta), views.FormatVersion)
		}
		// Anything staged before the newer stamp arrived must not land
		// on top of it.
		d.batcher.SetFiles(nil)
		return
	}
	d.viewsGuardLogged = false
	d.batcher.SetFiles(d.store.Changed(views.Render(d.state)))
}

// newEventLocked mints an event at the daemon layer (core stays pure):
// ULID from the monotonic entropy, machine from machine-id, actor from
// the request. Caller holds d.mu.
func (d *Daemon) newEventLocked(typ, actor, task string, payload any) (event.Event, error) {
	id, err := event.NewID(d.now(), d.entropy)
	if err != nil {
		return event.Event{}, err
	}
	return event.New(id, typ, event.Versions[typ], actor, d.machine, task, payload)
}

func isFailSafe(err error) bool {
	return errors.Is(err, core.ErrCannotReplay) || errors.Is(err, core.ErrMalformedEvent)
}

func emptyState() *core.State {
	return &core.State{
		Tasks:        make(map[string]*core.Task),
		Claims:       make(map[string]*core.Claim),
		ClaimsByTask: make(map[string][]string),
		Escalations:  make(map[string]*core.Escalation),
	}
}

// gitDirOf locates the .git directory for root. Chosen over shelling
// out to `git rev-parse --git-dir`: an explicit repo root plus a stat
// keeps lifecycle setup subprocess-free, and the linked-worktree form
// (".git" is a file holding "gitdir: <path>") is handled directly.
func gitDirOf(root string) (string, error) {
	p := filepath.Join(root, ".git")
	fi, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("daemon: %s is not a git repository: %w", root, err)
	}
	if fi.IsDir() {
		return p, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("daemon: read %s: %w", p, err)
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return "", fmt.Errorf("daemon: %s is neither a directory nor a gitdir pointer", p)
	}
	target = strings.TrimSpace(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return filepath.Clean(target), nil
}

// acquireLock takes the single-instance flock. The kernel releases it
// on process death, so there are no stale-pid heuristics: if the lock
// is held, a daemon is alive.
func acquireLock(dir string) (*os.File, error) {
	path := filepath.Join(dir, "daemon.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("daemon: open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("%w (%s); lock %s is held", ErrAlreadyRunning, livePID(dir), path)
	}
	return f, nil
}

// livePID names the running daemon from the discovery file, best-effort.
func livePID(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "daemon.json"))
	if err != nil {
		return "pid unknown"
	}
	var disc discovery
	if json.Unmarshal(b, &disc) != nil || disc.PID == 0 {
		return "pid unknown"
	}
	return fmt.Sprintf("pid %d", disc.PID)
}

// machineID returns the stable per-machine id, minting it on first use.
func machineID(dir string) (string, error) {
	path := filepath.Join(dir, "machine-id")
	b, err := os.ReadFile(path)
	if err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("daemon: read machine-id: %w", err)
	}
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("daemon: machine-id entropy: %w", err)
	}
	id := "m-" + hex.EncodeToString(buf[:])
	if err := os.WriteFile(path, []byte(id+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("daemon: write machine-id: %w", err)
	}
	return id, nil
}
