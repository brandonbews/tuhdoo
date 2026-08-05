package syncer

// Tests for the D6 confirmation gate's syncer half (2026-08-04): the
// merge chokepoint refuses to introduce a competing confirmation, and
// GateHead/GatePush land verdicts through the remote's ref CAS with a
// lost race leaving nothing behind.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
)

func confirmEvt(t *testing.T, n int, actor, machine, task, claim string) event.Event {
	t.Helper()
	return evt(t, n, event.TypeClaimConfirmed, actor, machine, task, event.ClaimConfirmed{Claim: claim})
}

func eventPath(t *testing.T, n int) string {
	t.Helper()
	p, err := event.Path(tick(t, n))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestMergeRefusesCompetingConfirmation drives the full Cycle path: a
// confirmation is settled on the remote, then a rogue peer commits a
// competing claim-plus-confirmation locally. The merge must union the
// claim (claims are optimistic) but refuse the second confirmation,
// and both machines must converge on identical trees.
func TestMergeRefusesCompetingConfirmation(t *testing.T) {
	a, b := newPair(t)
	alive := time.Now().Add(time.Hour)
	claimA, claimB := tick(t, 2), tick(t, 3)

	// A's claim and its confirmation reach the remote first — the
	// settled, CAS-won truth both sides then share.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "m-a", "t1", event.ClaimMade{}),
		confirmEvt(t, 4, "brandon/impl-1", "m-a", "t1", claimA),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.WriteLease(claimA, alive); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)
	cycle(t, b)

	// B goes rogue: a claim on the contested task plus a confirmation
	// for it, committed locally without ever winning the remote's CAS.
	if err := b.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "m-b", "t1", event.ClaimMade{}),
		confirmEvt(t, 5, "sarah/impl-9", "m-b", "t1", claimB),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := b.store.WriteLease(claimB, alive); err != nil {
		t.Fatal(err)
	}

	// A diverges and pushes first, so B's cycle must build a real merge.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 6, event.TypeNoteAdded, "brandon/impl-1", "m-a", "t1", event.NoteAdded{Text: "working"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)
	cycle(t, b) // fetches A, merges — the guard fires here — pushes
	cycle(t, a) // fast-forwards to the guarded merge
	sameTrees(t, a, b)

	tree := head(t, a)
	if _, ok := tree[eventPath(t, 5)]; ok {
		t.Fatal("merged tree carries the competing confirmation — the writers' invariant is broken")
	}
	if _, ok := tree[eventPath(t, 3)]; !ok {
		t.Fatal("merged tree lost the rogue claim event — claims union freely, only the confirmation is refused")
	}
	for _, p := range []peer{a, b} {
		s := stateOf(t, p)
		if got := s.Claims[claimA]; got.Status != core.ClaimActive || got.Confirmation != tick(t, 4) {
			t.Fatalf("claim A = %s/%q, want active with confirmation %s", got.Status, got.Confirmation, tick(t, 4))
		}
		if got := s.Claims[claimB]; got.Status != core.ClaimVoided || got.Confirmation != "" {
			t.Fatalf("claim B = %s/%q, want voided with no confirmation", got.Status, got.Confirmation)
		}
	}
}

// TestMergeRefusalKeepsEarliestConfirmation is the literal-tree table
// test: two heads each carrying a one-sided confirmation for the same
// contest. Whichever confirmation event has the earlier ULID keeps its
// place, the later is refused, and both merge directions produce the
// identical tree — matching replay's corrupt-ledger rule exactly.
func TestMergeRefusalKeepsEarliestConfirmation(t *testing.T) {
	cases := []struct {
		name             string
		confA, confB     int // confirmation event ticks written on A and B
		keepTick         int
		dropTick         int
		winner, loser    int // claim ticks (2 = A's claim, 3 = B's claim)
		winnerConfirmTck int
	}{
		{
			name:  "A's earlier confirmation wins",
			confA: 10, confB: 11,
			keepTick: 10, dropTick: 11,
			winner: 2, loser: 3, winnerConfirmTck: 10,
		},
		{
			name:  "B's earlier confirmation wins",
			confA: 11, confB: 10,
			keepTick: 10, dropTick: 11,
			winner: 3, loser: 2, winnerConfirmTck: 10,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := newPair(t)
			alive := time.Now().Add(time.Hour)
			claimA, claimB := tick(t, 2), tick(t, 3)

			// Shared history: the task and both racing claims.
			if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
				evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
				evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "m-a", "t1", event.ClaimMade{}),
				evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "m-b", "t1", event.ClaimMade{}),
			}}); err != nil {
				t.Fatal(err)
			}
			for _, c := range []string{claimA, claimB} {
				if err := a.store.WriteLease(c, alive); err != nil {
					t.Fatal(err)
				}
			}
			cycle(t, a)
			cycle(t, b)

			// Divergence: each side confirms its own machine's claim.
			if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
				confirmEvt(t, tc.confA, "brandon/impl-1", "m-a", "t1", claimA),
			}}); err != nil {
				t.Fatal(err)
			}
			if err := b.store.AppendBatch(store.Batch{Events: []event.Event{
				confirmEvt(t, tc.confB, "sarah/impl-9", "m-b", "t1", claimB),
			}}); err != nil {
				t.Fatal(err)
			}

			// Publish B's head so A's object database holds both sides,
			// then merge the two heads directly, in both orders.
			cycle(t, b)
			remoteHead, err := a.sync.fetch()
			if err != nil {
				t.Fatal(err)
			}
			localHead, err := a.git.ReadRef(store.DefaultRef)
			if err != nil {
				t.Fatal(err)
			}
			m1, err := a.sync.merge(localHead, remoteHead)
			if err != nil {
				t.Fatal(err)
			}
			m2, err := a.sync.merge(remoteHead, localHead)
			if err != nil {
				t.Fatal(err)
			}

			t1, err := treeMap(a.git, m1)
			if err != nil {
				t.Fatal(err)
			}
			t2, err := treeMap(a.git, m2)
			if err != nil {
				t.Fatal(err)
			}
			if len(t1) != len(t2) {
				t.Fatalf("merge directions disagree: %d vs %d entries", len(t1), len(t2))
			}
			for p, oid := range t1 {
				if t2[p] != oid {
					t.Fatalf("merge directions disagree at %s: %s vs %s", p, oid, t2[p])
				}
			}
			if _, ok := t1[eventPath(t, tc.dropTick)]; ok {
				t.Fatal("merged tree carries the later, competing confirmation")
			}
			if _, ok := t1[eventPath(t, tc.keepTick)]; !ok {
				t.Fatal("merged tree lost the earlier confirmation")
			}

			s, err := a.sync.replayTree(t1)
			if err != nil {
				t.Fatal(err)
			}
			winner, loser := tick(t, tc.winner), tick(t, tc.loser)
			if got := s.Claims[winner]; got.Status != core.ClaimActive || got.Confirmation != tick(t, tc.winnerConfirmTck) {
				t.Fatalf("winner = %s/%q, want active with confirmation %s",
					got.Status, got.Confirmation, tick(t, tc.winnerConfirmTck))
			}
			if got := s.Claims[loser]; got.Status != core.ClaimVoided || got.Confirmation != "" {
				t.Fatalf("loser = %s/%q, want voided with no confirmation", got.Status, got.Confirmation)
			}
		})
	}
}

func TestGateHeadRemoteless(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "solo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	runGit(t, dir, "init", "-b", "main")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(g, "", gitx.Identity{Name: "solo", Email: "s@test.invalid"})
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	solo := New(g, Options{})
	if _, _, err := solo.GateHead(); !errors.Is(err, gitx.ErrNoRemote) {
		t.Fatalf("GateHead without a remote = %v, want ErrNoRemote (the caller owns the T2 remoteless path)", err)
	}
}

// TestGatePushLostRaceLeavesNothing: a non-fast-forward GatePush is a
// lost race at the remote's CAS, and it must leave no trace — no local
// commit, no event file — so the caller can refetch and re-judge from a
// clean slate. The retry then lands the confirmation.
func TestGatePushLostRaceLeavesNothing(t *testing.T) {
	a, b := newPair(t)
	alive := time.Now().Add(time.Hour)
	claim := tick(t, 2)

	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "m-a", "t1", event.ClaimMade{}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.WriteLease(claim, alive); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)
	cycle(t, b)

	head1, state1, err := b.sync.GateHead()
	if err != nil {
		t.Fatalf("GateHead: %v", err)
	}
	if c := state1.ActiveClaim("t1"); c == nil || c.ID != claim {
		t.Fatalf("judged state active claim = %v, want %s", c, claim)
	}

	// The remote moves between B's judge and push: A lands a note.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 3, event.TypeNoteAdded, "brandon/impl-1", "m-a", "t1", event.NoteAdded{Text: "still here"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)

	conf := confirmEvt(t, 4, "brandon/impl-1", "m-b", "t1", claim)
	if err := b.sync.GatePush(head1, conf); !errors.Is(err, gitx.ErrNonFastForward) {
		t.Fatalf("GatePush onto a stale head = %v, want ErrNonFastForward", err)
	}
	if _, ok := head(t, b)[eventPath(t, 4)]; ok {
		t.Fatal("lost race left the confirmation on the local ref — nothing may be written before the CAS is won")
	}

	// Go around, as the daemon's loop would: refetch, re-judge, push.
	head2, _, err := b.sync.GateHead()
	if err != nil {
		t.Fatalf("GateHead retry: %v", err)
	}
	if head2 == head1 {
		t.Fatal("retry judged the same stale head")
	}
	if err := b.sync.GatePush(head2, conf); err != nil {
		t.Fatalf("GatePush retry: %v", err)
	}
	if _, ok := head(t, b)[eventPath(t, 4)]; !ok {
		t.Fatal("won CAS but the confirmation is not on the local ref")
	}
	cycle(t, a)
	sameTrees(t, a, b)
	for _, p := range []peer{a, b} {
		s := stateOf(t, p)
		if got := s.Claims[claim]; got.Status != core.ClaimActive || got.Confirmation != tick(t, 4) {
			t.Fatalf("claim = %s/%q, want active with confirmation %s", got.Status, got.Confirmation, tick(t, 4))
		}
	}
}
