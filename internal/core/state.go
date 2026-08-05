// Package core is tuhdoo's deterministic heart: pure functions from an
// event set to project state (001 D5/D6, 002 T3). Nothing in this
// package reads a clock, touches I/O, or spawns a goroutine — time and
// lease data are inputs, and the same inputs always produce the same
// state on every machine. That property is what lets divergent branch
// histories merge by set-union and still converge (D2).
package core

import "time"

// Task statuses stored in state. "Claimed" is deliberately not a status:
// whether a task is being worked on is derived from claims (D6), so a
// crashed agent can never wedge a task in a stuck status.
//
// Only StatusOpen is ever claimable (Ready below). Inbox and held
// (2026-07-31 grill cycle) are the capture and pause tiers: "inbox" is
// never-triaged capture carrying inherent review debt; "held" passed
// triage and is workable but deliberately paused. Transitions between
// statuses are mechanically permissive — replay validates the
// vocabulary, never the path (no rejected-event edge cases, T3); the
// semantics (promote deliberately, the tasks-are-prompts bar applies at
// promotion) live in docs/agent-protocol.md, not in code.
const (
	StatusOpen      = "open"
	StatusInbox     = "inbox"
	StatusHeld      = "held"
	StatusDone      = "done"
	StatusCancelled = "cancelled"
)

// Task is the unit of intent (D5): DAG edges, not a fixed hierarchy.
type Task struct {
	ID          string
	Title       string
	Description string
	Priority    int
	Labels      []string
	DependsOn   []string // prerequisite-task edges ("epics" are container tasks that depend on their children)
	Status      string
	CreatedBy   string // actor of task.created
	CreatedAt   time.Time

	// ClosedAt/ClosedBy record the event that put the task in its
	// current terminal status (done/cancelled): stamped on entering,
	// cleared on leaving, derived at replay only — no stored-byte
	// changes (T3). A task created directly terminal (the B12
	// migration shape) closes at its creation event. Zero-valued on
	// every non-terminal task.
	ClosedAt time.Time
	ClosedBy string
}

// ClaimStatus is the replay verdict on one claim.
type ClaimStatus string

const (
	// ClaimActive: the claim holds the task and its lease is unexpired.
	ClaimActive ClaimStatus = "active"
	// ClaimVoided: lost the D6 race — another claim already held the
	// task when this one arrived.
	ClaimVoided ClaimStatus = "voided"
	// ClaimReleased: ended voluntarily (release_claim).
	ClaimReleased ClaimStatus = "released"
	// ClaimFinished: ended by a finish_run from its holder.
	ClaimFinished ClaimStatus = "finished"
	// ClaimExpired: its lease ran out with no finish or release.
	ClaimExpired ClaimStatus = "expired"
)

// Claim records one "agent X on machine Y is working on task Z" (D5).
type Claim struct {
	ID      string // the claim.made event ID
	Task    string
	Actor   string
	Machine string
	Status  ClaimStatus
	MadeAt  time.Time

	// Confirmation is the claim.confirmed event ID that made this
	// claim's verdict final (D6, 2026-08-04), empty while the verdict is
	// only provisional. A confirmation settles the race, not liveness: a
	// confirmed claim can still end (finish, release, lease expiry), and
	// the empty/non-empty distinction is what lets replay tell a
	// revocable provisional winner from an irrevocable refereed one.
	Confirmation string
}

// Run is one attempt record (D5). Most runs come from run.finished
// events; runs with Synthesized true are derived by replay for claims
// whose lease lapsed without one — outcome "interrupted" for a holder
// that went silent, "superseded" for a race loser that never reported
// (D6, 2026-08-04) — the daemon may later materialize those as real
// events, but state never waits for it.
type Run struct {
	ID          string // the run.finished event ID, or the claim ID when synthesized
	Task        string
	Claim       string
	Actor       string
	Machine     string
	Outcome     string
	Branch      string
	PR          string
	Commits     []string
	Summary     string
	Synthesized bool
}

// Escalation is an agent-raised question awaiting a human (D5).
type Escalation struct {
	ID         string // the escalation.raised event ID
	Task       string
	Actor      string
	Question   string
	Context    string
	Blocking   bool
	RaisedAt   time.Time
	Answer     string
	AnsweredBy string // whom the answer is attributed to
	RelayedBy  string // actor that recorded it, when not AnsweredBy themselves
	Answered   bool
}

// Note is a comment on a task (D5).
type Note struct {
	ID      string
	Task    string
	Actor   string
	Text    string
	AddedAt time.Time
}

// State is the full replayed project state. Slices hold IDs in ULID
// (= replay) order; maps hold the entities.
type State struct {
	Tasks        map[string]*Task
	TaskOrder    []string // creation order
	Claims       map[string]*Claim
	ClaimsByTask map[string][]string // claim IDs per task, replay order
	Runs         []Run               // replay order; synthesized runs last
	Escalations  map[string]*Escalation
	EscOrder     []string
	Notes        []Note
}

// ActiveClaim returns the active claim on a task, or nil.
func (s *State) ActiveClaim(taskID string) *Claim {
	for _, cid := range s.ClaimsByTask[taskID] {
		if c := s.Claims[cid]; c.Status == ClaimActive {
			return c
		}
	}
	return nil
}

// Ready reports whether a task can be claimed right now: open (the only
// claimable status — inbox and held tasks are never served), not
// actively claimed, every dependency done (a dependency sitting in
// inbox or held blocks naturally: it is not done), and no open blocking
// escalation. The escalation clause closes the protocol loop: escalate →
// release → finish_run(blocked) returns the task to the pool, and it is
// the *answer* that makes it claimable again — serving it earlier would
// hand an agent a task it can only re-ask the same question about.
func (s *State) Ready(taskID string) bool {
	t, ok := s.Tasks[taskID]
	if !ok || t.Status != StatusOpen {
		return false
	}
	if s.ActiveClaim(taskID) != nil {
		return false
	}
	deps, escs := s.ClaimBlockers(taskID)
	return len(deps) == 0 && len(escs) == 0
}

// ClaimBlockers names the dependency and escalation clauses of Ready
// for one task: the not-yet-done dependency IDs and the open blocking
// escalation IDs, each in stored order. It reports these blockers
// regardless of the task's status or claim state — not-open and
// already-claimed have their own words, owned by the caller — and
// returns nothing for an unknown task. Ready consumes it, so the two
// can never disagree about what blocks a claim.
func (s *State) ClaimBlockers(taskID string) (unmetDeps, blockingEscalations []string) {
	t, ok := s.Tasks[taskID]
	if !ok {
		return nil, nil
	}
	for _, dep := range t.DependsOn {
		if d, ok := s.Tasks[dep]; ok && d.Status != StatusDone {
			unmetDeps = append(unmetDeps, dep)
		}
	}
	for _, id := range s.EscOrder {
		if e := s.Escalations[id]; e.Task == taskID && e.Blocking && !e.Answered {
			blockingEscalations = append(blockingEscalations, id)
		}
	}
	return unmetDeps, blockingEscalations
}

// Situation words for open tasks. Non-open tasks have no extra word:
// their situation is their status.
const (
	SituationReady      = "ready"
	SituationInProgress = "in_progress"
	SituationBlocked    = "blocked"
)

// Situation is the one classifier (2026-08-03 grill): the derived
// bucket every surface files a task under, one word per task. Open
// tasks split into ready / in_progress / blocked; for every other
// status the situation is the status word itself, so consumers switch
// on a single field. Unknown tasks get "". Computed from state at read
// time and never stored — there is no second copy to go stale.
func (s *State) Situation(taskID string) string {
	t, ok := s.Tasks[taskID]
	if !ok {
		return ""
	}
	if t.Status != StatusOpen {
		return t.Status
	}
	if s.ActiveClaim(taskID) != nil {
		return SituationInProgress
	}
	if s.Ready(taskID) {
		return SituationReady
	}
	return SituationBlocked
}

// ReadyTasks returns claimable tasks, highest priority first, ULID order
// within a priority. This is the ordering claim_next serves from.
func (s *State) ReadyTasks() []*Task {
	var ready []*Task
	for _, id := range s.TaskOrder {
		if s.Ready(id) {
			ready = append(ready, s.Tasks[id])
		}
	}
	// Stable by construction: TaskOrder is ULID order, and we only ever
	// move higher priorities forward.
	for i := 1; i < len(ready); i++ {
		for j := i; j > 0 && ready[j].Priority > ready[j-1].Priority; j-- {
			ready[j], ready[j-1] = ready[j-1], ready[j]
		}
	}
	return ready
}

// OpenEscalations returns unanswered escalations in raise order — the
// steering inbox.
func (s *State) OpenEscalations() []*Escalation {
	var open []*Escalation
	for _, id := range s.EscOrder {
		if e := s.Escalations[id]; !e.Answered {
			open = append(open, e)
		}
	}
	return open
}
