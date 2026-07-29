package core

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// Sentinel failures, matched with errors.Is. Both stop replay entirely:
// a partial state computed by skipping events is how two machines end up
// with different truths from the same log (T3 — fail-safe, never
// best-effort).
var (
	// ErrCannotReplay: an event from a newer daemon (unknown type, or a
	// version above what this binary understands, or below it with no
	// upcaster ladder). Remedy: upgrade tuhdoo.
	ErrCannotReplay = errors.New("cannot honestly replay")

	// ErrMalformedEvent: an event that violates the data contract
	// (missing subject, reference to a nonexistent entity). Remedy:
	// human investigation — something wrote garbage.
	ErrMalformedEvent = errors.New("malformed event")
)

// ReplayError says which event stopped replay and why.
type ReplayError struct {
	EventID  string
	Type     string
	V        int
	Sentinel error // ErrCannotReplay or ErrMalformedEvent
	Reason   string
}

func (e *ReplayError) Error() string {
	return fmt.Sprintf("replay stopped at event %s (%s v%d): %s: %s",
		e.EventID, e.Type, e.V, e.Sentinel, e.Reason)
}

func (e *ReplayError) Unwrap() error { return e.Sentinel }

// Input is everything replay may depend on. Leases live outside the
// event log (D9: mutable files) and clocks are never read in here, so
// both arrive as data: same Input, same State, on every machine.
type Input struct {
	Events []event.Event
	Leases map[string]time.Time // claim ID → lease expiry
	Now    time.Time
}

// Replayer folds an event set into State. The zero value is not usable;
// construct with NewReplayer. Upcasters registered on it lift old-schema
// payloads to current in memory only — stored bytes are never rewritten.
type Replayer struct {
	upcasters map[upKey]Upcaster
}

func NewReplayer() *Replayer {
	return &Replayer{upcasters: make(map[upKey]Upcaster)}
}

// Replay computes state from an event set. Events are treated as a set:
// duplicates (by ID) collapse, and input order is irrelevant — replay
// sorts by ID (ULID lexical order, the one total order every machine
// agrees on, D6/T3).
func (r *Replayer) Replay(in Input) (*State, error) {
	events := dedupeAndSort(in.Events)

	s := &State{
		Tasks:        make(map[string]*Task),
		Claims:       make(map[string]*Claim),
		ClaimsByTask: make(map[string][]string),
		Escalations:  make(map[string]*Escalation),
	}
	// holder[task] is the claim currently holding the task, if any.
	holder := make(map[string]*Claim)
	var synthesized []Run

	for i := range events {
		e, err := r.upcast(events[i])
		if err != nil {
			return nil, err
		}
		if err := apply(s, holder, &synthesized, in.Leases, e); err != nil {
			return nil, err
		}
	}

	// Final lease check: a holder whose lease is gone or past Now has
	// expired — the task returns to the pool and the abandoned attempt
	// becomes a synthesized "interrupted" run (T5: never assume an
	// agent's last act was tidy).
	for _, c := range holder {
		if c == nil || c.Status != ClaimActive {
			continue
		}
		exp, ok := in.Leases[c.ID]
		if !ok || !exp.After(in.Now) {
			c.Status = ClaimExpired
			synthesized = append(synthesized, interruptedRun(c))
		}
	}

	// Synthesized runs sort after real ones, in claim order (still
	// deterministic: claim IDs are ULIDs).
	sort.Slice(synthesized, func(i, j int) bool { return synthesized[i].Claim < synthesized[j].Claim })
	s.Runs = append(s.Runs, synthesized...)
	return s, nil
}

func dedupeAndSort(in []event.Event) []event.Event {
	seen := make(map[string]bool, len(in))
	out := make([]event.Event, 0, len(in))
	for _, e := range in {
		if !seen[e.ID] {
			seen[e.ID] = true
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// apply folds one event into state. holder and synthesized are the
// claim-tracking working set shared across the fold.
func apply(s *State, holder map[string]*Claim, synthesized *[]Run, leases map[string]time.Time, e event.Event) error {
	malformed := func(format string, args ...any) error {
		return &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
			Sentinel: ErrMalformedEvent, Reason: fmt.Sprintf(format, args...)}
	}
	if e.Task == "" {
		return malformed("event has no subject task")
	}
	when, err := event.IDTime(e.ID)
	if err != nil {
		return malformed("unparseable id: %v", err)
	}

	switch e.Type {
	case event.TypeTaskCreated:
		var p event.TaskCreated
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		if _, exists := s.Tasks[e.Task]; exists {
			return malformed("task %s already exists", e.Task)
		}
		s.Tasks[e.Task] = &Task{
			ID: e.Task, Title: p.Title, Description: p.Description,
			Priority: p.Priority, Labels: p.Labels,
			Parents: p.Parents, DependsOn: p.DependsOn,
			Status: StatusOpen, CreatedBy: e.Actor, CreatedAt: when,
		}
		s.TaskOrder = append(s.TaskOrder, e.Task)

	case event.TypeTaskUpdated:
		var p event.TaskUpdated
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		t, ok := s.Tasks[e.Task]
		if !ok {
			return malformed("update of unknown task %s", e.Task)
		}
		// Field-level last-writer-wins, in ULID order (D6's logical
		// ordering applied to curation).
		if p.Title != nil {
			t.Title = *p.Title
		}
		if p.Description != nil {
			t.Description = *p.Description
		}
		if p.Status != nil {
			switch *p.Status {
			case StatusOpen, StatusDone, StatusCancelled:
				t.Status = *p.Status
			default:
				return malformed("unknown task status %q", *p.Status)
			}
		}
		if p.Priority != nil {
			t.Priority = *p.Priority
		}
		if p.Labels != nil {
			t.Labels = *p.Labels
		}
		if p.Parents != nil {
			t.Parents = *p.Parents
		}
		if p.DependsOn != nil {
			t.DependsOn = *p.DependsOn
		}

	case event.TypeClaimMade:
		var p event.ClaimMade
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		if _, ok := s.Tasks[e.Task]; !ok {
			return malformed("claim on unknown task %s", e.Task)
		}
		c := &Claim{ID: e.ID, Task: e.Task, Actor: e.Actor,
			Machine: e.Machine, MadeAt: when}
		s.Claims[e.ID] = c
		s.ClaimsByTask[e.Task] = append(s.ClaimsByTask[e.Task], e.ID)

		switch h := holder[e.Task]; {
		case h == nil:
			c.Status = ClaimActive
			holder[e.Task] = c
		case leaseExpiredBy(leases, h.ID, when):
			// The incumbent's lease had lapsed by the time this claim
			// was made: the incumbent expired, the newcomer holds.
			// Deterministic — compares stored lease data to the ULID
			// timestamp, never to a live clock.
			h.Status = ClaimExpired
			*synthesized = append(*synthesized, interruptedRun(h))
			c.Status = ClaimActive
			holder[e.Task] = c
		default:
			// D6 winner rule: earliest claim wins, later claims are
			// voided. No human, no prompt — replay code is the referee.
			c.Status = ClaimVoided
		}

	case event.TypeClaimReleased:
		var p event.ClaimReleased
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		// Only the holder's own release ends the hold. A release from a
		// voided claimant (a race loser standing down) is a no-op, not
		// an error.
		if h := holder[e.Task]; h != nil && h.Status == ClaimActive && h.Actor == e.Actor {
			h.Status = ClaimReleased
			holder[e.Task] = nil
		}

	case event.TypeRunFinished:
		var p event.RunFinished
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		t, ok := s.Tasks[e.Task]
		if !ok {
			return malformed("run on unknown task %s", e.Task)
		}
		switch p.Outcome {
		case event.OutcomeDone, event.OutcomeFailed, event.OutcomeAbandoned,
			event.OutcomeBlocked, event.OutcomeInterrupted, event.OutcomeSuperseded:
		default:
			return malformed("unknown run outcome %q", p.Outcome)
		}
		run := Run{ID: e.ID, Task: e.Task, Actor: e.Actor, Machine: e.Machine,
			Outcome: p.Outcome, Branch: p.Branch, PR: p.PR,
			Commits: p.Commits, Summary: p.Summary}
		if h := holder[e.Task]; h != nil && h.Status == ClaimActive && h.Actor == e.Actor {
			run.Claim = h.ID
			h.Status = ClaimFinished
			holder[e.Task] = nil
			if p.Outcome == event.OutcomeDone {
				t.Status = StatusDone
			}
		}
		// A run from a non-holder (race loser recording superseded
		// work, or a daemon closing an interrupted attempt) is recorded
		// but moves neither the hold nor the task status.
		s.Runs = append(s.Runs, run)

	case event.TypeEscalationRaised:
		var p event.EscalationRaised
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		if _, ok := s.Tasks[e.Task]; !ok {
			return malformed("escalation on unknown task %s", e.Task)
		}
		s.Escalations[e.ID] = &Escalation{ID: e.ID, Task: e.Task,
			Actor: e.Actor, Question: p.Question, Context: p.Context,
			Blocking: p.Blocking, RaisedAt: when}
		s.EscOrder = append(s.EscOrder, e.ID)

	case event.TypeEscalationAnswered:
		var p event.EscalationAnswered
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		esc, ok := s.Escalations[p.Escalation]
		if !ok {
			return malformed("answer to unknown escalation %s", p.Escalation)
		}
		// Last answer wins: answers are human amendments, unlike claims.
		esc.Answer = p.Answer
		esc.AnsweredBy = e.Actor
		esc.Answered = true

	case event.TypeNoteAdded:
		var p event.NoteAdded
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		if _, ok := s.Tasks[e.Task]; !ok {
			return malformed("note on unknown task %s", e.Task)
		}
		s.Notes = append(s.Notes, Note{ID: e.ID, Task: e.Task,
			Actor: e.Actor, Text: p.Text, AddedAt: when})

	default:
		// Unreachable: upcast already rejected unknown types. Kept as a
		// guard so a future catalog addition cannot silently no-op here.
		return &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
			Sentinel: ErrCannotReplay, Reason: "type not handled by apply"}
	}
	return nil
}

// leaseExpiredBy reports whether claimID's lease had lapsed at instant t.
// A missing lease counts as lapsed: the claiming daemon writes the lease
// with the claim, so its absence means the record is gone.
func leaseExpiredBy(leases map[string]time.Time, claimID string, t time.Time) bool {
	exp, ok := leases[claimID]
	return !ok || !exp.After(t)
}

func interruptedRun(c *Claim) Run {
	return Run{ID: c.ID, Task: c.Task, Claim: c.ID, Actor: c.Actor,
		Machine: c.Machine, Outcome: event.OutcomeInterrupted,
		Summary: "lease expired without a finish or release", Synthesized: true}
}

func unmarshal(e event.Event, dst any) error {
	if err := jsonUnmarshal(e.Data, dst); err != nil {
		return &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
			Sentinel: ErrMalformedEvent, Reason: err.Error()}
	}
	return nil
}
