// Package gitx wraps the git command-line tool behind a small interface.
//
// tuhdoo's data branch is never checked out (design doc 002, T2): every
// method works on git's object database and refs directly, via plumbing
// commands. The interface deliberately has no checkout, worktree, or
// index operations, so code built on it cannot touch the user's working
// tree even by accident.
package gitx

import "errors"

// TreeEntry is one file inside a tree: a slash-separated path (never
// starting with "/") and the object ID of its blob. Used both to build
// trees (MkTree) and to list them (LsTree).
type TreeEntry struct {
	Path string
	OID  string
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

	// CatFile returns the exact bytes of the blob at oid.
	CatFile(oid string) (data []byte, err error)

	// LsTree lists every blob reachable from rev (a tree or commit OID,
	// or a ref) as path → blob OID, recursing into subtrees.
	LsTree(rev string) ([]TreeEntry, error)

	// Fetch fetches refspec from remote.
	Fetch(remote, refspec string) error

	// Push pushes refspec to remote. A non-fast-forward rejection
	// returns an error matching ErrNonFastForward. There is no force
	// option, by design: force-pushing the data branch is forbidden.
	Push(remote, refspec string) error

	// RemoteURL returns the URL configured for remote, or "" and an
	// error matching ErrNoRemote when it is not configured.
	RemoteURL(remote string) (string, error)
}
