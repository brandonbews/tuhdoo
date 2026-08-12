package core

// Replay tests for the D6 confirmation gate (2026-08-04 revision): a
// confirmed claim wins its contest unconditionally, one confirmation
// per contest (not one per task forever), and corrupt double
// confirmations resolve deterministically — fail-safe, never fail-stop.

import (
	"errors"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

func claimConfirmed(t *testing.T, n int, actor, task, claim string) event.Event {
	t.Helper()
	return evt(t, n, event.TypeClaimConfirmed, actor, task, event.ClaimConfirmed{Claim: claim})
}

func TestConfirmedClaimBeatsEarlierUnconfirmed(t *testing.T) {
	// Claim A minted first (provisional winner), claim B minted second
	// (provisionally voided) — then B's confirmation lands, won at the
	// remote CAS. The final verdict is the referee's: B holds, A is
	// voided, in every arrival order (union merges guarantee nothing).
	cA, cB := tick(t, 2), tick(t, 3)
	confB := tick(t, 4)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		claimConfirmed(t, 4, "sarah/impl-9", "t1", cB),
	}
	leases := map[string]time.Time{
		cA: testNow.Add(time.Hour),
		cB: testNow.Add(time.Hour),
	}

	same := sameAsFirst(t)
	for _, p := range permutations(len(events)) {
		s := replay(t, permute(events, p), leases)
		if got := s.Claims[cA].Status; got != ClaimVoided {
			t.Fatalf("perm %v: earlier unconfirmed claim status = %s, want voided", p, got)
		}
		if got := s.Claims[cB]; got.Status != ClaimActive || got.Confirmation != confB {
			t.Fatalf("perm %v: confirmed claim = %s/%q, want active/%s", p, got.Status, got.Confirmation, confB)
		}
		if ac := s.ActiveClaim("t1"); ac == nil || ac.ID != cB {
			t.Fatalf("perm %v: active claim = %v, want %s", p, ac, cB)
		}
		same(p, s)
	}
}

func TestConfirmationBindsToOneContest(t *testing.T) {
	// A confirmation settles one contest; after the confirmed claim
	// ends, the task returns to the pool, a new contest opens, and its
	// confirmation is honored. One confirmation per contest, not one
	// per task forever.
	type expectClaim struct {
		status  ClaimStatus
		confirm int // tick of the expected confirmation; 0 for none
	}
	cases := []struct {
		name   string
		events func(t *testing.T) []event.Event
		leases func(t *testing.T) map[string]time.Time
		want   map[int]expectClaim // claim tick → expected verdict
		active int                 // tick of the expected active claim; 0 for none
	}{
		{
			name: "released confirmed claim yields a new contest whose confirmation is honored",
			events: func(t *testing.T) []event.Event {
				return []event.Event{
					taskCreated(t, 1, "t1", "fix login"),
					evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
					claimConfirmed(t, 3, "brandon/impl-1", "t1", tick(t, 2)),
					evt(t, 4, event.TypeClaimReleased, "brandon/impl-1", "t1", event.ClaimReleased{Reason: "handing off"}),
					evt(t, 5, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
					claimConfirmed(t, 6, "sarah/impl-9", "t1", tick(t, 5)),
				}
			},
			leases: func(t *testing.T) map[string]time.Time {
				return map[string]time.Time{tick(t, 5): testNow.Add(time.Hour)}
			},
			want: map[int]expectClaim{
				2: {status: ClaimReleased, confirm: 3},
				5: {status: ClaimActive, confirm: 6},
			},
			active: 5,
		},
		{
			name: "expired confirmed claim yields a new contest whose confirmation is honored",
			events: func(t *testing.T) []event.Event {
				return []event.Event{
					taskCreated(t, 1, "t1", "fix login"),
					evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
					claimConfirmed(t, 3, "brandon/impl-1", "t1", tick(t, 2)),
					// Minute 40: the incumbent's lease (below) lapsed at
					// minute 17 — confirmation settled the race, not
					// liveness, so the newcomer takes the hold.
					evt(t, 40, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
					claimConfirmed(t, 41, "sarah/impl-9", "t1", tick(t, 40)),
				}
			},
			leases: func(t *testing.T) map[string]time.Time {
				return map[string]time.Time{
					tick(t, 2):  base.Add(17 * time.Minute),
					tick(t, 40): testNow.Add(time.Hour),
				}
			},
			want: map[int]expectClaim{
				2:  {status: ClaimExpired, confirm: 3},
				40: {status: ClaimActive, confirm: 41},
			},
			active: 40,
		},
		{
			name: "confirmation for an already-ended claim settles nothing",
			events: func(t *testing.T) []event.Event {
				return []event.Event{
					taskCreated(t, 1, "t1", "fix login"),
					evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
					evt(t, 3, event.TypeClaimReleased, "brandon/impl-1", "t1", event.ClaimReleased{Reason: "standing down"}),
					claimConfirmed(t, 4, "brandon/impl-1", "t1", tick(t, 2)),
				}
			},
			leases: func(t *testing.T) map[string]time.Time { return nil },
			want: map[int]expectClaim{
				2: {status: ClaimReleased},
			},
			active: 0,
		},
		{
			name: "duplicate confirmations of one claim keep the earliest",
			events: func(t *testing.T) []event.Event {
				return []event.Event{
					taskCreated(t, 1, "t1", "fix login"),
					evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
					claimConfirmed(t, 3, "brandon/impl-1", "t1", tick(t, 2)),
					claimConfirmed(t, 4, "brandon/impl-1", "t1", tick(t, 2)),
				}
			},
			leases: func(t *testing.T) map[string]time.Time {
				return map[string]time.Time{tick(t, 2): testNow.Add(time.Hour)}
			},
			want: map[int]expectClaim{
				2: {status: ClaimActive, confirm: 3},
			},
			active: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := replay(t, tc.events(t), tc.leases(t))
			for n, want := range tc.want {
				c := s.Claims[tick(t, n)]
				if c == nil {
					t.Fatalf("claim at tick %d missing from state", n)
				}
				wantConf := ""
				if want.confirm != 0 {
					wantConf = tick(t, want.confirm)
				}
				if c.Status != want.status || c.Confirmation != wantConf {
					t.Fatalf("claim at tick %d = %s/%q, want %s/%q",
						n, c.Status, c.Confirmation, want.status, wantConf)
				}
			}
			ac := s.ActiveClaim("t1")
			switch {
			case tc.active == 0 && ac != nil:
				t.Fatalf("active claim = %s, want none", ac.ID)
			case tc.active != 0 && (ac == nil || ac.ID != tick(t, tc.active)):
				t.Fatalf("active claim = %v, want tick %d", ac, tc.active)
			}
		})
	}
}

func TestCorruptDoubleConfirmationResolvesDeterministically(t *testing.T) {
	// Two confirmations inside one contest can only come from a corrupt
	// ledger — the writers' invariant refuses to carry a second — but
	// replay must still resolve it, identically on every machine and in
	// every arrival order: the earliest confirmation ULID wins.
	cA, cB := tick(t, 2), tick(t, 3)
	confB := tick(t, 4) // earliest confirmation: B wins
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		claimConfirmed(t, 4, "sarah/impl-9", "t1", cB),
		claimConfirmed(t, 5, "brandon/impl-1", "t1", cA),
	}
	leases := map[string]time.Time{
		cA: testNow.Add(time.Hour),
		cB: testNow.Add(time.Hour),
	}

	same := sameAsFirst(t)
	for _, p := range permutations(len(events)) {
		s := replay(t, permute(events, p), leases)
		if got := s.Claims[cB]; got.Status != ClaimActive || got.Confirmation != confB {
			t.Fatalf("perm %v: claim B = %s/%q, want active/%s (earliest confirmation wins)",
				p, got.Status, got.Confirmation, confB)
		}
		if got := s.Claims[cA]; got.Status != ClaimVoided || got.Confirmation != "" {
			t.Fatalf("perm %v: claim A = %s/%q, want voided with no confirmation",
				p, got.Status, got.Confirmation)
		}
		same(p, s)
	}
}

func TestConfirmationOfUnknownClaimIsMalformed(t *testing.T) {
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		claimConfirmed(t, 2, "brandon/impl-1", "t1", tick(t, 9)),
	}
	_, err := NewReplayer().Replay(Input{Events: events, Now: testNow})
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("err = %v, want ErrMalformedEvent", err)
	}
}

func TestConfirmationTaskMismatchIsMalformed(t *testing.T) {
	// The envelope names one task, the payload a claim on another:
	// something wrote garbage — fail-safe, not best-effort.
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		taskCreated(t, 2, "t2", "fix logout"),
		evt(t, 3, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		claimConfirmed(t, 4, "brandon/impl-1", "t2", tick(t, 3)),
	}
	_, err := NewReplayer().Replay(Input{Events: events, Now: testNow})
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("err = %v, want ErrMalformedEvent", err)
	}
}
