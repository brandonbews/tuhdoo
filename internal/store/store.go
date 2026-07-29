// Package store keeps tuhdoo's data — events, views, lease files — on
// the never-checked-out data branch (design docs 002 T2/T3, 001 D9).
// Every write is one commit built from plumbing objects via gitx; the
// working tree is untouched by construction.
package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
)

// DefaultRef is the data branch: a normal visible branch (T2), never
// checked out.
const DefaultRef = "refs/heads/tuhdoo"

// maxCASRetries bounds AppendBatch's retry loop when the ref moves under
// it. The daemon is the only local writer, but the sync loop may also
// advance the ref; more than a handful of consecutive losses means
// something is wrong enough to surface.
const maxCASRetries = 5

// Store reads and writes the data branch of one repository.
type Store struct {
	git   gitx.Git
	ref   string
	ident gitx.Identity
}

// New returns a Store writing to ref as ident. An empty ref means
// DefaultRef.
func New(g gitx.Git, ref string, ident gitx.Identity) *Store {
	if ref == "" {
		ref = DefaultRef
	}
	return &Store{git: g, ref: ref, ident: ident}
}

// Batch is one commit's worth of changes. Events are stored at their
// event.Path() locations; Files carries arbitrary paths (views, lease
// files); Delete removes paths. Deletes are applied after writes, so a
// path both written and deleted in one batch ends up absent.
type Batch struct {
	Events []event.Event
	Files  map[string][]byte
	Delete []string
}

func (b Batch) empty() bool {
	return len(b.Events) == 0 && len(b.Files) == 0 && len(b.Delete) == 0
}

// Init creates the orphan data branch if it does not exist: a parentless
// root commit on the empty tree. If the branch already exists — including
// when a concurrent init wins the creation race — Init is a no-op.
func (s *Store) Init() error {
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
	err = s.git.UpdateRef(s.ref, commit, "")
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

// AppendBatch commits b to the data branch as exactly one commit: the
// current tree plus b's changes, parented on the current head, advanced
// with a compare-and-swap. If the ref moves underneath us the batch is
// rebuilt on the new head and retried (bounded). An empty batch is a
// no-op.
func (s *Store) AppendBatch(b Batch) error {
	if b.empty() {
		return nil
	}

	// Hash all new blobs once; hashing is idempotent, so this can live
	// outside the retry loop.
	changed := make(map[string]string) // path → blob OID
	for _, e := range b.Events {
		path, err := event.Path(e.ID)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}
		data, err := event.Encode(e)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}
		oid, err := s.git.HashObject(data)
		if err != nil {
			return fmt.Errorf("store: append %s: %w", path, err)
		}
		changed[path] = oid
	}
	for path, data := range b.Files {
		oid, err := s.git.HashObject(data)
		if err != nil {
			return fmt.Errorf("store: append %s: %w", path, err)
		}
		changed[path] = oid
	}

	for attempt := 0; ; attempt++ {
		head, err := s.git.ReadRef(s.ref)
		if err != nil {
			return fmt.Errorf("store: append: read %s (run Init first?): %w", s.ref, err)
		}
		entries, err := s.git.LsTree(head)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}

		tree := make(map[string]string, len(entries)+len(changed))
		for _, e := range entries {
			tree[e.Path] = e.OID
		}
		for path, oid := range changed {
			tree[path] = oid
		}
		for _, path := range b.Delete {
			delete(tree, path)
		}

		next := make([]gitx.TreeEntry, 0, len(tree))
		for path, oid := range tree {
			next = append(next, gitx.TreeEntry{Path: path, OID: oid})
		}
		sort.Slice(next, func(i, j int) bool { return next[i].Path < next[j].Path })

		treeOID, err := s.git.MkTree(next)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}
		msg := fmt.Sprintf("tuhdoo: %d events, %d files, %d deletions\n",
			len(b.Events), len(b.Files), len(b.Delete))
		commit, err := s.git.CommitTree(treeOID, []string{head}, s.ident, msg)
		if err != nil {
			return fmt.Errorf("store: append: %w", err)
		}

		err = s.git.UpdateRef(s.ref, commit, head)
		if err == nil {
			return nil
		}
		if !errors.Is(err, gitx.ErrRefCASFailed) {
			return fmt.Errorf("store: append: %w", err)
		}
		if attempt+1 >= maxCASRetries {
			return fmt.Errorf("store: append: ref %s kept moving after %d attempts: %w",
				s.ref, maxCASRetries, err)
		}
	}
}

// LoadEvents reads and decodes every event blob under events/ at the
// current head, in path order (which is ULID-date order).
func (s *Store) LoadEvents() ([]event.Event, error) {
	head, err := s.git.ReadRef(s.ref)
	if err != nil {
		return nil, fmt.Errorf("store: load events: %w", err)
	}
	entries, err := s.git.LsTree(head)
	if err != nil {
		return nil, fmt.Errorf("store: load events: %w", err)
	}

	var events []event.Event
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Path, "events/") {
			continue
		}
		data, err := s.git.CatFile(entry.OID)
		if err != nil {
			return nil, fmt.Errorf("store: load events: %s: %w", entry.Path, err)
		}
		e, err := event.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("store: load events: %s: %w", entry.Path, err)
		}
		events = append(events, e)
	}
	return events, nil
}
