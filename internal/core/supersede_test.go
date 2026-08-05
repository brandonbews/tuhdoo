package core

// Replay tests for loser expiry synthesis (D6 clause 3, 2026-08-04): a
// voided claim whose lease lapses with no closing run by its actor gets
// a synthesized branch-less superseded run — the trace a race loser
// that never reported leaves behind — and a real close by the loser
// always wins over synthesis (one close per attempt). The verdict is a
// pure function of the replay inputs, so it is deterministic at every
// instant and in every event arrival order.

import (
	"reflect"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// supersededRuns returns the synthesized superseded runs in s, keyed by
// nothing — callers assert on count and fields.
func supersededRuns(s *State) []Run {
	var out []Run
	for _, r := range s.Runs {
		if r.Synthesized && r.Outcome == event.OutcomeSuperseded {
			out = append(out, r)
		}
	}
	return out
}

func TestVoidedClaimExpirySynthesizesSupersededRun(t *testing.T) {
	// Winner claims at tick 2 and holds; loser claims at tick 3 and is
	// voided; the loser's lease lapsed well before Now with no report.
	// Replay closes the attempt itself: one branch-less superseded run,
	// identical in every arrival order.
	cWin, cLose := tick(t, 2), tick(t, 3)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
	}
	leases := map[string]time.Time{
		cWin:  testNow.Add(time.Hour),
		cLose: base.Add(20 * time.Minute), // lapsed long before testNow
	}

	var first *State
	for _, p := range permutations(len(events)) {
		shuffled := make([]event.Event, len(events))
		for i, idx := range p {
			shuffled[i] = events[idx]
		}
		s := replay(t, shuffled, leases)
		if got := s.Claims[cLose].Status; got != ClaimVoided {
			t.Fatalf("perm %v: loser status = %s, want voided (synthesis never re-buckets the claim)", p, got)
		}
		if got := s.Claims[cWin].Status; got != ClaimActive {
			t.Fatalf("perm %v: winner status = %s, want active", p, got)
		}
		synth := supersededRuns(s)
		if len(synth) != 1 {
			t.Fatalf("perm %v: %d synthesized superseded runs, want one (all runs: %+v)", p, len(synth), s.Runs)
		}
		r := synth[0]
		if r.Claim != cLose || r.Actor != "sarah/impl-9" || r.Task != "t1" {
			t.Fatalf("perm %v: synthesized run %+v not tied to the loser's attempt", p, r)
		}
		if r.Branch != "" || r.PR != "" || len(r.Commits) != 0 {
			t.Fatalf("perm %v: synthesized run carries links %+v — must be branch-less (only the loser knows its branch)", p, r)
		}
		if len(s.Runs) != 1 {
			t.Fatalf("perm %v: %d runs total, want exactly the one synthesized close", p, len(s.Runs))
		}
		if first == nil {
			first = s
		} else if !reflect.DeepEqual(first, s) {
			t.Fatalf("perm %v: state differs from first permutation", p)
		}
	}
}

func TestVoidedClaimClosedByRealRunSkipsSynthesis(t *testing.T) {
	// The loser reported back (the daemon coerces such a finish to
	// superseded) — even though the report landed after the lease
	// lapsed at minute 20, the real run is the close and replay
	// synthesizes nothing. One close per attempt.
	cWin, cLose := tick(t, 2), tick(t, 3)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		evt(t, 40, event.TypeRunFinished, "sarah/impl-9", "t1", event.RunFinished{
			Outcome: event.OutcomeSuperseded, Branch: "tuh-x/lost-work",
			Summary: "peer won; branch kept for salvage"}),
	}
	leases := map[string]time.Time{
		cWin:  testNow.Add(time.Hour),
		cLose: base.Add(20 * time.Minute),
	}
	s := replay(t, events, leases)

	if got := supersededRuns(s); len(got) != 0 {
		t.Fatalf("synthesized superseded runs = %+v, want none: a real close wins", got)
	}
	if len(s.Runs) != 1 || s.Runs[0].Synthesized || s.Runs[0].Branch != "tuh-x/lost-work" {
		t.Fatalf("runs = %+v, want exactly the loser's real superseded run with its branch", s.Runs)
	}
	if got := s.Claims[cLose].Status; got != ClaimVoided {
		t.Fatalf("loser status = %s, want voided", got)
	}
}

func TestLoserExpiryVerdictDeterministicAcrossInstants(t *testing.T) {
	// The synthesis verdict is a pure function of lease data and Now —
	// both replay inputs — so the same ledger answers identically on
	// every machine at any given instant: no synthesis while the
	// loser's lease is alive (a report may still be coming), synthesis
	// the moment it is not. A voided claim with no lease record at all
	// counts as lapsed (the same rule interruptedRun applies).
	expiry := base.Add(30 * time.Minute)
	cWin, cLose := tick(t, 2), tick(t, 3)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
	}

	cases := []struct {
		name      string
		loserExp  *time.Time // nil: no lease record at all
		now       time.Time
		wantSynth int
	}{
		{"before expiry the attempt is still open", &expiry, expiry.Add(-time.Minute), 0},
		{"at expiry the lease is no longer after now", &expiry, expiry, 1},
		{"after expiry the attempt is closed", &expiry, expiry.Add(time.Minute), 1},
		{"a missing lease record counts as lapsed", nil, base.Add(5 * time.Minute), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leases := map[string]time.Time{cWin: tc.now.Add(time.Hour)}
			if tc.loserExp != nil {
				leases[cLose] = *tc.loserExp
			}
			s, err := NewReplayer().Replay(Input{Events: events, Leases: leases, Now: tc.now})
			if err != nil {
				t.Fatalf("replay: %v", err)
			}
			if got := len(supersededRuns(s)); got != tc.wantSynth {
				t.Fatalf("synthesized superseded runs at %s = %d, want %d",
					tc.now.Format(time.RFC3339), got, tc.wantSynth)
			}
			// The verdict never re-buckets the claim: voided throughout.
			if got := s.Claims[cLose].Status; got != ClaimVoided {
				t.Fatalf("loser status = %s, want voided", got)
			}
		})
	}
}

func TestEveryExpiredLoserLeavesItsOwnTrace(t *testing.T) {
	// Two distinct losers on one contest: each voided claim whose lease
	// lapsed unclosed gets its own synthesized run, sorted after real
	// runs in claim (ULID) order.
	cWin, cL1, cL2 := tick(t, 2), tick(t, 3), tick(t, 4)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		evt(t, 4, event.TypeClaimMade, "kim/impl-2", "t1", event.ClaimMade{}),
	}
	leases := map[string]time.Time{
		cWin: testNow.Add(time.Hour),
		cL1:  base.Add(20 * time.Minute),
		cL2:  base.Add(25 * time.Minute),
	}
	s := replay(t, events, leases)

	synth := supersededRuns(s)
	if len(synth) != 2 {
		t.Fatalf("synthesized superseded runs = %+v, want one per expired loser", synth)
	}
	if synth[0].Claim != cL1 || synth[1].Claim != cL2 {
		t.Fatalf("synthesized runs out of claim order: %+v", synth)
	}
}
