// Package daemon is the per-repo tuhdoo daemon (002 T4): single
// instance per repository, a Unix-socket JSON HTTP API, and the one
// mutex through which every write is serialized — that mutex is D2's
// machine-local serialization, not an implementation detail.
package daemon

import (
	"context"
	"crypto/rand"
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

// defaultIdent commits on behalf of the daemon; the true author of each
// change is the actor stamped on the events themselves.
var defaultIdent = gitx.Identity{Name: "tuhdoo daemon", Email: "daemon@tuhdoo.invalid"}

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

	// mu serializes every write and guards all fields below. Reads take
	// it too — boring wins over a RWMutex at v0 volumes.
	mu     sync.Mutex
	state  *core.State
	leases map[string]time.Time
	// degraded is non-nil after a fail-safe replay error (T3): reads
	// keep serving the last good state, writes are rejected with 503.
	degraded error
	// written holds every event this process has produced, overlaid on
	// LoadEvents at refresh so debounced (not-yet-committed) writes are
	// visible immediately. Replay dedupes by ID, so overlap with
	// already-committed events is harmless. Grows for the process
	// lifetime — with full replay per write, the obvious later
	// optimization point (incremental apply) covers both.
	written []event.Event
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
	done         chan struct{}
}

// New prepares a daemon for the git repository rooted at root: acquires
// the single-instance lock, loads (or mints) the machine id, opens the
// store, computes initial state, and binds the socket. Run serves it.
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

	g, err := gitx.New(root)
	if err != nil {
		return nil, fmt.Errorf("daemon: %w", err)
	}
	st := store.New(g, opts.Ref, ident)
	if err := st.Init(); err != nil {
		return nil, fmt.Errorf("daemon: %w", err)
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
		lockFile:     lockFile,
		entropy:      ulid.Monotonic(rand.Reader, 0),
		agentSeq:     make(map[string]int),
		done:         make(chan struct{}),
	}
	d.sync = syncer.New(g, syncer.Options{
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
	d.state = emptyState()

	if err := d.Refresh(); err != nil {
		if !isFailSafe(err) {
			return nil, fmt.Errorf("daemon: initial load: %w", err)
		}
		// T3 fail-safe: start anyway, serving reads of the last
		// comprehensible state (here: empty) with writes rejected.
		logger.Printf("daemon: starting in fail-safe read-only mode: %v", err)
	}

	sock := filepath.Join(dir, "daemon.sock")
	if len(sock) > maxSocketPath {
		return nil, fmt.Errorf("daemon: socket path %s exceeds the %d-byte unix socket limit", sock, maxSocketPath)
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
		Started: time.Now().UTC().Format(time.RFC3339),
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
// completes. Every exit path logs its reason.
func (d *Daemon) Run() error {
	d.log.Printf("daemon: pid %d serving %s", os.Getpid(), d.sockPath)
	go d.sync.Run()
	err := d.srv.Serve(d.ln)
	if !errors.Is(err, http.ErrServerClosed) {
		d.log.Printf("daemon: exiting: listener failed: %v", err)
		d.cleanup()
		return err
	}
	<-d.done // wait for Shutdown's cleanup
	return nil
}

// Shutdown stops serving, flushes pending events, and removes the
// socket and discovery file. Safe to call more than once.
func (d *Daemon) Shutdown(reason string) {
	d.shutdownOnce.Do(func() {
		d.log.Printf("daemon: exiting: %s", reason)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = d.srv.Shutdown(ctx)
		if err := d.batcher.Flush(); err != nil {
			d.log.Printf("daemon: final flush failed, events lost: %v", err)
		}
		d.sync.Stop()
		// Best-effort final push so a laptop closing its lid doesn't
		// strand the last few commits locally.
		if err := d.sync.Cycle(); err != nil {
			d.log.Printf("daemon: final sync: %v", err)
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

// Refresh recomputes cached state from the branch. The sync loop (B7)
// will call this after every fetch/merge.
func (d *Daemon) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.refreshLocked(time.Now())
}

// refreshLocked reloads events and leases and replays. On a fail-safe
// replay error the daemon degrades: the last good state keeps serving
// reads and every write path starts rejecting. Full replay per write is
// fine at v0 volumes; incremental apply is the known later optimization.
func (d *Daemon) refreshLocked(now time.Time) error {
	events, err := d.store.LoadEvents()
	if err != nil {
		return err
	}
	leases, err := d.store.ReadLeases()
	if err != nil {
		return err
	}
	st, err := d.replay.Replay(core.Input{
		Events: append(events, d.written...),
		Leases: leases,
		Now:    now,
	})
	if err != nil {
		if isFailSafe(err) {
			d.degraded = err
		}
		return err
	}
	d.degraded = nil
	d.state = st
	d.leases = leases
	return nil
}

// stageLocked records events in the overlay and the batcher. Caller
// holds d.mu.
func (d *Daemon) stageLocked(evs ...event.Event) {
	for _, e := range evs {
		d.written = append(d.written, e)
		d.batcher.Add(e)
	}
}

// commitLocked stages events, flushes immediately when eager (claim and
// escalation writes, T8 — everything else rides the debounce), and
// refreshes cached state. Caller holds d.mu.
func (d *Daemon) commitLocked(eager bool, evs ...event.Event) error {
	d.stageLocked(evs...)
	// Refresh before flushing so the views staged below render the state
	// these events produce; a fail-safe refresh skips views but must not
	// strand the events themselves.
	refreshErr := d.refreshLocked(time.Now())
	if refreshErr == nil {
		d.stageViewsLocked()
	}
	if eager {
		if err := d.batcher.Flush(); err != nil {
			return fmt.Errorf("daemon: flush: %w", err)
		}
		// Eager writes deserve eager wire time too (T8): ask the sync
		// loop for an immediate pass.
		d.sync.Poke()
	}
	return refreshErr
}

// stageViewsLocked renders the four views from current replayed state
// and stages them to ride the next batch commit (T6: views land
// alongside their events). Highest version wins (B8): views stamped by
// a newer generator are never overwritten — events still flow, and the
// newer peer keeps regenerating. Caller holds d.mu.
func (d *Daemon) stageViewsLocked() {
	meta, err := d.store.ReadFile(views.MetaPath)
	if err != nil {
		d.log.Printf("daemon: views: reading stamp: %v", err)
		return
	}
	if !views.CanWrite(meta) {
		d.log.Printf("daemon: views stamped by a newer tuhdoo (format %d > %d); writing events only",
			views.Format(meta), views.FormatVersion)
		return
	}
	d.batcher.AddFiles(views.Render(d.state))
}

// newEventLocked mints an event at the daemon layer (core stays pure):
// ULID from the monotonic entropy, machine from machine-id, actor from
// the request. Caller holds d.mu.
func (d *Daemon) newEventLocked(typ, actor, task string, payload any) (event.Event, error) {
	id, err := event.NewID(time.Now(), d.entropy)
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
		return nil, fmt.Errorf("daemon: another daemon is already running for this repo (%s); lock %s is held", livePID(dir), path)
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
