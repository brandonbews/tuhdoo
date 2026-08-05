package syncer

import (
	"encoding/json"
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
//     so it fails loudly as corruption. Union also means a file present
//     on only one side always survives; for events that is correctness
//     (append-only), and for leases it is harmless by construction —
//     lease files are never deleted, only overwritten (2026-08-04), so
//     a one-sided lease is simply news the other head hasn't heard yet,
//     never a resurrection. One exception to the union (D6 writers'
//     invariant, 2026-08-04): a one-sided claim.confirmed that competes
//     with the other head's active confirmed claim is refused — see
//     confirmGuard below.
//   - leases/  : same path on both sides → a released tombstone beats a
//     plain lease regardless of expiry; two tombstones → the earlier
//     expiry wins (fail-safe determinism, the same posture as replay's
//     earliest-confirmation rule); two plain leases → the later expiry
//     wins (a renewal must never be undone by an older copy).
//     Released-beats-plain is safe, not a heuristic: a claim's lease is
//     written only by the claiming machine's own daemon under one
//     mutex, and a daemon never renews after standing down — a renewal
//     later than the tombstone structurally cannot exist, so any plain
//     copy losing to a tombstone is by construction stale.
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

	refused, err := s.confirmGuard(ourTree, theirTree)
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
			winner, err := s.mergeLease(path, ourOID, theirOID)
			if err != nil {
				return "", err
			}
			merged[path] = winner
		default:
			merged[path] = resolveOther(ourOID, theirOID)
		}
	}

	for path := range refused {
		delete(merged, path)
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

// confirmGuard enforces the D6 writers' invariant (2026-08-04) at the
// merge chokepoint: the merged tree never carries a confirmation for a
// task whose other head already shows a different active confirmed
// claim. It returns the event paths to leave out of the union — always
// one-sided claim.confirmed files, never settled history, so stored
// bytes on any published head stand untouched (T3).
//
// Honest gates never trip this: a confirmation reaches a head only
// after winning the remote's ref CAS, so competing confirmations cannot
// both be remote-accepted — this guards against buggy or rogue writers.
// When two confirmations do compete, the earliest event ULID keeps its
// place and the later is refused — the same rule replay applies to a
// corrupt ledger, so the merged tree and its replay can never disagree
// about the winner, and both merge directions refuse identically.
//
// Determinism notes: each side's state is replayed at that side's own
// latest event timestamp, never at the wall clock, so two machines
// merging the same pair of heads compute the same refusals at any hour.
// A side that cannot be decoded or replayed (fail-safe) skips the guard
// — identically on every machine of this binary version, the same
// posture as overlayViews; replay's earliest-confirmation rule still
// resolves whatever the union then carries.
func (s *Syncer) confirmGuard(ourTree, theirTree map[string]string) (map[string]bool, error) {
	oursOnly, oursOK, err := s.oneSidedConfirmations(ourTree, theirTree)
	if err != nil {
		return nil, err
	}
	theirsOnly, theirsOK, err := s.oneSidedConfirmations(theirTree, ourTree)
	if err != nil {
		return nil, err
	}
	if !oursOK || !theirsOK {
		s.logf("sync: merge: a head has undecodable events; confirmation guard skipped")
		return nil, nil
	}
	if len(oursOnly) == 0 && len(theirsOnly) == 0 {
		return nil, nil
	}

	ourState, errOurs := s.replayTreeFrozen(ourTree)
	theirState, errTheirs := s.replayTreeFrozen(theirTree)
	if errOurs != nil || errTheirs != nil {
		s.logf("sync: merge: cannot replay a head (%v / %v); confirmation guard skipped", errOurs, errTheirs)
		return nil, nil
	}

	refused := make(map[string]bool)
	judge := func(confirmations []confirmation, other *core.State) {
		for _, cf := range confirmations {
			incumbent := other.ActiveClaim(cf.task)
			if incumbent == nil || incumbent.Confirmation == "" || incumbent.ID == cf.claim {
				continue
			}
			// Competing confirmations: the earlier event ULID wins. A
			// one-sided loser is refused here; a one-sided winner keeps
			// its place and the other side's later confirmation is
			// refused by the symmetric pass (or, if it is settled shared
			// history, left for replay's identical earliest-wins rule).
			if cf.id > incumbent.Confirmation {
				refused[cf.path] = true
			}
		}
	}
	judge(oursOnly, theirState)
	judge(theirsOnly, ourState)
	return refused, nil
}

// confirmation is one claim.confirmed event found in a tree.
type confirmation struct {
	path  string
	id    string // the confirmation event's ULID
	task  string
	claim string // the claim it confirms
}

// oneSidedConfirmations lists the claim.confirmed events present in own
// but absent from other. ok is false when own carries an event this
// binary cannot decode — the caller skips the guard rather than judging
// a head it cannot read (fail-safe posture, deterministic per binary).
func (s *Syncer) oneSidedConfirmations(own, other map[string]string) ([]confirmation, bool, error) {
	var out []confirmation
	for path, oid := range own {
		if !strings.HasPrefix(path, "events/") {
			continue
		}
		if _, shared := other[path]; shared {
			continue
		}
		data, err := s.git.CatFile(oid)
		if err != nil {
			return nil, false, fmt.Errorf("syncer: merge: %w", err)
		}
		e, err := event.Decode(data)
		if err != nil {
			return nil, false, nil
		}
		if e.Type != event.TypeClaimConfirmed {
			continue
		}
		var p event.ClaimConfirmed
		if err := json.Unmarshal(e.Data, &p); err != nil {
			return nil, false, nil
		}
		out = append(out, confirmation{path: path, id: e.ID, task: e.Task, claim: p.Claim})
	}
	return out, true, nil
}

// replayTreeFrozen replays a tree at a clock frozen to the tree's own
// latest event timestamp, so the verdict is a function of the tree
// alone — merges must be deterministic across machines and hours, and
// lease expiry is the one place replay consults Now.
func (s *Syncer) replayTreeFrozen(tree map[string]string) (*core.State, error) {
	frozen := time.Time{}
	for path := range tree {
		if !strings.HasPrefix(path, "events/") {
			continue
		}
		id := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".json")
		when, err := event.IDTime(id)
		if err != nil {
			continue // undecodable name; the replay below fails honestly if it matters
		}
		if when.After(frozen) {
			frozen = when
		}
	}
	return s.replayTreeAt(tree, frozen)
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
// replays them at the current instant. Used on merged trees that exist
// only in the object database (no ref points at them yet).
func (s *Syncer) replayTree(tree map[string]string) (*core.State, error) {
	return s.replayTreeAt(tree, s.now())
}

// replayTreeAt is replayTree with an explicit instant, for callers that
// need the verdict to be a pure function of the tree (confirmGuard).
func (s *Syncer) replayTreeAt(tree map[string]string, now time.Time) (*core.State, error) {
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
	return s.replay.Replay(core.Input{Events: events, Leases: leases, Now: now})
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

// mergeLease picks the winning lease blob when the same leases/ path
// holds different content on both heads. Released beats plain, two
// released picks the earlier expiry, two plain picks the later expiry —
// the rationale lives in the per-area rules on merge above. Ties fall
// back to the lexically greater OID so both merge directions agree.
func (s *Syncer) mergeLease(path, a, b string) (string, error) {
	ta, releasedA, err := s.leaseState(path, a)
	if err != nil {
		return "", err
	}
	tb, releasedB, err := s.leaseState(path, b)
	if err != nil {
		return "", err
	}
	switch {
	case releasedA && !releasedB:
		return a, nil
	case releasedB && !releasedA:
		return b, nil
	case releasedA && releasedB && ta.Before(tb):
		return a, nil
	case releasedA && releasedB && tb.Before(ta):
		return b, nil
	case !releasedA && !releasedB && ta.After(tb):
		return a, nil
	case !releasedA && !releasedB && tb.After(ta):
		return b, nil
	}
	return maxOID(a, b), nil
}

func (s *Syncer) leaseState(path, oid string) (time.Time, bool, error) {
	data, err := s.git.CatFile(oid)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("syncer: merge %s: %w", path, err)
	}
	t, released, err := store.DecodeLeaseState(data)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("syncer: merge %s: %w", path, err)
	}
	return t, released, nil
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
