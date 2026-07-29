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
const (
	StatusOpen      = "open"
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
	Parents     []string // parent-task edges ("epics" are just tasks)
	DependsOn   []string // prerequisite-task edges
	Status      string
	CreatedBy   string // actor of task.created
	CreatedAt   time.Time
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
}

// Run is one attempt record (D5). Most runs come from run.finished
// events; runs with Synthesized true are derived by replay for claims
// that expired without one (outcome "interrupted") — the daemon may
// later materialize those as real events, but state never waits for it.
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
	AnsweredBy string
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

// Ready reports whether a task can be claimed right now: open, not
// actively claimed, and every dependency done.
func (s *State) Ready(taskID string) bool {
	t, ok := s.Tasks[taskID]
	if !ok || t.Status != StatusOpen {
		return false
	}
	if s.ActiveClaim(taskID) != nil {
		return false
	}
	for _, dep := range t.DependsOn {
		if d, ok := s.Tasks[dep]; ok && d.Status != StatusDone {
			return false
		}
	}
	return true
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
