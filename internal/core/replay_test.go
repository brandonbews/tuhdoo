package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

var (
	base    = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	testNow = base.Add(2 * time.Hour)
)

// tick returns a deterministic ULID minted n minutes after base. Larger
// n always sorts later, so tests state intended replay order directly.
func tick(t *testing.T, n int) string {
	t.Helper()
	entropy := make([]byte, 10)
	entropy[9] = byte(n)
	id, err := event.NewID(base.Add(time.Duration(n)*time.Minute), bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// evt mints an event at the version this binary writes, like the
// daemon does; oldEvt pins an explicit version for evolution tests.
func evt(t *testing.T, n int, typ, actor, task string, payload any) event.Event {
	t.Helper()
	return oldEvt(t, n, typ, event.Versions[typ], actor, task, payload)
}

func oldEvt(t *testing.T, n int, typ string, v int, actor, task string, payload any) event.Event {
	t.Helper()
	e, err := event.New(tick(t, n), typ, v, actor, "m-test", task, payload)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func replay(t *testing.T, events []event.Event, leases map[string]time.Time) *State {
	t.Helper()
	s, err := NewReplayer().Replay(Input{Events: events, Leases: leases, Now: testNow})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return s
}

// aliveLease keeps a claim alive well past testNow.
func aliveLease(id string) map[string]time.Time {
	return map[string]time.Time{id: testNow.Add(time.Hour)}
}

func taskCreated(t *testing.T, n int, taskID, title string, deps ...string) event.Event {
	return evt(t, n, event.TypeTaskCreated, "brandon", taskID,
		event.TaskCreated{Title: title, DependsOn: deps})
}

func TestClaimRaceIsOrderInsensitive(t *testing.T) {
	// Two agents on two machines claim the same task. Whatever order the
	// events arrive in (union merges guarantee nothing), the earliest
	// ULID wins and every machine agrees (D6).
	c1, c2 := tick(t, 2), tick(t, 3)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		evt(t, 4, event.TypeNoteAdded, "brandon/impl-1", "t1", event.NoteAdded{Text: "starting"}),
	}
	leases := map[string]time.Time{
		c1: testNow.Add(time.Hour),
		c2: testNow.Add(time.Hour),
	}

	var first *State
	perms := permutations(len(events))
	for _, p := range perms {
		shuffled := make([]event.Event, len(events))
		for i, idx := range p {
			shuffled[i] = events[idx]
		}
		s := replay(t, shuffled, leases)
		if got := s.Claims[c1].Status; got != ClaimActive {
			t.Fatalf("perm %v: first claim status = %s, want active", p, got)
		}
		if got := s.Claims[c2].Status; got != ClaimVoided {
			t.Fatalf("perm %v: second claim status = %s, want voided", p, got)
		}
		if first == nil {
			first = s
		} else if !reflect.DeepEqual(first, s) {
			t.Fatalf("perm %v: state differs from first permutation", p)
		}
	}
	if len(perms) != 24 {
		t.Fatalf("expected 24 permutations, got %d", len(perms))
	}
}

func permutations(n int) [][]int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	var out [][]int
	var rec func(k int)
	rec = func(k int) {
		if k == n {
			out = append(out, append([]int(nil), idx...))
			return
		}
		for i := k; i < n; i++ {
			idx[k], idx[i] = idx[i], idx[k]
			rec(k + 1)
			idx[k], idx[i] = idx[i], idx[k]
		}
	}
	rec(0)
	return out
}

func TestDuplicateEventsCollapse(t *testing.T) {
	e1 := taskCreated(t, 1, "t1", "fix login")
	s := replay(t, []event.Event{e1, e1, e1}, nil)
	if len(s.TaskOrder) != 1 {
		t.Fatalf("duplicates did not collapse: %d tasks", len(s.TaskOrder))
	}
}

func TestLeaseExpiryReturnsTaskToPool(t *testing.T) {
	c1 := tick(t, 2)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
	}
	// Lease lapsed before Now: the agent died silently.
	leases := map[string]time.Time{c1: testNow.Add(-10 * time.Minute)}
	s := replay(t, events, leases)

	if got := s.Claims[c1].Status; got != ClaimExpired {
		t.Fatalf("claim status = %s, want expired", got)
	}
	if !s.Ready("t1") {
		t.Fatal("task should be back in the pool after lease expiry")
	}
	if len(s.Runs) != 1 || s.Runs[0].Outcome != event.OutcomeInterrupted || !s.Runs[0].Synthesized {
		t.Fatalf("expected one synthesized interrupted run, got %+v", s.Runs)
	}
}

func TestClaimAfterExpiredIncumbent(t *testing.T) {
	// Incumbent's lease lapsed BEFORE the newcomer claimed: newcomer
	// holds, incumbent expires with a synthesized interrupted run.
	c1, c2 := tick(t, 2), tick(t, 40)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 40, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
	}
	leases := map[string]time.Time{
		c1: base.Add(17 * time.Minute), // lapsed before c2's minute-40 claim
		c2: testNow.Add(time.Hour),
	}
	s := replay(t, events, leases)

	if got := s.Claims[c1].Status; got != ClaimExpired {
		t.Fatalf("incumbent status = %s, want expired", got)
	}
	if got := s.Claims[c2].Status; got != ClaimActive {
		t.Fatalf("newcomer status = %s, want active", got)
	}
	if ac := s.ActiveClaim("t1"); ac == nil || ac.ID != c2 {
		t.Fatalf("active claim = %+v, want %s", ac, c2)
	}
}

func TestReleaseThenReclaim(t *testing.T) {
	c2 := tick(t, 4)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimReleased, "brandon/impl-1", "t1", event.ClaimReleased{Reason: "wrong task"}),
		evt(t, 4, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
	}
	s := replay(t, events, aliveLease(c2))
	if got := s.Claims[tick(t, 2)].Status; got != ClaimReleased {
		t.Fatalf("first claim = %s, want released", got)
	}
	if got := s.Claims[c2].Status; got != ClaimActive {
		t.Fatalf("second claim = %s, want active", got)
	}
}

func TestLoserReleaseDoesNotDisturbHolder(t *testing.T) {
	c1, c2 := tick(t, 2), tick(t, 3)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "t1", event.ClaimMade{}),
		// The race loser stands down, as the protocol tells it to.
		evt(t, 4, event.TypeClaimReleased, "sarah/impl-9", "t1", event.ClaimReleased{Reason: "superseded"}),
	}
	leases := map[string]time.Time{c1: testNow.Add(time.Hour), c2: testNow.Add(time.Hour)}
	s := replay(t, events, leases)
	if ac := s.ActiveClaim("t1"); ac == nil || ac.ID != c1 {
		t.Fatalf("holder disturbed by loser's release: %+v", ac)
	}
}

func TestFinishRunDone(t *testing.T) {
	c1 := tick(t, 2)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeRunFinished, "brandon/impl-1", "t1", event.RunFinished{
			Outcome: event.OutcomeDone, Branch: "feat/login", PR: "https://example.com/pr/7",
			Summary: "fixed"}),
	}
	s := replay(t, events, nil) // no live lease needed: run finished before Now
	if got := s.Tasks["t1"].Status; got != StatusDone {
		t.Fatalf("task status = %s, want done", got)
	}
	if got := s.Claims[c1].Status; got != ClaimFinished {
		t.Fatalf("claim status = %s, want finished", got)
	}
	if len(s.Runs) != 1 || s.Runs[0].Claim != c1 || s.Runs[0].Synthesized {
		t.Fatalf("run not tied to claim: %+v", s.Runs)
	}
	if s.Ready("t1") {
		t.Fatal("done task must not be ready")
	}
}

func TestReadyRespectsDependenciesAndPriority(t *testing.T) {
	events := []event.Event{
		taskCreated(t, 1, "t1", "build daemon"),
		taskCreated(t, 2, "t2", "build views", "t1"), // depends on t1
		evt(t, 3, event.TypeTaskCreated, "brandon", "t3",
			event.TaskCreated{Title: "urgent fix", Priority: 9}),
	}
	s := replay(t, events, nil)
	if s.Ready("t2") {
		t.Fatal("t2 ready before its dependency is done")
	}
	ready := s.ReadyTasks()
	if len(ready) != 2 || ready[0].ID != "t3" || ready[1].ID != "t1" {
		t.Fatalf("ready order wrong: %+v", ready)
	}

	// Completing t1 unlocks t2.
	events = append(events,
		evt(t, 4, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 5, event.TypeRunFinished, "brandon/impl-1", "t1",
			event.RunFinished{Outcome: event.OutcomeDone}))
	s = replay(t, events, nil)
	if !s.Ready("t2") {
		t.Fatal("t2 should be ready once t1 is done")
	}
}

// ClaimBlockers names Ready's dependency and escalation clauses
// (task tuh-01KYWKT8NQ980F0NF4MN3VMT0Y): unmet dep IDs and open
// blocking escalation IDs in stored order, reported regardless of the
// task's status or claim state — those cases carry their own words at
// the call site.
func TestClaimBlockers(t *testing.T) {
	st := func(s string) *string { return &s }
	esc1, esc2 := tick(t, 10), tick(t, 11)
	claim := tick(t, 12)
	events := []event.Event{
		taskCreated(t, 1, "t-dep1", "unfinished dep one"),
		taskCreated(t, 2, "t-dep2", "unfinished dep two"),
		taskCreated(t, 3, "t-done", "finished dep"),
		taskCreated(t, 4, "t-esc", "escalation-only"),
		taskCreated(t, 5, "t-deps", "deps-only", "t-dep1", "t-dep2", "t-done", "t-ghost"),
		taskCreated(t, 6, "t-both", "deps and escalation", "t-dep1"),
		taskCreated(t, 7, "t-held", "parked with a dep", "t-dep1"),
		taskCreated(t, 8, "t-claimed", "claimed with a dep", "t-dep1"),
		// t-done completes; the shelved task pauses; t-claimed is held.
		evt(t, 9, event.TypeClaimMade, "brandon/impl-1", "t-done", event.ClaimMade{}),
		evt(t, 10, event.TypeEscalationRaised, "brandon/impl-1", "t-esc",
			event.EscalationRaised{Question: "which way?", Blocking: true}),
		evt(t, 11, event.TypeEscalationRaised, "brandon/impl-1", "t-both",
			event.EscalationRaised{Question: "and this?", Blocking: true}),
		evt(t, 12, event.TypeClaimMade, "brandon/impl-2", "t-claimed", event.ClaimMade{}),
		evt(t, 13, event.TypeRunFinished, "brandon/impl-1", "t-done",
			event.RunFinished{Outcome: event.OutcomeDone}),
		evt(t, 14, event.TypeTaskUpdated, "brandon", "t-held",
			event.TaskUpdated{Status: st(StatusHeld)}),
	}
	s := replay(t, events, aliveLease(claim))
	tests := []struct {
		name, task string
		deps, escs []string
	}{
		{"escalation-only", "t-esc", nil, []string{esc1}},
		// Done deps and deps unknown to state drop out; unmet ones keep
		// their stored order.
		{"deps-only", "t-deps", []string{"t-dep1", "t-dep2"}, nil},
		{"both at once", "t-both", []string{"t-dep1"}, []string{esc2}},
		{"not open still reports its dep", "t-held", []string{"t-dep1"}, nil},
		{"actively claimed still reports its dep", "t-claimed", []string{"t-dep1"}, nil},
		{"ready task has no blockers", "t-dep1", nil, nil},
		{"unknown task", "t-nope", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, escs := s.ClaimBlockers(tt.task)
			if !reflect.DeepEqual(deps, tt.deps) || !reflect.DeepEqual(escs, tt.escs) {
				t.Errorf("ClaimBlockers(%s) = %v, %v; want %v, %v",
					tt.task, deps, escs, tt.deps, tt.escs)
			}
		})
	}
}

// Blockage is the annotated "why is this blocked" answer every surface
// reads (2026-08-05 edge grill, tuh-01KZ9Y3THHH5B8GT22T1A1WPYP):
// ClaimBlockers' lists plus loop membership among not-done tasks and
// cancelled-dep marking. Loops arrive by set-union merge of two
// individually-acyclic writes, so the fixtures build them from raw
// creation events — exactly the shape replay meets after such a merge.
func TestBlockage(t *testing.T) {
	st := func(s string) *string { return &s }
	events := []event.Event{
		// A two-task loop.
		taskCreated(t, 1, "t-2a", "two-loop a", "t-2b"),
		taskCreated(t, 2, "t-2b", "two-loop b", "t-2a"),
		// A three-task loop, plus a tail hanging off it.
		taskCreated(t, 3, "t-3a", "three-loop a", "t-3b"),
		taskCreated(t, 4, "t-3b", "three-loop b", "t-3c"),
		taskCreated(t, 5, "t-3c", "three-loop c", "t-3a"),
		taskCreated(t, 6, "t-tail", "tail into the loop", "t-3a"),
		// A cancelled dep and a live one on the same waiter.
		taskCreated(t, 7, "t-gone", "the cancelled dep"),
		taskCreated(t, 8, "t-live", "the live dep"),
		taskCreated(t, 9, "t-wait", "waits on both", "t-gone", "t-live"),
		// A structural loop with a done member: the done edge is
		// satisfied, so nothing is cyclic.
		taskCreated(t, 10, "t-da", "done-loop a", "t-db"),
		taskCreated(t, 11, "t-db", "done-loop b", "t-da"),
		// A loop with a cancelled member: cancelled never counts as
		// done, so the loop stands and the dep is marked cancelled too.
		taskCreated(t, 12, "t-ca", "cancelled-loop a", "t-cb"),
		taskCreated(t, 13, "t-cb", "cancelled-loop b", "t-ca"),
		evt(t, 14, event.TypeTaskUpdated, "brandon", "t-gone",
			event.TaskUpdated{Status: st(StatusCancelled)}),
		evt(t, 15, event.TypeTaskUpdated, "brandon", "t-db",
			event.TaskUpdated{Status: st(StatusDone)}),
		evt(t, 16, event.TypeTaskUpdated, "brandon", "t-cb",
			event.TaskUpdated{Status: st(StatusCancelled)}),
	}
	s := replay(t, events, nil)
	tests := []struct {
		name, task string
		want       Blockage
	}{
		{"two-loop member a", "t-2a", Blockage{UnmetDeps: []string{"t-2b"}, Cyclic: true}},
		{"two-loop member b", "t-2b", Blockage{UnmetDeps: []string{"t-2a"}, Cyclic: true}},
		{"three-loop member a", "t-3a", Blockage{UnmetDeps: []string{"t-3b"}, Cyclic: true}},
		{"three-loop member b", "t-3b", Blockage{UnmetDeps: []string{"t-3c"}, Cyclic: true}},
		{"three-loop member c", "t-3c", Blockage{UnmetDeps: []string{"t-3a"}, Cyclic: true}},
		// The tail waits on the loop but is not in it: only the members
		// carry the marker a human must cut an edge between.
		{"tail off a loop is not cyclic", "t-tail", Blockage{UnmetDeps: []string{"t-3a"}}},
		{"cancelled dep marked, live dep not", "t-wait",
			Blockage{UnmetDeps: []string{"t-gone", "t-live"}, CancelledDeps: []string{"t-gone"}}},
		{"loop through a done task is no loop", "t-da", Blockage{}},
		{"cancelled member keeps the loop alive", "t-ca",
			Blockage{UnmetDeps: []string{"t-cb"}, CancelledDeps: []string{"t-cb"}, Cyclic: true}},
		{"unknown task", "t-nope", Blockage{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Blockage(tt.task); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Blockage(%s) = %+v, want %+v", tt.task, got, tt.want)
			}
		})
	}
}

// Situation is the one classifier (task tuh-01KZ0ES83SFH6MKWP82YRXWQD6):
// ready / in_progress / blocked for open tasks, the status word itself
// for everything else, "" for unknown IDs. Tested table-driven next to
// Ready so the words and the predicates can never drift apart.
func TestSituation(t *testing.T) {
	st := func(s string) *string { return &s }
	claim := tick(t, 9)
	events := []event.Event{
		taskCreated(t, 1, "t-ready", "no blockers"),
		taskCreated(t, 2, "t-dep", "unmet dependency", "t-ready"),
		taskCreated(t, 3, "t-esc", "escalation-blocked"),
		taskCreated(t, 4, "t-claimed", "actively claimed"),
		taskCreated(t, 5, "t-held", "parked"),
		taskCreated(t, 6, "t-inbox", "captured"),
		taskCreated(t, 7, "t-done", "finished"),
		taskCreated(t, 8, "t-cancelled", "abandoned"),
		evt(t, 9, event.TypeClaimMade, "brandon/impl-1", "t-claimed", event.ClaimMade{}),
		evt(t, 10, event.TypeEscalationRaised, "brandon/impl-1", "t-esc",
			event.EscalationRaised{Question: "which way?", Blocking: true}),
		evt(t, 11, event.TypeTaskUpdated, "brandon", "t-held",
			event.TaskUpdated{Status: st(StatusHeld)}),
		evt(t, 12, event.TypeTaskUpdated, "brandon", "t-inbox",
			event.TaskUpdated{Status: st(StatusInbox)}),
		evt(t, 13, event.TypeClaimMade, "brandon/impl-2", "t-done", event.ClaimMade{}),
		evt(t, 14, event.TypeRunFinished, "brandon/impl-2", "t-done",
			event.RunFinished{Outcome: event.OutcomeDone}),
		evt(t, 15, event.TypeTaskUpdated, "brandon", "t-cancelled",
			event.TaskUpdated{Status: st(StatusCancelled)}),
	}
	s := replay(t, events, aliveLease(claim))
	tests := []struct {
		name, task, want string
	}{
		{"open with no blockers", "t-ready", SituationReady},
		{"open with an unmet dep", "t-dep", SituationBlocked},
		{"open with a blocking escalation", "t-esc", SituationBlocked},
		{"open and actively claimed", "t-claimed", SituationInProgress},
		{"held is its status word", "t-held", StatusHeld},
		{"inbox is its status word", "t-inbox", StatusInbox},
		{"done is its status word", "t-done", StatusDone},
		{"cancelled is its status word", "t-cancelled", StatusCancelled},
		{"unknown task", "t-nope", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Situation(tt.task); got != tt.want {
				t.Errorf("Situation(%s) = %q, want %q", tt.task, got, tt.want)
			}
		})
	}
}

func TestEscalationLifecycle(t *testing.T) {
	esc := tick(t, 2)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeEscalationRaised, "brandon/impl-1", "t1",
			event.EscalationRaised{Question: "retry rate limits?", Blocking: true}),
	}
	s := replay(t, events, nil)
	if open := s.OpenEscalations(); len(open) != 1 || open[0].ID != esc {
		t.Fatalf("open escalations = %+v", open)
	}

	events = append(events,
		evt(t, 3, event.TypeEscalationAnswered, "brandon", "t1",
			event.EscalationAnswered{Answer: "yes", Escalation: esc}),
		// A later amendment wins (answers are curation, unlike claims).
		evt(t, 4, event.TypeEscalationAnswered, "brandon", "t1",
			event.EscalationAnswered{Answer: "yes, with backoff", Escalation: esc}))
	s = replay(t, events, nil)
	if len(s.OpenEscalations()) != 0 {
		t.Fatal("answered escalation still open")
	}
	if got := s.Escalations[esc].Answer; got != "yes, with backoff" {
		t.Fatalf("answer = %q, want the amendment", got)
	}
}

// Attribution of answers (T5 relay_answer, 2026-07-30 revision): the
// payload's answered_by wins over the envelope actor when present, and
// an envelope actor differing from it is recorded as the relay. Events
// from before the field existed (empty answered_by) keep attributing to
// the envelope actor.
func TestAnswerAttribution(t *testing.T) {
	esc := tick(t, 2)
	base := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeEscalationRaised, "brandon/impl-1", "t1",
			event.EscalationRaised{Question: "which auth flow?", Blocking: true}),
	}

	// Relayed: agent on the envelope, root human in the payload.
	relayed := append(base, evt(t, 3, event.TypeEscalationAnswered, "brandon/impl-1", "t1",
		event.EscalationAnswered{Answer: "oauth", AnsweredBy: "brandon", Escalation: esc}))
	s := replay(t, relayed, nil)
	e := s.Escalations[esc]
	if e.AnsweredBy != "brandon" || e.RelayedBy != "brandon/impl-1" {
		t.Fatalf("relayed answer: answered_by=%q relayed_by=%q, want brandon / brandon/impl-1",
			e.AnsweredBy, e.RelayedBy)
	}
	if !s.Ready("t1") {
		t.Fatal("relayed answer must unblock readiness exactly like a steering answer")
	}

	// Steering surface: actor answers as themselves — no relay marker.
	direct := append(base, evt(t, 3, event.TypeEscalationAnswered, "brandon", "t1",
		event.EscalationAnswered{Answer: "oauth", AnsweredBy: "brandon", Escalation: esc}))
	e = replay(t, direct, nil).Escalations[esc]
	if e.AnsweredBy != "brandon" || e.RelayedBy != "" {
		t.Fatalf("direct answer: answered_by=%q relayed_by=%q, want brandon / empty",
			e.AnsweredBy, e.RelayedBy)
	}

	// Pre-field event: empty answered_by falls back to the envelope actor.
	old := append(base, evt(t, 3, event.TypeEscalationAnswered, "brandon", "t1",
		event.EscalationAnswered{Answer: "oauth", Escalation: esc}))
	e = replay(t, old, nil).Escalations[esc]
	if e.AnsweredBy != "brandon" || e.RelayedBy != "" {
		t.Fatalf("legacy answer: answered_by=%q relayed_by=%q, want brandon / empty",
			e.AnsweredBy, e.RelayedBy)
	}

	// A later direct amendment wins and clears the relay marker.
	amended := append(relayed, evt(t, 4, event.TypeEscalationAnswered, "sarah", "t1",
		event.EscalationAnswered{Answer: "saml, actually", AnsweredBy: "sarah", Escalation: esc}))
	e = replay(t, amended, nil).Escalations[esc]
	if e.Answer != "saml, actually" || e.AnsweredBy != "sarah" || e.RelayedBy != "" {
		t.Fatalf("amended answer = %q by %q relayed %q, want the amendment by sarah, no relay",
			e.Answer, e.AnsweredBy, e.RelayedBy)
	}
}

func TestBlockingEscalationGatesReadiness(t *testing.T) {
	esc := tick(t, 2)
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeEscalationRaised, "brandon/impl-1", "t1",
			event.EscalationRaised{Question: "which auth flow?", Blocking: true}),
	}
	s := replay(t, events, nil)
	if s.Ready("t1") {
		t.Fatal("task with an open blocking escalation must not be ready")
	}

	// A non-blocking question does not gate readiness.
	events2 := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeEscalationRaised, "brandon/impl-1", "t1",
			event.EscalationRaised{Question: "fyi ok?", Blocking: false}),
	}
	if s2 := replay(t, events2, nil); !s2.Ready("t1") {
		t.Fatal("non-blocking escalation must not gate readiness")
	}

	// The answer returns the task to the pool.
	events = append(events, evt(t, 3, event.TypeEscalationAnswered, "brandon", "t1",
		event.EscalationAnswered{Answer: "oauth", Escalation: esc}))
	if s3 := replay(t, events, nil); !s3.Ready("t1") {
		t.Fatal("answered escalation must return the task to the pool")
	}
}

// The status model (2026-07-31): inbox and held join open/done/cancelled.
// Only open is claimable; transitions are mechanically permissive —
// replay validates vocabulary, never paths.
func TestCreateWithStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  string // payload status on task.created
		want    string // replayed task status
		ready   bool
		wantErr error // non-nil: replay must stop with this sentinel
	}{
		{"default open", "", StatusOpen, true, nil},
		{"explicit open", StatusOpen, StatusOpen, true, nil},
		{"inbox capture", StatusInbox, StatusInbox, false, nil},
		{"born held", StatusHeld, StatusHeld, false, nil},
		// Permissive vocabulary: replay accepts any known status at
		// create (the write surfaces gate what gets written).
		{"born done", StatusDone, StatusDone, false, nil},
		{"unknown status", "someday", "", false, ErrMalformedEvent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []event.Event{evt(t, 1, event.TypeTaskCreated, "brandon", "t1",
				event.TaskCreated{Title: "captured", Status: tt.status})}
			s, err := NewReplayer().Replay(Input{Events: events, Now: testNow})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("replay: %v", err)
			}
			if got := s.Tasks["t1"].Status; got != tt.want {
				t.Errorf("status = %q, want %q", got, tt.want)
			}
			if got := s.Ready("t1"); got != tt.ready {
				t.Errorf("Ready = %v, want %v", got, tt.ready)
			}
		})
	}
}

// A v1 task.created (no status field) still replays as open through the
// identity upcaster — the payload's absent status is the v1 reading.
func TestV1TaskCreatedReplaysOpen(t *testing.T) {
	old := event.Event{ID: tick(t, 1), Type: event.TypeTaskCreated, V: 1,
		Actor: "brandon", Machine: "m-test", Task: "t1",
		Data: json.RawMessage(`{"title":"pre-inbox era"}`)}
	s := replay(t, []event.Event{old}, nil)
	if got := s.Tasks["t1"].Status; got != StatusOpen {
		t.Fatalf("v1 task status = %q, want open", got)
	}
	if !s.Ready("t1") {
		t.Fatal("v1 task should be ready")
	}
}

// The parents edge is retired (edge grill, 2026-08-05): stored payloads
// that still carry a "parents" array replay without error — readers
// ignore fields they have no place for (T3), no version bump, stored
// bytes untouched — and every remaining field lands as usual.
func TestRetiredParentsFieldTolerated(t *testing.T) {
	created := event.Event{ID: tick(t, 1), Type: event.TypeTaskCreated, V: 2,
		Actor: "brandon", Machine: "m-test", Task: "t1",
		Data: json.RawMessage(`{"title":"child of an epic","status":"open","priority":3,"depends_on":["t0"],"parents":["t9"]}`)}
	updated := event.Event{ID: tick(t, 2), Type: event.TypeTaskUpdated, V: 2,
		Actor: "brandon", Machine: "m-test", Task: "t1",
		Data: json.RawMessage(`{"title":"renamed","parents":["t8","t9"]}`)}

	tests := map[string]struct {
		events    []event.Event
		wantTitle string
	}{
		"task.created carries parents": {[]event.Event{created}, "child of an epic"},
		"task.updated carries parents": {[]event.Event{created, updated}, "renamed"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := replay(t, tt.events, nil)
			task := s.Tasks["t1"]
			if task == nil {
				t.Fatal("t1 missing after replay")
			}
			if task.Title != tt.wantTitle || task.Status != StatusOpen || task.Priority != 3 {
				t.Fatalf("task = %+v, want title %q, open, priority 3", task, tt.wantTitle)
			}
			if !reflect.DeepEqual(task.DependsOn, []string{"t0"}) {
				t.Fatalf("depends_on = %v, want [t0]", task.DependsOn)
			}
		})
	}
}

// Promote/pause/resume round-trips: inbox→open→held→open, all through
// ordinary task.updated events, with readiness following the status.
func TestStatusRoundTrips(t *testing.T) {
	st := func(s string) *string { return &s }
	events := []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "t1",
			event.TaskCreated{Title: "an idea", Status: StatusInbox}),
	}
	s := replay(t, events, nil)
	if s.Ready("t1") || len(s.ReadyTasks()) != 0 {
		t.Fatal("inbox task must not be ready or served")
	}

	events = append(events, evt(t, 2, event.TypeTaskUpdated, "brandon", "t1",
		event.TaskUpdated{Status: st(StatusOpen), Description: st("Context, ask, acceptance.")}))
	if s = replay(t, events, nil); !s.Ready("t1") {
		t.Fatal("promoted task must be ready")
	}

	events = append(events, evt(t, 3, event.TypeTaskUpdated, "brandon", "t1",
		event.TaskUpdated{Status: st(StatusHeld)}))
	if s = replay(t, events, nil); s.Ready("t1") {
		t.Fatal("held task must not be ready")
	}

	events = append(events, evt(t, 4, event.TypeTaskUpdated, "brandon", "t1",
		event.TaskUpdated{Status: st(StatusOpen)}))
	if s = replay(t, events, nil); !s.Ready("t1") {
		t.Fatal("resumed task must be ready again")
	}
}

// A dependency sitting in inbox or held blocks its dependents exactly
// like any other not-done task — captures participate in the DAG.
func TestInboxHeldDependenciesBlock(t *testing.T) {
	st := func(s string) *string { return &s }
	events := []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "t-idea",
			event.TaskCreated{Title: "the idea", Status: StatusInbox}),
		taskCreated(t, 2, "t-build", "build on the idea", "t-idea"),
	}
	if s := replay(t, events, nil); s.Ready("t-build") {
		t.Fatal("task depending on an inbox capture must not be ready")
	}
	events = append(events, evt(t, 3, event.TypeTaskUpdated, "brandon", "t-idea",
		event.TaskUpdated{Status: st(StatusHeld)}))
	if s := replay(t, events, nil); s.Ready("t-build") {
		t.Fatal("task depending on a held task must not be ready")
	}
	events = append(events,
		evt(t, 4, event.TypeTaskUpdated, "brandon", "t-idea", event.TaskUpdated{Status: st(StatusOpen)}),
		evt(t, 5, event.TypeClaimMade, "brandon/impl-1", "t-idea", event.ClaimMade{}),
		evt(t, 6, event.TypeRunFinished, "brandon/impl-1", "t-idea",
			event.RunFinished{Outcome: event.OutcomeDone}))
	if s := replay(t, events, nil); !s.Ready("t-build") {
		t.Fatal("dependent must be ready once the promoted dependency is done")
	}
}

// An unknown status in task.updated stops replay as malformed — the
// same fail-safe posture the pre-inbox binary had (verified 2026-07-31),
// now paired with the v2 schema bump so old binaries stop with "upgrade
// tuhdoo" instead of "malformed" when they meet the new values.
func TestUnknownStatusUpdateStopsReplay(t *testing.T) {
	st := "someday"
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeTaskUpdated, "brandon", "t1", event.TaskUpdated{Status: &st}),
	}
	_, err := NewReplayer().Replay(Input{Events: events, Now: testNow})
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("err = %v, want ErrMalformedEvent", err)
	}
}

// What a v1-only binary sees when a v2 binary has written: the exact
// old-binary fail-safe this cycle's version bump exists to guarantee. A
// replayer stripped of the standard v1→v2 upcasters stands in for the
// old binary meeting bytes from the future — replay stops with
// ErrCannotReplay ("upgrade tuhdoo"), never a mis-bucketed open task.
func TestV2EventsFailSafeWithoutUpcasters(t *testing.T) {
	v2 := evt(t, 1, event.TypeTaskCreated, "brandon", "t1",
		event.TaskCreated{Title: "captured", Status: StatusInbox})
	if v2.V != 2 {
		t.Fatalf("task.created writes v%d, want v2", v2.V)
	}
	// An old binary has no idea v2 exists: its Versions map says 1, so
	// upcast refuses upward. Simulate by asking this binary about v3 —
	// the same "version above mine" gate the old binary hits at v2.
	future := v2
	future.V = 3
	_, err := NewReplayer().Replay(Input{Events: []event.Event{future}, Now: testNow})
	if !errors.Is(err, ErrCannotReplay) {
		t.Fatalf("err = %v, want ErrCannotReplay", err)
	}
}

func TestFailSafeUnknownType(t *testing.T) {
	bad := event.Event{ID: tick(t, 2), Type: "task.frobnicated", V: 1,
		Actor: "future/agent", Machine: "m-x", Task: "t1",
		Data: json.RawMessage(`{}`)}
	events := []event.Event{taskCreated(t, 1, "t1", "fix login"), bad}

	_, err := NewReplayer().Replay(Input{Events: events, Now: testNow})
	if !errors.Is(err, ErrCannotReplay) {
		t.Fatalf("err = %v, want ErrCannotReplay", err)
	}
	var re *ReplayError
	if !errors.As(err, &re) || re.EventID != bad.ID {
		t.Fatalf("error does not name the offending event: %v", err)
	}
}

func TestFailSafeNewerVersion(t *testing.T) {
	e := evt(t, 2, event.TypeNoteAdded, "future/agent", "t1", event.NoteAdded{Text: "hi"})
	e.V = 2
	events := []event.Event{taskCreated(t, 1, "t1", "fix login"), e}
	_, err := NewReplayer().Replay(Input{Events: events, Now: testNow})
	if !errors.Is(err, ErrCannotReplay) {
		t.Fatalf("err = %v, want ErrCannotReplay", err)
	}
}

func TestMalformedReferenceStopsReplay(t *testing.T) {
	events := []event.Event{
		evt(t, 1, event.TypeNoteAdded, "brandon", "t-ghost", event.NoteAdded{Text: "?"}),
	}
	_, err := NewReplayer().Replay(Input{Events: events, Now: testNow})
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("err = %v, want ErrMalformedEvent", err)
	}
}

func TestUpcasterLiftsOldEvents(t *testing.T) {
	// A v0 task.created wrote "name" where v1 says "title". The upcaster
	// lifts it in memory; stored bytes are untouched by construction.
	old := event.Event{ID: tick(t, 1), Type: event.TypeTaskCreated, V: 0,
		Actor: "brandon", Machine: "m-test", Task: "t1",
		Data: json.RawMessage(`{"name":"fix login"}`)}

	r := NewReplayer()
	r.RegisterUpcaster(event.TypeTaskCreated, 0, func(data json.RawMessage) (json.RawMessage, error) {
		var v0 map[string]json.RawMessage
		if err := json.Unmarshal(data, &v0); err != nil {
			return nil, err
		}
		v0["title"] = v0["name"]
		delete(v0, "name")
		return json.Marshal(v0)
	})

	s, err := r.Replay(Input{Events: []event.Event{old}, Now: testNow})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := s.Tasks["t1"].Title; got != "fix login" {
		t.Fatalf("title = %q after upcast", got)
	}

	// Without the upcaster the same event is honestly refused.
	if _, err := NewReplayer().Replay(Input{Events: []event.Event{old}, Now: testNow}); !errors.Is(err, ErrCannotReplay) {
		t.Fatalf("err = %v, want ErrCannotReplay without an upcaster", err)
	}
}

// ClosedAt/ClosedBy (2026-08-02, T5 read parity / history view): the
// event *entering* a terminal status is the closing event; leaving
// terminal clears the stamp; a task created directly terminal (the B12
// migration shape) closes at its creation event.
func TestClosedMetadata(t *testing.T) {
	st := func(s string) *string { return &s }
	create := func(n int, status string) event.Event {
		return evt(t, n, event.TypeTaskCreated, "brandon", "t1",
			event.TaskCreated{Title: "one task", Status: status})
	}
	update := func(n int, actor, status string) event.Event {
		return evt(t, n, event.TypeTaskUpdated, actor, "t1",
			event.TaskUpdated{Status: st(status)})
	}
	at := func(n int) time.Time { return base.Add(time.Duration(n) * time.Minute) }

	tests := []struct {
		name   string
		events []event.Event
		wantAt time.Time // zero: no close metadata
		wantBy string
	}{
		{"open task carries none",
			[]event.Event{create(1, "")}, time.Time{}, ""},
		{"update to done stamps",
			[]event.Event{create(1, ""), update(2, "brandon/impl-1", StatusDone)},
			at(2), "brandon/impl-1"},
		{"update to cancelled stamps",
			[]event.Event{create(1, ""), update(2, "brandon", StatusCancelled)},
			at(2), "brandon"},
		{"born done closes at creation",
			[]event.Event{create(1, StatusDone)}, at(1), "brandon"},
		{"born cancelled closes at creation",
			[]event.Event{create(1, StatusCancelled)}, at(1), "brandon"},
		{"leaving terminal clears",
			[]event.Event{create(1, ""), update(2, "brandon", StatusDone), update(3, "brandon", StatusOpen)},
			time.Time{}, ""},
		{"done to cancelled restamps",
			[]event.Event{create(1, ""), update(2, "sarah", StatusDone), update(3, "brandon", StatusCancelled)},
			at(3), "brandon"},
		{"re-asserting done keeps the original stamp",
			[]event.Event{create(1, ""), update(2, "sarah", StatusDone), update(3, "brandon", StatusDone)},
			at(2), "sarah"},
		{"reopen then cancel stamps the cancel",
			[]event.Event{create(1, ""), update(2, "sarah", StatusDone),
				update(3, "sarah", StatusOpen), update(4, "brandon", StatusCancelled)},
			at(4), "brandon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := replay(t, tt.events, nil).Tasks["t1"]
			if !task.ClosedAt.Equal(tt.wantAt) {
				t.Errorf("ClosedAt = %v, want %v", task.ClosedAt, tt.wantAt)
			}
			if task.ClosedBy != tt.wantBy {
				t.Errorf("ClosedBy = %q, want %q", task.ClosedBy, tt.wantBy)
			}
		})
	}
}

// A finish_run with outcome done is a status change into done, so it is
// a closing event like any other.
func TestFinishRunDoneStampsClose(t *testing.T) {
	events := []event.Event{
		taskCreated(t, 1, "t1", "fix login"),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "t1", event.ClaimMade{}),
		evt(t, 3, event.TypeRunFinished, "brandon/impl-1", "t1",
			event.RunFinished{Outcome: event.OutcomeDone}),
	}
	task := replay(t, events, nil).Tasks["t1"]
	if want := base.Add(3 * time.Minute); !task.ClosedAt.Equal(want) {
		t.Errorf("ClosedAt = %v, want %v", task.ClosedAt, want)
	}
	if task.ClosedBy != "brandon/impl-1" {
		t.Errorf("ClosedBy = %q, want brandon/impl-1", task.ClosedBy)
	}
}

// Close metadata is a fold over ULID order, so input order is
// irrelevant — every permutation lands the same stamps.
func TestClosedMetadataOrderInsensitive(t *testing.T) {
	st := func(s string) *string { return &s }
	events := []event.Event{
		taskCreated(t, 1, "t1", "one task"),
		evt(t, 2, event.TypeTaskUpdated, "sarah", "t1", event.TaskUpdated{Status: st(StatusDone)}),
		evt(t, 3, event.TypeTaskUpdated, "brandon", "t1", event.TaskUpdated{Status: st(StatusCancelled)}),
	}
	for _, p := range permutations(len(events)) {
		shuffled := make([]event.Event, len(events))
		for i, idx := range p {
			shuffled[i] = events[idx]
		}
		task := replay(t, shuffled, nil).Tasks["t1"]
		if want := base.Add(3 * time.Minute); !task.ClosedAt.Equal(want) || task.ClosedBy != "brandon" {
			t.Fatalf("perm %v: close = %v by %q, want %v by brandon", p, task.ClosedAt, task.ClosedBy, want)
		}
	}
}
