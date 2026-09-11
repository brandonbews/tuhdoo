// Package gitx wraps the git command-line tool behind a small interface.
//
// tuhdoo's data branch is never checked out (design doc 002, T2): every
// method works on git's object database and refs directly, via plumbing
// commands. The interface deliberately has no checkout or worktree
// operations, so code built on it cannot touch the user's working tree
// even by accident. The only index it touches is tuhdoo's own private
// one (T2, 2026-09-10: `<git-dir>/tuhdoo/index`, selected per
// subprocess through GIT_INDEX_FILE) — the user's index is never read
// or written, and a private index is not a worktree.
package gitx

import (
	"errors"
	"sort"
	"time"
)

// TreeEntry is one file inside a tree: a slash-separated path (never
// starting with "/") and the object ID of its blob. Used both to build
// trees (MkTree) and to list them (LsTree).
type TreeEntry struct {
	Path string
	OID  string
}

// MkTreeFromMap builds a tree from a path → blob-OID map via g.MkTree,
// with entries sorted by path.
func MkTreeFromMap(g Git, files map[string]string) (string, error) {
	entries := make([]TreeEntry, 0, len(files))
	for path, oid := range files {
		entries = append(entries, TreeEntry{Path: path, OID: oid})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return g.MkTree(entries)
}

// LsTreeMap is g.LsTree as a path → blob-OID map — the inverse of
// MkTreeFromMap, and the one shape every tree reader (the store's
// load, the syncer's merge) works in.
func LsTreeMap(g Git, rev string) (map[string]string, error) {
	entries, err := g.LsTree(rev)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Path] = e.OID
	}
	return m, nil
}

// Identity names the author and committer of a commit. gitx never reads
// the user's git config, so callers must always say who is committing.
type Identity struct {
	Name  string
	Email string
}

// Typed failures callers branch on with errors.Is.
var (
	// ErrRefCASFailed reports that UpdateRef found the ref was not at
	// the expected old value — someone else moved it first.
	ErrRefCASFailed = errors.New("ref not at expected value")

	// ErrRefNotFound reports that ReadRef was asked for a ref that does
	// not exist.
	ErrRefNotFound = errors.New("ref does not exist")

	// ErrNonFastForward reports a push rejected because the remote ref
	// has commits we do not have. Fetch, merge, then push again.
	ErrNonFastForward = errors.New("push rejected: non-fast-forward")

	// ErrNoRemote reports that the asked-for remote is not configured.
	// Remoteless operation is a normal state (T2), not a crash.
	ErrNoRemote = errors.New("remote not configured")

	// ErrRemoteRefMissing reports a fetch of a ref the remote does not
	// have — normal before the first push of the data branch.
	ErrRemoteRefMissing = errors.New("remote does not have the ref")

	// ErrIndexMissing reports that UpdateIndex (or WriteTree) found no
	// private index file to work on: something removed it since the
	// last ReadTree. The caller reseeds and retries; git itself would
	// silently create an empty index and commit the empty tree.
	ErrIndexMissing = errors.New("private index file is missing")
)

// Git is the only door to git for the rest of tuhdoo.
type Git interface {
	// HashObject writes data to the object database as a blob and
	// returns its OID.
	HashObject(data []byte) (oid string, err error)

	// MkTree builds a tree object from blob entries and returns its
	// OID. Entries may use nested paths ("events/2026/07/29/x.json");
	// subtrees are built as needed. An empty entries list yields the
	// empty tree, which is how the orphan branch's root commit starts.
	MkTree(entries []TreeEntry) (oid string, err error)

	// CommitTree creates a commit object for treeOID with zero, one, or
	// two parents and returns its OID. Zero parents is the orphan root;
	// two parents is tuhdoo's app-level merge commit.
	CommitTree(treeOID string, parentOIDs []string, ident Identity, message string) (oid string, err error)

	// UpdateRef atomically points ref at newOID, but only if the ref is
	// currently at oldOID (compare-and-swap). Pass oldOID == "" to
	// require that the ref does not exist yet. Losing the race returns
	// an error matching ErrRefCASFailed.
	UpdateRef(ref, newOID, oldOID string) error

	// ReadRef resolves a fully-qualified ref ("refs/heads/tuhdoo") to
	// the OID it points at. A missing ref returns an error matching
	// ErrRefNotFound.
	ReadRef(ref string) (oid string, err error)

	// CatFile returns the exact bytes of the blob at oid — a full hex
	// object name exactly as git prints it (a tree listing, a ref
	// read), never an abbreviation or a ref. It is CatFiles for one
	// object: the batch format is parsed in exactly one place.
	CatFile(oid string) (data []byte, err error)

	// CatFiles returns the exact bytes of every blob named in oids,
	// keyed by OID, read through one `git cat-file --batch` process
	// (T2, 2026-09-10: reads are batched — a cold start is one process
	// for the whole tree, not one per blob). Each oid is a full hex
	// object name exactly as git prints it: the batch parser matches
	// git's answer records to the requests by name, so an abbreviated
	// or ref-form name would be answered under a different string and
	// fail the read. Duplicate OIDs are read once. An OID the
	// repository lacks is an error naming it, never a silently absent
	// key (T3's fail-safe posture: a load that cannot read an object
	// must fail, not skip it); an OID naming a non-blob is an error
	// too. Empty input spawns nothing and returns an empty map.
	CatFiles(oids []string) (map[string][]byte, error)

	// BlobOID returns the object ID git would assign data as a blob —
	// the hash of "blob <len>\0<bytes>" in the repository's object
	// format (SHA-1 or SHA-256) — without writing anything. Local OIDs
	// exist to diff in-memory bytes against a tree (T2, 2026-09-10);
	// HashObject remains the only way an object is written.
	BlobOID(data []byte) string

	// LsTree lists every blob reachable from rev (a tree or commit OID,
	// or a ref) as path → blob OID, recursing into subtrees.
	LsTree(rev string) ([]TreeEntry, error)

	// ReadTree seeds the private index with the contents of tree (a
	// tree or commit OID), replacing whatever the index held — a
	// missing index file is created, a garbage one overwritten, and a
	// stale index.lock cleared first (the daemon's flock proves no
	// other process writes this index, so a lock file is a leftover of
	// a killed git). T2, 2026-09-10: the index mirrors the parent tree
	// of the next commit, so the committing process reseeds before the
	// first commit after any reload of the head from git; a tree built
	// from an index not reseeded after a reload silently drops the
	// other side's files. Only a committing process may call it: a
	// read-only load must not rewrite a running daemon's index.
	ReadTree(tree string) error

	// UpdateIndex applies entries to the private index in one
	// `update-index --index-info` process: an entry with an OID adds or
	// replaces the regular blob (mode 100644) at its path; an entry
	// with an empty OID removes the path (a no-op when absent — the
	// mode-0 line D9 compaction will use). Paths are validated exactly
	// as MkTree validates them. The index file must already exist: its
	// absence is an error matching ErrIndexMissing, checked before
	// anything runs, never an index silently recreated empty (git
	// would; the resulting tree would drop every file on the branch).
	// Empty entries spawn nothing (the existence check still runs).
	UpdateIndex(entries []TreeEntry) error

	// WriteTree writes the private index out as a tree object and
	// returns its OID. A missing index file is an error matching
	// ErrIndexMissing, never the empty tree git would otherwise
	// silently produce: the index must have been seeded by ReadTree in
	// this process.
	WriteTree() (oid string, err error)

	// Fetch fetches refspec from remote. A refspec naming a ref the
	// remote lacks returns an error matching ErrRemoteRefMissing.
	// Refspecs are used unforced on purpose: the data branch never
	// rewinds (no-force-push law), so a non-fast-forward fetch is
	// corruption and must fail loudly.
	Fetch(remote, refspec string) error

	// FetchTimeout is Fetch bounded by a wall-clock deadline: the git
	// subprocess is killed once timeout elapses. Startup paths use it so
	// an unreachable remote cannot hang the daemon — git's own timeout
	// knobs are transport-specific and unreliable. The sync loop keeps
	// the unbounded Fetch: there, a slow first transfer of a large
	// branch is legitimate work, not a hang.
	FetchTimeout(remote, refspec string, timeout time.Duration) error

	// IsAncestor reports whether commit a is an ancestor of (or equal
	// to) commit b.
	IsAncestor(a, b string) (bool, error)

	// Push pushes refspec to remote. A non-fast-forward rejection
	// returns an error matching ErrNonFastForward. There is no force
	// option, by design: force-pushing the data branch is forbidden.
	Push(remote, refspec string) error

	// RemoteURL returns the URL configured for remote, or "" and an
	// error matching ErrNoRemote when it is not configured.
	RemoteURL(remote string) (string, error)
}
