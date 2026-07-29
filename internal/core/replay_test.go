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

func evt(t *testing.T, n int, typ, actor, task string, payload any) event.Event {
	t.Helper()
	e, err := event.New(tick(t, n), typ, 1, actor, "m-test", task, payload)
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
