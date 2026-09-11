package syncer

import (
	"encoding/json"
	"errors"
	"fmt"
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
func (s *Syncer) merge(remote string) error {
	theirTree, err := treeMap(s.git, remote)
	if err != nil {
		return err
	}
	// Our side is the replica's head and tree, taken together: the
	// head anchors the commit below, so a merge computed against this
	// tree can only land on this head.
	ourHead, ourTree := s.store.HeadAndTree()

	merged, err := s.mergeTrees(ourTree, theirTree)
	if err != nil {
		return err
	}

	// The store commits the union as changes against our tree, with
	// the remote head as the second parent; it moves the ref and the
	// replica together (T2: single mover). A head that moved since —
	// a local batch, an external move — refuses the commit with
	// ErrRefCASFailed, and the caller's next pass merges afresh.
	_, err = s.store.Commit(ourHead, treeChanges(ourTree, merged), []string{remote}, "tuhdoo: merge\n")
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	return nil
}

// mergeTrees computes the merged tree of two heads' trees under the
// per-area rules above. Symmetric: both argument orders produce the
// same map. Blob reads (lease states, view stamps, the guard's
// replays) go through the store's caches.
func (s *Syncer) mergeTrees(ourTree, theirTree map[string]string) (map[string]string, error) {
	resolveOther, regen, err := s.viewsPolicy(ourTree, theirTree)
	if err != nil {
		return nil, err
	}

	refused, err := s.confirmGuard(ourTree, theirTree)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]string, len(ourTree)+len(theirTree))
	for path, oid := range ourTree {
		merged[path] = oid
	}
	// Same-path lease conflicts are collected and decided after one
	// batched read of every blob involved: a renewal since the last
	// push conflicts with the remote's copy on every merge, and each
	// such copy is a blob the replica no longer holds.
	var conflicts []leaseConflict
	for path, theirOID := range theirTree {
		ourOID, both := merged[path]
		if !both || ourOID == theirOID {
			merged[path] = theirOID
			continue
		}
		switch {
		case strings.HasPrefix(path, "events/"):
			return nil, fmt.Errorf("syncer: merge: event %s differs between heads — data corruption", path)
		case strings.HasPrefix(path, "leases/"):
			conflicts = append(conflicts, leaseConflict{path: path, ours: ourOID, theirs: theirOID})
		default:
			merged[path] = resolveOther(ourOID, theirOID)
		}
	}
	winners, err := s.mergeLeases(conflicts)
	if err != nil {
		return nil, err
	}
	for path, oid := range winners {
		merged[path] = oid
	}

	for path := range refused {
		delete(merged, path)
	}

	if regen {
		if err := s.overlayViews(merged); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

// treeChanges expresses to as changes against from, in store.Commit's
// shape: every path whose blob differs or is new maps to its OID in
// to; every path from holds that to lacks maps to "" (delete). Pure.
func treeChanges(from, to map[string]string) map[string]string {
	changes := make(map[string]string)
	for path, oid := range to {
		if from[path] != oid {
			changes[path] = oid
		}
	}
	for path := range from {
		if _, kept := to[path]; !kept {
			changes[path] = ""
		}
	}
	return changes
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
	var paths, oids []string
	for path, oid := range own {
		if !strings.HasPrefix(path, "events/") {
			continue
		}
		if _, shared := other[path]; shared {
			continue
		}
		paths = append(paths, path)
		oids = append(oids, oid)
	}
	events, err := s.store.EventsByOID(oids)
	if err != nil {
		if errors.Is(err, store.ErrUndecodable) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("syncer: merge: %w", err)
	}
	var out []confirmation
	for i, path := range paths {
		e := events[oids[i]]
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
	data, err := s.store.Blob(oid)
	if err != nil {
		return 0, fmt.Errorf("syncer: merge: %w", err)
	}
	return views.Format(data), nil
}

// overlayViews replaces the view paths in the merged tree with a fresh
// render of the merged state, writing only the pages whose bytes
// differ from what the tree already holds (store.WriteBlobs: the
// object-ID diff of T2). A replay failure (fail-safe) skips the
// overlay: the tree merge is still valid, every machine of this binary
// version skips identically, and the daemon degrades honestly on its
// next refresh.
func (s *Syncer) overlayViews(merged map[string]string) error {
	state, err := s.replayTree(merged)
	if err != nil {
		s.logf("sync: cannot replay merged events (%v); views not regenerated", err)
		return nil
	}
	rendered, err := s.store.WriteBlobs(views.Render(state), merged)
	if err != nil {
		return fmt.Errorf("syncer: merge: render: %w", err)
	}
	for path, oid := range rendered {
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
// The tree is read by the store's reader — the one place that
// classifies paths and batches the blob reads, and the owner of the
// decode caches (T2, 2026-09-10) — so this replay and the store's head
// load compute the same lease set by construction, and a replay after
// a fetch reads only the other side's new blobs; views are never read
// back.
func (s *Syncer) replayTreeAt(tree map[string]string, now time.Time) (*core.State, error) {
	events, leases, err := s.store.ReplayInputFor(tree)
	if err != nil {
		return nil, err
	}
	return s.replay.Replay(core.Input{Events: events, Leases: leases, Now: now})
}

func treeMap(g gitx.Git, rev string) (map[string]string, error) {
	m, err := gitx.LsTreeMap(g, rev)
	if err != nil {
		return nil, fmt.Errorf("syncer: %w", err)
	}
	return m, nil
}

// leaseConflict is one leases/ path holding different blobs on the
// two heads.
type leaseConflict struct {
	path, ours, theirs string
}

// mergeLeases decides every same-path lease conflict after one batched
// read of the blobs involved (through the store's cache), returning
// path → winning OID. Released beats plain, two released picks the
// earlier expiry, two plain picks the later expiry — the rationale
// lives in the per-area rules on merge above. Ties fall back to the
// lexically greater OID so both merge directions agree.
func (s *Syncer) mergeLeases(conflicts []leaseConflict) (map[string]string, error) {
	if len(conflicts) == 0 {
		return nil, nil
	}
	oids := make([]string, 0, 2*len(conflicts))
	for _, c := range conflicts {
		oids = append(oids, c.ours, c.theirs)
	}
	states, err := s.store.LeasesByOID(oids)
	if err != nil {
		return nil, fmt.Errorf("syncer: merge leases: %w", err)
	}
	winners := make(map[string]string, len(conflicts))
	for _, c := range conflicts {
		winners[c.path] = pickLease(c.ours, states[c.ours], c.theirs, states[c.theirs])
	}
	return winners, nil
}

// pickLease is the pure lease rule for one conflict: the winning OID
// of a (blob a, state sa) versus (blob b, state sb).
func pickLease(a string, sa store.LeaseState, b string, sb store.LeaseState) string {
	switch {
	case sa.Released && !sb.Released:
		return a
	case sb.Released && !sa.Released:
		return b
	case sa.Released && sb.Released && sa.Expires.Before(sb.Expires):
		return a
	case sa.Released && sb.Released && sb.Expires.Before(sa.Expires):
		return b
	case !sa.Released && !sb.Released && sa.Expires.After(sb.Expires):
		return a
	case !sa.Released && !sb.Released && sb.Expires.After(sa.Expires):
		return b
	}
	return maxOID(a, b)
}

func maxOID(a, b string) string {
	if a > b {
		return a
	}
	return b
}
