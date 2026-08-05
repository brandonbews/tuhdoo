package syncer

// The referee's half of the D6 confirmation gate (001 D6, revised
// 2026-08-04; T8 "confirmation gate" row): the one deliberately
// synchronous sync in the system. The daemon judges; these two methods
// carry the git legwork — GateHead answers "what head would a
// confirmation push onto, and what does its state say", GatePush lands
// the verdict through the remote's ref compare-and-swap. The caller's
// loop around them (fetch → judge → push, retry on non-fast-forward) is
// the same bounded shape as Cycle's.

import (
	"fmt"
	"sort"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/views"
)

// GateHead fetches the remote data branch, reconciles it into the local
// ref, and returns the local head with its replayed state — exactly the
// head a confirmation would be pushed onto, and exactly the state the
// referee judges.
//
// With no remote configured the error matches gitx.ErrNoRemote: the
// caller owns the T2 remoteless path (the daemon is the sole writer, so
// its local verdict is final). Any other error means the remote could
// not be honestly consulted; the caller must refuse and write nothing —
// the referee never guesses.
func (s *Syncer) GateHead() (string, *core.State, error) {
	if _, err := s.git.RemoteURL(s.remote); err != nil {
		return "", nil, fmt.Errorf("syncer: gate: %w", err)
	}
	remoteHead, err := s.fetch()
	if err != nil {
		return "", nil, err
	}
	local, err := s.git.ReadRef(s.ref)
	if err != nil {
		return "", nil, fmt.Errorf("syncer: gate: %w", err)
	}
	if remoteHead != "" && remoteHead != local {
		// Bring the remote's work in (fast-forward or app-level merge).
		// A CAS loss inside reconcile leaves the local ref where it was;
		// the judged head then simply lacks the remote's tip, GatePush
		// answers non-fast-forward, and the caller's loop goes around.
		if _, err := s.reconcile(local, remoteHead); err != nil {
			return "", nil, err
		}
		local, err = s.git.ReadRef(s.ref)
		if err != nil {
			return "", nil, fmt.Errorf("syncer: gate: %w", err)
		}
	}
	tree, err := treeMap(s.git, local)
	if err != nil {
		return "", nil, err
	}
	state, err := s.replayTree(tree)
	if err != nil {
		return "", nil, fmt.Errorf("syncer: gate: replay %s: %w", local, err)
	}
	return local, state, nil
}

// GatePush publishes the referee's verdict: one commit holding exactly
// e, built on top of head, pushed through the remote's atomic ref
// compare-and-swap. The event touches no ref until the remote has
// accepted the push, so a lost race leaves nothing behind — no local
// commit to unwind, no stored bytes to rewrite (T3). An error matching
// gitx.ErrNonFastForward means the remote moved past head (possibly a
// competitor's confirmation landing first); the caller refetches and
// re-judges. On success the confirmation is irrevocable: the local ref
// is advanced to include it, best-effort — a busy local writer defers
// that to the next sync cycle without touching the verdict, because the
// remote already holds it.
func (s *Syncer) GatePush(head string, e event.Event) error {
	tree, err := treeMap(s.git, head)
	if err != nil {
		return err
	}
	path, err := event.Path(e.ID)
	if err != nil {
		return fmt.Errorf("syncer: gate: %w", err)
	}
	data, err := event.Encode(e)
	if err != nil {
		return fmt.Errorf("syncer: gate: %w", err)
	}
	oid, err := s.git.HashObject(data)
	if err != nil {
		return fmt.Errorf("syncer: gate: %w", err)
	}
	tree[path] = oid

	// Views ride every write (T6), under the same highest-wins guard as
	// the daemon's own commits: views stamped by a newer generator are
	// never overwritten — the event still lands, the newer peer renders.
	format, err := s.stampFormat(tree)
	if err != nil {
		return err
	}
	if format <= views.FormatVersion {
		if err := s.overlayViews(tree); err != nil {
			return err
		}
	}

	entries := make([]gitx.TreeEntry, 0, len(tree))
	for p, o := range tree {
		entries = append(entries, gitx.TreeEntry{Path: p, OID: o})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	treeOID, err := s.git.MkTree(entries)
	if err != nil {
		return fmt.Errorf("syncer: gate: %w", err)
	}
	commit, err := s.git.CommitTree(treeOID, []string{head}, s.ident, "tuhdoo: confirm claim\n")
	if err != nil {
		return fmt.Errorf("syncer: gate: %w", err)
	}

	if err := s.git.Push(s.remote, commit+":"+s.ref); err != nil {
		if errIsAny(err, gitx.ErrNonFastForward) {
			s.bumpCollisions() // gate contention is push contention (T8 telemetry)
		}
		return err
	}
	s.recordPush(commit)

	// Bring the confirmation into the local ref so the daemon's next
	// refresh sees it without waiting a fetch interval. Failures here
	// are logged, not returned: the verdict is already won at the
	// remote, and the ordinary sync loop converges on it regardless.
	for attempt := 0; attempt < maxCycleRetries; attempt++ {
		local, err := s.git.ReadRef(s.ref)
		if err != nil {
			s.logf("sync: gate: read local ref after push: %v", err)
			return nil
		}
		landed, err := s.git.IsAncestor(commit, local)
		if err != nil {
			s.logf("sync: gate: ancestry check after push: %v", err)
			return nil
		}
		if landed {
			return nil
		}
		if _, err := s.reconcile(local, commit); err != nil {
			s.logf("sync: gate: advancing local ref: %v", err)
			return nil
		}
	}
	s.logf("sync: gate: local ref kept moving; confirmation %s arrives on the next cycle", e.ID)
	return nil
}
