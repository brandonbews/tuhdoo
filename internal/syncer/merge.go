package syncer

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
	"github.com/brandonbews/tuhdoo/internal/views"
)

// merge builds the app-level merge of two divergent data-branch heads
// (002 T2): git's merge machinery is never invoked. The merged tree is
// computed here — deterministically, so both machines merging the same
// pair of heads (in either order) produce the same tree — and committed
// with both heads as parents.
//
// Per-area rules:
//   - events/  : set union. The same path holding different content is
//     impossible for honest writers (events are immutable, path = ULID),
//     so it fails loudly as corruption. Union also means a file deleted
//     on one side but present on the other comes back; for events that
//     is correctness (append-only), for leases it is accepted junk — a
//     resurrected lease only matters to an ACTIVE claim, and active
//     claims never had their lease deleted.
//   - leases/  : same path on both sides → the later expiry wins
//     (a renewal must never be undone by an older copy).
//   - views + everything else: decided by the view-format stamps — see
//     resolveOther below.
func (s *Syncer) merge(ours, theirs string) (string, error) {
	ourTree, err := treeMap(s.git, ours)
	if err != nil {
		return "", err
	}
	theirTree, err := treeMap(s.git, theirs)
	if err != nil {
		return "", err
	}

	resolveOther, regen, err := s.viewsPolicy(ourTree, theirTree)
	if err != nil {
		return "", err
	}

	merged := make(map[string]string, len(ourTree)+len(theirTree))
	for path, oid := range ourTree {
		merged[path] = oid
	}
	for path, theirOID := range theirTree {
		ourOID, both := merged[path]
		if !both || ourOID == theirOID {
			merged[path] = theirOID
			continue
		}
		switch {
		case strings.HasPrefix(path, "events/"):
			return "", fmt.Errorf("syncer: merge: event %s differs between heads — data corruption", path)
		case strings.HasPrefix(path, "leases/"):
			winner, err := s.laterLease(path, ourOID, theirOID)
			if err != nil {
				return "", err
			}
			merged[path] = winner
		default:
			merged[path] = resolveOther(ourOID, theirOID)
		}
	}

	if regen {
		if err := s.overlayViews(merged); err != nil {
			return "", err
		}
	}

	entries := make([]gitx.TreeEntry, 0, len(merged))
	for path, oid := range merged {
		entries = append(entries, gitx.TreeEntry{Path: path, OID: oid})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	treeOID, err := s.git.MkTree(entries)
	if err != nil {
		return "", fmt.Errorf("syncer: merge: %w", err)
	}

	// Sorted parents: both machines merging the same pair state the
	// parents identically.
	parents := []string{ours, theirs}
	sort.Strings(parents)
	commit, err := s.git.CommitTree(treeOID, parents, s.ident, "tuhdoo: merge\n")
	if err != nil {
		return "", fmt.Errorf("syncer: merge: %w", err)
	}
	return commit, nil
}

// viewsPolicy reads both sides' view-format stamps and decides two
// things: how non-event, non-lease path conflicts resolve, and whether
// this binary regenerates views after the union (T6 highest-wins).
//
//   - Neither side newer than us → we own the views: conflicts resolve
//     by lexically-greater OID (arbitrary but symmetric — regeneration
//     overwrites the view paths anyway) and regen is on.
//   - A side is newer → that side's files win every such conflict
//     wholesale (its views must stay internally consistent) and regen
//     is off. Both sides newer → the higher format wins; equal higher
//     formats → greater-OID, still symmetric.
func (s *Syncer) viewsPolicy(ourTree, theirTree map[string]string) (func(a, b string) string, bool, error) {
	fOurs, err := s.stampFormat(ourTree)
	if err != nil {
		return nil, false, err
	}
	fTheirs, err := s.stampFormat(theirTree)
	if err != nil {
		return nil, false, err
	}

	if fOurs <= views.FormatVersion && fTheirs <= views.FormatVersion {
		return maxOID, true, nil
	}
	s.logf("sync: views stamped by a newer tuhdoo (format %d/%d > %d); leaving them to that peer",
		fOurs, fTheirs, views.FormatVersion)
	switch {
	case fOurs > fTheirs:
		return func(a, _ string) string { return a }, false, nil
	case fTheirs > fOurs:
		return func(_, b string) string { return b }, false, nil
	default:
		return maxOID, false, nil
	}
}

func (s *Syncer) stampFormat(tree map[string]string) (int, error) {
	oid, ok := tree[views.MetaPath]
	if !ok {
		return 0, nil
	}
	data, err := s.git.CatFile(oid)
	if err != nil {
		return 0, fmt.Errorf("syncer: merge: %w", err)
	}
	return views.Format(data), nil
}

// overlayViews replaces the view paths in the merged tree with a fresh
// render of the merged state. A replay failure (fail-safe) skips the
// overlay: the tree merge is still valid, every machine of this binary
// version skips identically, and the daemon degrades honestly on its
// next refresh.
func (s *Syncer) overlayViews(merged map[string]string) error {
	state, err := s.replayTree(merged)
	if err != nil {
		s.logf("sync: cannot replay merged events (%v); views not regenerated", err)
		return nil
	}
	for path, data := range views.Render(state) {
		oid, err := s.git.HashObject(data)
		if err != nil {
			return fmt.Errorf("syncer: merge: render %s: %w", path, err)
		}
		merged[path] = oid
	}
	return nil
}

// replayTree loads events and leases straight out of a tree map and
// replays them. Used on merged trees that exist only in the object
// database (no ref points at them yet).
func (s *Syncer) replayTree(tree map[string]string) (*core.State, error) {
	var events []event.Event
	leases := make(map[string]time.Time)
	for path, oid := range tree {
		switch {
		case strings.HasPrefix(path, "events/"):
			data, err := s.git.CatFile(oid)
			if err != nil {
				return nil, err
			}
			e, err := event.Decode(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			events = append(events, e)
		case strings.HasPrefix(path, "leases/"):
			claimID := strings.TrimSuffix(strings.TrimPrefix(path, "leases/"), ".json")
			data, err := s.git.CatFile(oid)
			if err != nil {
				return nil, err
			}
			expires, err := store.DecodeLease(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			leases[claimID] = expires
		}
	}
	return s.replay.Replay(core.Input{Events: events, Leases: leases, Now: s.now()})
}

func treeMap(g gitx.Git, rev string) (map[string]string, error) {
	entries, err := g.LsTree(rev)
	if err != nil {
		return nil, fmt.Errorf("syncer: %w", err)
	}
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Path] = e.OID
	}
	return m, nil
}

// laterLease picks the lease blob with the later expiry; ties fall back
// to the lexically greater OID so both merge directions agree.
func (s *Syncer) laterLease(path, a, b string) (string, error) {
	ta, err := s.leaseExpiry(path, a)
	if err != nil {
		return "", err
	}
	tb, err := s.leaseExpiry(path, b)
	if err != nil {
		return "", err
	}
	if ta.After(tb) {
		return a, nil
	}
	if tb.After(ta) {
		return b, nil
	}
	return maxOID(a, b), nil
}

func (s *Syncer) leaseExpiry(path, oid string) (time.Time, error) {
	data, err := s.git.CatFile(oid)
	if err != nil {
		return time.Time{}, fmt.Errorf("syncer: merge %s: %w", path, err)
	}
	t, err := store.DecodeLease(data)
	if err != nil {
		return time.Time{}, fmt.Errorf("syncer: merge %s: %w", path, err)
	}
	return t, nil
}

func maxOID(a, b string) string {
	if a > b {
		return a
	}
	return b
}

// errIsAny reports whether err matches any of the given sentinels.
func errIsAny(err error, sentinels ...error) bool {
	for _, s := range sentinels {
		if errors.Is(err, s) {
			return true
		}
	}
	return false
}
