package core

// Close-by-claim linkage (D6 clause 3; 2026-08-27 revision): a
// run.finished event may name the claim whose attempt it closes (the
// additive claim payload field), and close matching then binds to that
// claim alone — a later attempt's close no longer erases an earlier
// lost attempt's superseded trace. Events without the field keep the
// original task+actor+order heuristic, so legacy history replays to
// the state it always had (T3: stored bytes never rewritten, settled
// contests never move).

import (
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

func TestReclaimAfterLostRaceYieldsBothCloses(t *testing.T) {
	// The double-booking shape (Go-sweep audit, 2026-08-27): brandon's
	// first claim lost the race and its lease lapsed unreported; the
	// winner released without finishing; brandon re-claimed and finished
	// done. The done run names the second claim, so it closes only that
	// attempt — the first still gets its synthesized superseded trace:
	// two closes, one per attempt, identical in every arrival order.
	cWin, cLost, cAgain := tick(t, 2), tick(t, 3), tick(t, 6)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 5, event.TypeClaimReleased, "sarah/impl-9", "t1", event.ClaimReleased{Reason: "pulled away"}),
		evt(t, 6, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 10, event.TypeRunFinished, "brandon/impl-1", "t1", event.RunFinished{
			Outcome: event.OutcomeDone, Claim: cAgain, Summary: "second attempt landed"}),
	}
	leases := map[string]time.Time{
		cWin:   base.Add(40 * time.Minute), // alive through the tick-5 release
		cLost:  base.Add(20 * time.Minute), // lapsed long before testNow, unreported
		cAgain: testNow.Add(time.Hour),
	}

	same := sameAsFirst(t)
	for _, p := range permutations(len(events)) {
		s := replay(t, permute(events, p), leases)
		if got := s.Claims[cLost].Status; got != ClaimVoided {
			t.Fatalf("perm %v: lost claim status = %s, want voided", p, got)
		}
		if got := s.Claims[cAgain].Status; got != ClaimFinished {
			t.Fatalf("perm %v: re-claim status = %s, want finished", p, got)
		}
		if got := s.Tasks["t1"].Status; got != StatusDone {
			t.Fatalf("perm %v: task status = %s, want done", p, got)
		}
		if len(s.Runs) != 2 {
			t.Fatalf("perm %v: %d runs, want two — one close per attempt (all runs: %+v)", p, len(s.Runs), s.Runs)
		}
		done := s.Runs[0]
		if done.Synthesized || done.Outcome != event.OutcomeDone || done.Claim != cAgain {
			t.Fatalf("perm %v: real run %+v, want done closing the second attempt %s", p, done, cAgain)
		}
		synth := s.Runs[1]
		if !synth.Synthesized || synth.Outcome != event.OutcomeSuperseded ||
			synth.Claim != cLost || synth.Branch != "" {
			t.Fatalf("perm %v: lost attempt's close = %+v, want a branch-less synthesized superseded run for %s", p, synth, cLost)
		}
		same(p, s)
	}
}

func TestLoserSalvageRunCarriesItsClaim(t *testing.T) {
	// The loser reported back through finish_run (daemon-coerced to
	// superseded); the written event names the loser's claim, and the
	// replayed run carries it — real linkage, where a non-holder run
	// used to replay with no claim at all. The linked close also stands
	// in for synthesis even though the report landed after the lease
	// lapsed, and the winner's hold is untouched.
	cWin, cLost := tick(t, 2), tick(t, 3)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		evt(t, 40, event.TypeRunFinished, "sarah/impl-9", "t1", event.RunFinished{
			Outcome: event.OutcomeSuperseded, Claim: cLost,
			Branch: "tuh-x/lost-work", Summary: "peer won; branch kept for salvage"}),
	}
	leases := map[string]time.Time{
		cWin:  testNow.Add(time.Hour),
		cLost: base.Add(20 * time.Minute),
	}
	s := replay(t, events, leases)

	if len(s.Runs) != 1 {
		t.Fatalf("runs = %+v, want exactly the loser's real close", s.Runs)
	}
	r := s.Runs[0]
	if r.Synthesized || r.Claim != cLost || r.Branch != "tuh-x/lost-work" {
		t.Fatalf("salvage run = %+v, want it carrying its claim %s and its branch", r, cLost)
	}
	if got := s.Claims[cWin].Status; got != ClaimActive {
		t.Fatalf("winner status = %s, want active (a non-holder run moves nothing)", got)
	}
	if got := s.Tasks["t1"].Status; got != StatusOpen {
		t.Fatalf("task status = %s, want open", got)
	}
}

func TestLegacyRunsWithoutLinkageReplayUnchanged(t *testing.T) {
	// T3: events minted before the claim field existed carry none and
	// keep replaying through the original heuristic — any later run of
	// the same actor on the task closes the attempt — so existing
	// ledgers replay to the state they always had, INCLUDING the shape
	// the field was added to fix: on a legacy ledger the re-claimer's
	// close still suppresses the lost attempt's trace. Settled history
	// never moves; only claim-carrying events get the sharper
	// accounting. Payloads are built as raw maps so the encoded bytes
	// genuinely lack the field, exactly like stored history.
	legacyRun := func(outcome, branch, summary string) map[string]any {
		return map[string]any{
			"outcome": outcome, "branch": branch, "pr": "",
			"commits": nil, "merged_as": nil, "summary": summary,
		}
	}
	c2, c3, c6 := tick(t, 2), tick(t, 3), tick(t, 6)

	cases := []struct {
		name      string
		events    []event.Event
		leases    map[string]time.Time
		wantClaim string // Claim on the single real run
	}{
		{
			name: "a re-claimer's legacy close keeps suppressing the lost attempt's trace",
			events: []event.Event{
				taskCreated(t, 1, "t1", "fix login"),
				evt(t, 2, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
				evt(t, 3, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
				evt(t, 5, event.TypeClaimReleased, "sarah/impl-9", "t1", event.ClaimReleased{Reason: "pulled away"}),
				evt(t, 6, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
				evt(t, 10, event.TypeRunFinished, "brandon/impl-1", "t1",
					legacyRun(event.OutcomeDone, "", "second attempt landed")),
			},
			leases: map[string]time.Time{
				c2: base.Add(40 * time.Minute),
				c3: base.Add(20 * time.Minute),
				c6: testNow.Add(time.Hour),
			},
			wantClaim: c6, // replay's holder inference, as always
		},
		{
			name: "a claimless salvage run closes the lost attempt by the heuristic",
			events: []event.Event{
				taskCreated(t, 1, "t1", "fix login"),
				evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
				evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
				evt(t, 40, event.TypeRunFinished, "sarah/impl-9", "t1",
					legacyRun(event.OutcomeSuperseded, "tuh-x/lost-work", "peer won")),
			},
			leases: map[string]time.Time{
				c2: testNow.Add(time.Hour),
				c3: base.Add(20 * time.Minute),
			},
			wantClaim: "", // a legacy non-holder run is tied to no claim
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := replay(t, tc.events, tc.leases)
			if got := supersededRuns(s); len(got) != 0 {
				t.Fatalf("synthesized superseded runs = %+v, want none: legacy closes match by the heuristic", got)
			}
			var real []Run
			for _, r := range s.Runs {
				if !r.Synthesized {
					real = append(real, r)
				}
			}
			if len(real) != 1 || len(s.Runs) != 1 {
				t.Fatalf("runs = %+v, want exactly the one legacy close", s.Runs)
			}
			if real[0].Claim != tc.wantClaim {
				t.Fatalf("legacy run claim = %q, want %q", real[0].Claim, tc.wantClaim)
			}
		})
	}
}

func TestRunCloses(t *testing.T) {
	// The one close-matching rule, shared by replay's synthesis and the
	// daemon's write-side guard. IDs are plain strings — RunCloses only
	// ever compares them, and ULID lexical order is string order.
	c := &Claim{ID: "05", Task: "t1", Actor: "sarah/impl-9"}
	cases := []struct {
		name string
		run  Run
		want bool
	}{
		{"a run naming the claim closes it",
			Run{ID: "07", Task: "t1", Actor: "sarah/impl-9", Claim: "05", claimFromEvent: true}, true},
		{"a run naming another claim closes only that one, whatever else matches",
			Run{ID: "07", Task: "t1", Actor: "sarah/impl-9", Claim: "06", claimFromEvent: true}, false},
		{"a synthesized run binds to its claim",
			Run{ID: "05", Task: "t1", Actor: "sarah/impl-9", Claim: "05", Synthesized: true}, true},
		{"a synthesized run for another claim does not close this one",
			Run{ID: "06", Task: "t1", Actor: "sarah/impl-9", Claim: "06", Synthesized: true}, false},
		{"a legacy later run of the same actor on the task closes it (the original heuristic)",
			Run{ID: "07", Task: "t1", Actor: "sarah/impl-9", Claim: "04"}, true},
		{"a legacy run minted before the claim does not",
			Run{ID: "04", Task: "t1", Actor: "sarah/impl-9"}, false},
		{"a legacy run by another actor does not",
			Run{ID: "07", Task: "t1", Actor: "brandon/impl-1"}, false},
		{"a legacy run on another task does not",
			Run{ID: "07", Task: "t2", Actor: "sarah/impl-9"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RunCloses(&tc.run, c); got != tc.want {
				t.Fatalf("RunCloses(%+v, %+v) = %v, want %v", tc.run, c, got, tc.want)
			}
		})
	}
}
