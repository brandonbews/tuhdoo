// Package store keeps tuhdoo's data — events, views, lease files — on
// the never-checked-out data branch (design docs 002 T2/T3, 001 D9).
//
// The Store is the live replica (001 D2, 2026-09-10): it holds the
// local head commit, the head's tree (path → blob OID), and the
// decoded events and leases behind that tree in memory. Reads never
// spawn git; git is touched only by Load (cold start, and the reload
// that follows a lost compare-and-swap or an external move of the
// ref) and by Commit. The Store is also the single mover of the data
// ref (002 T2): every commit — a local batch, the syncer's union
// merge, a fast-forward — goes through Commit or FastForward, so the
// replica and the ref advance together. Every write is one commit
// built from plumbing objects through the private index; the working
// tree is untouched by construction. The index belongs to the
// committing path alone: a load never touches it, and the first
// commit after any reload (or after a failed commit) reseeds it from
// the head before building on it.
package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
)

// DefaultRef is the data branch: a normal visible branch (T2), never
// checked out.
const DefaultRef = "refs/heads/tuhdoo"

// maxCASRetries bounds AppendBatch's retry loop when the ref moves
// under it. The daemon is the only local writer and the syncer commits
// through the same Store, so a lost compare-and-swap means the ref was
// moved from outside; more than a handful of consecutive losses means
// something is wrong enough to surface.
const maxCASRetries = 5

// ErrUndecodable marks a blob the store could read from git but could
// not decode as the event or lease its path says it is. Callers that
// must skip rather than fail on an unreadable head (the syncer's
// confirmation guard) branch on it; everything else fails, as T3 asks.
var ErrUndecodable = errors.New("store: blob does not decode")

// LeaseState is a decoded lease file: its expiry and whether it is a
// released tombstone (lease.go). Replay reads only the expiry; the
// syncer's merge rule needs the marker too.
type LeaseState struct {
	Expires  time.Time
	Released bool
}

// Store reads and writes the data branch of one repository.
type Store struct {
	git   gitx.Git
	ref   string
	ident gitx.Identity

	// mu guards everything below. Git subprocesses run under it too:
	// writes to the branch stay serialized, and a read that arrives
	// during a commit waits for the head and tree to move together.
	mu sync.Mutex

	// loaded is true once head and tree reflect git. A Store loads
	// itself on first use; the daemon loads explicitly at startup so
	// every later read is answered from memory.
	loaded bool
	// head is the local ref's commit OID — the parent of the next
	// commit and the expected old value of its compare-and-swap.
	head string
	// tree is the head commit's tree: path → blob OID for every file
	// on the branch. The private index (T2) mirrors it at every commit.
	tree map[string]string
	// indexStale is true whenever the private index is not known to
	// hold exactly tree: at construction, after every load or reload
	// (a load never touches the index — a read-only Load beside a
	// running daemon must not rewrite the daemon's index), and after
	// any commit attempt that failed past UpdateIndex (the index may
	// hold that attempt's entries). The next commit reseeds from the
	// head first (ReadTree, which also clears a stale index.lock) and
	// only then builds on it.
	indexStale bool

	// The decode caches are content-addressed — the bytes behind an
	// OID can never change — so a cached decode never goes stale.
	// eventByOID is insert-only: events are immutable and never leave
	// the tree, so it is bounded by the event log. Cached events are
	// shared across callers and must be treated as read-only (replay
	// already does: upcasters return fresh payload bytes). leaseByOID
	// and fileByOID hold mutable files — every lease renewal and every
	// view render is a new blob — so both are pruned to the OIDs in
	// the head tree whenever the head moves, and cannot grow with
	// churn. fileByOID serves ReadFile and Blob (the view-format stamp
	// is the one file read back in production).
	eventByOID map[string]event.Event
	leaseByOID map[string]LeaseState
	fileByOID  map[string][]byte
}

// New returns a Store writing to ref as ident. An empty ref means
// DefaultRef. Nothing is read until Load or first use.
func New(g gitx.Git, ref string, ident gitx.Identity) *Store {
	if ref == "" {
		ref = DefaultRef
	}
	return &Store{
		git:        g,
		ref:        ref,
		ident:      ident,
		indexStale: true,
		eventByOID: make(map[string]event.Event),
		leaseByOID: make(map[string]LeaseState),
		fileByOID:  make(map[string][]byte),
	}
}

// Batch is one commit's worth of changes. Events are stored at their
// event.Path() locations; Files carries arbitrary paths (views, lease
// files).
type Batch struct {
	Events []event.Event
	Files  map[string][]byte
}

func (b Batch) empty() bool {
	return len(b.Events) == 0 && len(b.Files) == 0
}

// Init creates the orphan data branch if it does not exist: a parentless
// root commit on the empty tree. If the branch already exists — including
// when a concurrent init wins the creation race — Init is a no-op.
func (s *Store) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.git.ReadRef(s.ref)
	if err == nil {
		return nil
	}
	if !errors.Is(err, gitx.ErrRefNotFound) {
		return fmt.Errorf("store: init: %w", err)
	}

	tree, err := s.git.MkTree(nil)
	if err != nil {
		return fmt.Errorf("store: init: %w", err)
	}
	commit, err := s.git.CommitTree(tree, nil, s.ident, "tuhdoo: init data branch\n")
	if err != nil {
		return fmt.Errorf("store: init: %w", err)
	}
	// Must-not-exist compare-and-swap, through the one ref mover.
	err = s.moveRefLocked(commit, "")
	if errors.Is(err, gitx.ErrRefCASFailed) {
		// Someone else created the branch first; their root is as good
		// as ours.
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: init: %w", err)
	}
	return nil
}

// Load reads the head and its tree from git: one rev-parse, one
// ls-tree, plus one batched cat-file for whichever event and lease
// blobs the caches lack (none, on a warm reload). It is the cold start
// and the reload path — a lost compare-and-swap inside a commit, an
// external move of the ref noticed by the sync cycle — and the only
// reader of the ref outside those two. It never touches the private
// index: a Load may run in a process that only reads (a harness
// beside a running daemon), and the index is the committing daemon's.
// The head it installs marks the index stale, so this Store's next
// commit reseeds before building.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// loadLocked is Load with s.mu held. The caches are filled before the
// head is installed, so a load that fails leaves the previous replica
// serving.
func (s *Store) loadLocked() error {
	head, err := s.git.ReadRef(s.ref)
	if err != nil {
		return fmt.Errorf("store: load %s (run Init first?): %w", s.ref, err)
	}
	tree, err := gitx.LsTreeMap(s.git, head)
	if err != nil {
		return fmt.Errorf("store: load: %w", err)
	}
	if _, _, err := s.replayInputLocked(tree); err != nil {
		return fmt.Errorf("store: load: %w", err)
	}
	s.installHeadLocked(head, tree)
	// Whatever the index held, it was seeded from some other head (or
	// by some other process, or never): the next commit reseeds.
	s.indexStale = true
	return nil
}

// ensureLoadedLocked loads on first use. Caller holds s.mu.
func (s *Store) ensureLoadedLocked() error {
	if s.loaded {
		return nil
	}
	return s.loadLocked()
}

// installHeadLocked makes head/tree the replica's head and prunes the
// mutable-blob caches to what the new tree references. Caller holds
// s.mu and owns indexStale: a commit that built the index into exactly
// tree leaves it false; a load sets it true.
func (s *Store) installHeadLocked(head string, tree map[string]string) {
	s.head = head
	s.tree = tree
	s.loaded = true
	live := make(map[string]bool, len(tree))
	for _, oid := range tree {
		live[oid] = true
	}
	for oid := range s.leaseByOID {
		if !live[oid] {
			delete(s.leaseByOID, oid)
		}
	}
	for oid := range s.fileByOID {
		if !live[oid] {
			delete(s.fileByOID, oid)
		}
	}
}

// Head returns the local head commit OID as the replica knows it, or
// "" before the first load.
func (s *Store) Head() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.head
}

// Tree returns a copy of the head tree (path → blob OID). Empty before
// the first load.
func (s *Store) Tree() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return copyTree(s.tree)
}

// HeadAndTree returns Head and Tree from one instant: the tree is the
// head's tree, whatever moves after.
func (s *Store) HeadAndTree() (string, map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.head, copyTree(s.tree)
}

func copyTree(tree map[string]string) map[string]string {
	out := make(map[string]string, len(tree))
	for path, oid := range tree {
		out[path] = oid
	}
	return out
}

// Changed returns the subset of files whose bytes are not already at
// their path in the head tree — the object-ID diff of T2 (2026-09-10)
// without writing anything. The daemon runs every re-render through it
// (T6: views are staged only when their bytes change), so an unchanged
// page costs a hash in memory and nothing else. Every file is hashed;
// no git is spawned. Empty before the first load (nothing is known to
// be unchanged).
func (s *Store) Changed(files map[string][]byte) map[string][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]byte)
	for path, data := range files {
		if s.tree[path] != s.git.BlobOID(data) {
			out[path] = data
		}
	}
	return out
}

// WriteBlobs computes each file's object ID in memory and writes only
// the blobs whose OID is not already at that path in against — the
// object-ID diff of T2 (2026-09-10): a rendered view or a lease whose
// bytes did not change costs nothing, a genuinely new blob costs one
// `hash-object -w`. Returns path → OID for every file, written or not.
// The syncer's merge re-render goes through here too, against the
// merged tree, so it writes only the pages a merge actually changed.
func (s *Store) WriteBlobs(files map[string][]byte, against map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(files))
	// Deterministic order: a failure names the same path every time,
	// and the subprocess sequence is reproducible.
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		data := files[path]
		oid := s.git.BlobOID(data)
		if against[path] == oid {
			out[path] = oid
			continue
		}
		written, err := s.git.HashObject(data)
		if err != nil {
			return nil, fmt.Errorf("store: write %s: %w", path, err)
		}
		if written != oid {
			// BlobOID and git disagree about this repository's object
			// format: nothing committed on that basis can be trusted.
			return nil, fmt.Errorf("store: write %s: computed object id %s but git wrote %s", path, oid, written)
		}
		out[path] = oid
	}
	return out, nil
}

// Commit is the one path that advances the data ref. base is the head
// the caller computed changes against — the anchor, exactly as
// FastForward's from: if the replica's head is no longer base (a local
// batch landed since the caller looked), the commit is refused before
// git is touched with an error matching gitx.ErrRefCASFailed. changes
// is path → blob OID, or "" to delete the path; the new tree is the
// head tree with changes applied, built through the private index from
// only the paths that actually differ from the head tree, then written
// as a commit whose first parent is the head and whose remaining
// parents are extraParents (the syncer's merge passes the remote
// head), and the ref is moved by compare-and-swap from base. On
// success head and tree are replaced in memory and the new commit's
// OID returned. A lost compare-and-swap at the ref — moved from
// outside this Store — reloads head, tree, and caches from git and
// returns an error matching gitx.ErrRefCASFailed: Commit itself never
// retries, because changes were computed against base and a merge
// recomputed against the new head is a different merge (the syncer
// simply runs its next pass). AppendBatch, whose events and leases are
// valid against any head, owns the retry loop.
func (s *Store) Commit(base string, changes map[string]string, extraParents []string, message string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return "", fmt.Errorf("store: commit: %w", err)
	}
	if s.head != base {
		return "", fmt.Errorf("store: commit on %s: replica head is %s: %w", base, s.head, gitx.ErrRefCASFailed)
	}
	return s.commitLocked(changes, extraParents, message)
}

// commitLocked is one commit attempt on the replica's head, with s.mu
// held and the Store loaded. It validates changes against the head
// tree before any git runs, reseeds the private index from the head
// when it is stale (ReadTree: the one place a leftover index.lock is
// cleared — the committing daemon holds the flock), feeds the index
// the diff, writes the tree and the commit, and moves the ref by
// compare-and-swap from the head. An index that turns out to be
// missing at UpdateIndex is reseeded and the attempt retried once.
// Any failure after the index was touched marks it stale, so no later
// commit builds on entries from a commit that never landed; a lost
// compare-and-swap additionally reloads the replica from the ref's
// new position and returns an error matching gitx.ErrRefCASFailed.
func (s *Store) commitLocked(changes map[string]string, extraParents []string, message string) (string, error) {
	tree, entries, err := applyChanges(s.tree, changes)
	if err != nil {
		return "", fmt.Errorf("store: commit: %w", err)
	}
	reseeded := false
	for {
		if s.indexStale {
			if err := s.git.ReadTree(s.head); err != nil {
				return "", fmt.Errorf("store: commit: reseed private index: %w", err)
			}
			s.indexStale = false
		}
		err := s.git.UpdateIndex(entries)
		if err == nil {
			break
		}
		s.indexStale = true
		if errors.Is(err, gitx.ErrIndexMissing) && !reseeded {
			reseeded = true
			continue
		}
		return "", fmt.Errorf("store: commit: %w", err)
	}
	treeOID, err := s.git.WriteTree()
	if err != nil {
		s.indexStale = true
		return "", fmt.Errorf("store: commit: %w", err)
	}
	parents := append([]string{s.head}, extraParents...)
	commit, err := s.git.CommitTree(treeOID, parents, s.ident, message)
	if err != nil {
		s.indexStale = true
		return "", fmt.Errorf("store: commit: %w", err)
	}

	err = s.git.UpdateRef(s.ref, commit, s.head)
	if err == nil {
		// The index now holds exactly tree: it was seeded from the old
		// head and fed the diff to the new one.
		s.installHeadLocked(commit, tree)
		return commit, nil
	}
	s.indexStale = true
	if !errors.Is(err, gitx.ErrRefCASFailed) {
		return "", fmt.Errorf("store: commit: %w", err)
	}
	// The ref moved from outside: the replica follows it, so the
	// caller's retry (or the syncer's next pass) starts from the truth.
	// A reload that fails is not a compare-and-swap loss to retry on —
	// the replica is now behind a ref it cannot read — so it is
	// reported as itself.
	if lerr := s.loadLocked(); lerr != nil {
		return "", fmt.Errorf("store: commit: after lost compare-and-swap (%v), reload failed: %w", err, lerr)
	}
	return "", fmt.Errorf("store: commit: %w", err)
}

// applyChanges returns base with changes applied, and the index
// entries that turn base into it: only paths whose OID differs (adds,
// replacements) or that base holds and changes deletes. Pure.
//
// It refuses a change set whose result is not a tree: a blob at a
// path that is a directory of another path in the result ("events"
// beside "events/a"), or under a path that holds a blob ("views/t/x"
// under a blob "views/t"). `update-index --index-info` would accept
// either and silently drop the entries that stand in the way — a blob
// at "events" wipes every event — so the check runs here, against the
// resulting tree, before any git does.
func applyChanges(base, changes map[string]string) (map[string]string, []gitx.TreeEntry, error) {
	tree := copyTree(base)
	var entries []gitx.TreeEntry
	var added []string
	for path, oid := range changes {
		if oid == "" {
			if _, present := base[path]; present {
				delete(tree, path)
				entries = append(entries, gitx.TreeEntry{Path: path})
			}
			continue
		}
		added = append(added, path)
		if base[path] == oid {
			continue
		}
		tree[path] = oid
		entries = append(entries, gitx.TreeEntry{Path: path, OID: oid})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	sort.Strings(added)
	if err := checkTreeShape(tree, added); err != nil {
		return nil, nil, err
	}
	return tree, entries, nil
}

// checkTreeShape reports the first file-versus-directory conflict an
// added path has with the rest of tree: every directory on the way to
// an added blob must not itself hold a blob, and no added blob may sit
// where another path needs a directory. base trees come from git or
// from a previous check, so only the added paths can be at fault.
func checkTreeShape(tree map[string]string, added []string) error {
	if len(added) == 0 {
		return nil
	}
	// Every directory the tree needs, each with the lexically first
	// path that needs it (so the error names the same path every time).
	dirs := make(map[string]string)
	for path := range tree {
		for dir := parentDir(path); dir != ""; dir = parentDir(dir) {
			if prev, seen := dirs[dir]; !seen || path < prev {
				dirs[dir] = path
			}
		}
	}
	for _, path := range added {
		if under, needed := dirs[path]; needed {
			return fmt.Errorf("blob at %q conflicts with %q, which needs it as a directory", path, under)
		}
		for dir := parentDir(path); dir != ""; dir = parentDir(dir) {
			if tree[dir] != "" {
				return fmt.Errorf("blob at %q conflicts with the blob at %q on its path", path, dir)
			}
		}
	}
	return nil
}

// parentDir is the directory part of a slash-separated path, or ""
// for a top-level path.
func parentDir(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return ""
	}
	return path[:i]
}

// FastForward moves the ref from the head the caller saw, from, to
// newHead by compare-and-swap, then reloads the replica from git so
// head, tree, and caches all reflect the adopted commit (the index is
// left to the next commit, which reseeds from it). from
// is the caller's anchor on purpose — the syncer decided newHead was a
// descendant of the head it read, and a local commit landing since
// makes that decision stale: the replica then already holds a newer
// head, and the move is refused with an error matching
// gitx.ErrRefCASFailed before git is touched, exactly as a ref moved
// from outside is refused by git itself (and, in that case, after the
// same reload, so the replica is fresh either way). from is "" before
// the first load: the ref must not exist yet — the clone-join adopt.
func (s *Store) FastForward(from, newHead string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.head != from {
		return fmt.Errorf("store: fast-forward from %s: replica head is %s: %w", from, s.head, gitx.ErrRefCASFailed)
	}
	return s.moveRefLocked(newHead, from)
}

// moveRefLocked is the ref move behind FastForward and Init: the
// compare-and-swap from old, followed by a reload from git. Caller
// holds s.mu.
func (s *Store) moveRefLocked(newHead, old string) error {
	casErr := s.git.UpdateRef(s.ref, newHead, old)
	if casErr != nil && !errors.Is(casErr, gitx.ErrRefCASFailed) {
		return fmt.Errorf("store: move %s: %w", s.ref, casErr)
	}
	if err := s.loadLocked(); err != nil {
		if casErr != nil {
			return fmt.Errorf("store: move %s: %w (and reload failed: %v)", s.ref, casErr, err)
		}
		return fmt.Errorf("store: move %s: %w", s.ref, err)
	}
	if casErr != nil {
		return fmt.Errorf("store: move %s: %w", s.ref, casErr)
	}
	return nil
}

// AppendBatch commits b to the data branch as exactly one commit: the
// head tree plus b's changes, parented on the head. Blobs are written
// only for events and files whose bytes are not already at their path
// (WriteBlobs); the decoded forms go into the caches once the commit
// lands, so the next ReplayInput needs no git. An empty batch is a
// no-op.
//
// A lost compare-and-swap — the ref moved from outside this Store —
// has reloaded the replica; AppendBatch then retries on the new head,
// bounded by maxCASRetries (exhaustion is an error), with the batch
// cut down to its events and leases: those are valid against any
// head, but the rendered files were computed against a state the
// reload has superseded, and landing them now could overwrite pages a
// newer generator stamped on the moved head (T6: highest stamp wins).
// The daemon's next refresh re-renders from the merged state.
func (s *Store) AppendBatch(b Batch) error {
	if b.empty() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return fmt.Errorf("store: append: %w", err)
	}

	files := make(map[string][]byte, len(b.Events)+len(b.Files))
	eventAt := make(map[string]event.Event, len(b.Events)) // path → event, for the cache
	for _, e := range b.Events {
		path, err := event.Path(e.ID)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}
		data, err := event.Encode(e)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}
		files[path] = data
		eventAt[path] = e
	}
	// Decode the leases up front, the way a load would: a lease file
	// that does not decode is the caller's bug, caught here rather
	// than at the next load — and before anything is written.
	leaseAt := make(map[string]LeaseState) // path → state, for the cache
	for path, data := range b.Files {
		files[path] = data
		if _, ok := LeaseClaimID(path); !ok {
			continue
		}
		expires, released, err := DecodeLeaseState(data)
		if err != nil {
			return fmt.Errorf("store: append %s: %w", path, err)
		}
		leaseAt[path] = LeaseState{Expires: expires, Released: released}
	}

	msg := fmt.Sprintf("tuhdoo: %d events, %d files\n", len(b.Events), len(b.Files))
	for attempt := 0; ; attempt++ {
		changes, err := s.WriteBlobs(files, s.tree)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}
		_, err = s.commitLocked(changes, nil, msg)
		if err == nil {
			// Cache after the fact, not before: the reload behind a
			// lost compare-and-swap prunes the lease cache to a tree
			// that does not hold this batch yet.
			for path, e := range eventAt {
				s.eventByOID[changes[path]] = e
			}
			for path, st := range leaseAt {
				s.leaseByOID[changes[path]] = st
			}
			return nil
		}
		if !errors.Is(err, gitx.ErrRefCASFailed) {
			return fmt.Errorf("store: append: %w", err)
		}
		if attempt+1 >= maxCASRetries {
			return fmt.Errorf("store: append: ref %s kept moving after %d attempts: %w",
				s.ref, maxCASRetries, err)
		}
		// Retry with events and leases only (see above); the blob OIDs
		// are recomputed against the reloaded tree.
		for path := range files {
			if _, isEvent := eventAt[path]; isEvent {
				continue
			}
			if _, isLease := leaseAt[path]; isLease {
				continue
			}
			delete(files, path)
		}
	}
}

// ReadFile returns the blob at path in the head tree, or nil when the
// path does not exist there. The tree is read from memory; the bytes
// come from the file cache, or from git once per distinct blob.
func (s *Store) ReadFile(path string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}
	oid, ok := s.tree[path]
	if !ok {
		return nil, nil
	}
	data, err := s.blobLocked(oid)
	if err != nil {
		return nil, fmt.Errorf("store: read %s: %w", path, err)
	}
	return data, nil
}

// Blob returns the bytes of the blob at oid, from the file cache or
// git. For files that are neither events nor leases — the syncer reads
// both sides' view-format stamps through it; the cache is pruned to
// the head tree whenever the head moves.
func (s *Store) Blob(oid string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blobLocked(oid)
}

func (s *Store) blobLocked(oid string) ([]byte, error) {
	if data, ok := s.fileByOID[oid]; ok {
		return data, nil
	}
	data, err := s.git.CatFile(oid)
	if err != nil {
		return nil, err
	}
	s.fileByOID[oid] = data
	return data, nil
}

// LoadEvents returns every event at the head, in path order (which is
// ULID-date order), from memory.
func (s *Store) LoadEvents() ([]event.Event, error) {
	events, _, err := s.ReplayInput()
	return events, err
}

// LoadReplayInput is Load followed by ReplayInput: the one-shot read
// for a process that holds no replica of its own (a harness, a
// one-off report). The daemon calls Load once and ReplayInput after.
func (s *Store) LoadReplayInput() ([]event.Event, map[string]time.Time, error) {
	if err := s.Load(); err != nil {
		return nil, nil, err
	}
	return s.ReplayInput()
}

// ReplayInput returns everything replay consumes at the head: events
// under events/ (path order, which is ULID-date order) and leases
// under leases/ keyed by claim id. Answered from memory — the head
// tree and the decode caches — which Load and Commit keep complete;
// should a blob nonetheless be uncached, it is read from git in one
// batch rather than failing.
func (s *Store) ReplayInput() ([]event.Event, map[string]time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, nil, err
	}
	return s.replayInputLocked(s.tree)
}

// ReplayInputFor is ReplayInput for an arbitrary tree map — the other
// side of a merge, a merged tree no ref points at yet — served from
// the same caches, so a replay after a fetch reads only the blobs the
// replica has never seen (the other machine's new events and leases),
// in one batch, and caches them for the commit that follows.
func (s *Store) ReplayInputFor(tree map[string]string) ([]event.Event, map[string]time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replayInputLocked(tree)
}

// EventsByOID returns the decoded events behind oids, keyed by OID:
// cached ones from memory, the rest read from git in one batch and
// cached. A blob that does not decode as an event fails with an error
// matching ErrUndecodable.
func (s *Store) EventsByOID(oids []string) (map[string]event.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var need []string
	for _, oid := range oids {
		if _, ok := s.eventByOID[oid]; !ok {
			need = append(need, oid)
		}
	}
	if err := s.fetchLocked(need, nil); err != nil {
		return nil, err
	}
	out := make(map[string]event.Event, len(oids))
	for _, oid := range oids {
		out[oid] = s.eventByOID[oid]
	}
	return out, nil
}

// LeasesByOID returns the decoded lease files behind oids, keyed by
// OID, the same way EventsByOID serves events.
func (s *Store) LeasesByOID(oids []string) (map[string]LeaseState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var need []string
	for _, oid := range oids {
		if _, ok := s.leaseByOID[oid]; !ok {
			need = append(need, oid)
		}
	}
	if err := s.fetchLocked(nil, need); err != nil {
		return nil, err
	}
	out := make(map[string]LeaseState, len(oids))
	for _, oid := range oids {
		out[oid] = s.leaseByOID[oid]
	}
	return out, nil
}

// replayInputLocked is the one reader of replay input: it classifies
// the tree — events under events/, leases under leases/ with a
// well-formed claim id; everything else (rendered views, paths no
// writer produces) is skipped — reads through git in one batch only
// the blobs the caches lack, decodes them into the caches, and returns
// the events in path order plus the leases keyed by claim id. The
// store's head load and the syncer's merge-time replays all go through
// here, so every reader of a tree computes the same event list and
// lease set by construction. A blob git cannot produce fails the whole
// read (CatFiles answers every OID it was asked for or errors), never
// a skipped entry. Caller holds s.mu.
func (s *Store) replayInputLocked(tree map[string]string) ([]event.Event, map[string]time.Time, error) {
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var eventNeed, leaseNeed []string
	for _, path := range paths {
		oid := tree[path]
		switch {
		case strings.HasPrefix(path, "events/"):
			if _, ok := s.eventByOID[oid]; !ok {
				eventNeed = append(eventNeed, oid)
			}
		case strings.HasPrefix(path, "leases/"):
			if _, ok := LeaseClaimID(path); !ok {
				continue
			}
			if _, ok := s.leaseByOID[oid]; !ok {
				leaseNeed = append(leaseNeed, oid)
			}
		}
	}
	if err := s.fetchLocked(eventNeed, leaseNeed); err != nil {
		return nil, nil, err
	}

	var events []event.Event
	leases := make(map[string]time.Time)
	for _, path := range paths {
		oid := tree[path]
		switch {
		case strings.HasPrefix(path, "events/"):
			events = append(events, s.eventByOID[oid])
		case strings.HasPrefix(path, "leases/"):
			claimID, ok := LeaseClaimID(path)
			if !ok {
				continue
			}
			leases[claimID] = s.leaseByOID[oid].Expires
		}
	}
	return events, leases, nil
}

// fetchLocked reads the named event and lease blobs from git in one
// batch and decodes them into the caches. Nothing is spawned when both
// lists are empty. A decode failure is an ErrUndecodable naming the
// blob. Caller holds s.mu.
func (s *Store) fetchLocked(eventOIDs, leaseOIDs []string) error {
	if len(eventOIDs) == 0 && len(leaseOIDs) == 0 {
		return nil
	}
	blobs, err := s.git.CatFiles(append(append([]string{}, eventOIDs...), leaseOIDs...))
	if err != nil {
		return err
	}
	for _, oid := range eventOIDs {
		e, err := event.Decode(blobs[oid])
		if err != nil {
			return fmt.Errorf("event blob %s: %w: %w", oid, ErrUndecodable, err)
		}
		s.eventByOID[oid] = e
	}
	for _, oid := range leaseOIDs {
		expires, released, err := DecodeLeaseState(blobs[oid])
		if err != nil {
			return fmt.Errorf("lease blob %s: %w: %w", oid, ErrUndecodable, err)
		}
		s.leaseByOID[oid] = LeaseState{Expires: expires, Released: released}
	}
	return nil
}
