package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
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
	r := &Replayer{upcasters: make(map[upKey]Upcaster)}
	registerCatalogUpcasters(r)
	return r
}

// terminalStatus reports whether s is a closed status — the two the
// ClosedAt/ClosedBy stamps track.
func terminalStatus(s string) bool {
	return s == StatusDone || s == StatusCancelled
}

// knownStatus reports whether s is a status this binary understands.
// An unknown value is a malformed event, never best-effort-bucketed:
// mis-filing a status a newer binary wrote is exactly the divergence T3
// forbids (the writer bumps the schema version instead — see upcast.go).
func knownStatus(s string) bool {
	switch s {
	case StatusOpen, StatusInbox, StatusHeld, StatusDone, StatusCancelled:
		return true
	}
	return false
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

	// Loser expiry check (D6 clause 3, 2026-08-04): a voided claim is an
	// attempt the race ended, and its loser owes a closing run — the
	// daemon coerces any finish_run on it to "superseded". A loser that
	// never reports leaves a trace anyway: once the voided claim's lease
	// lapses with no run closing the attempt (RunCloses: claim-linked
	// runs match by claim, legacy runs by the actor+order heuristic),
	// replay synthesizes a branch-less superseded run, exactly as interruptedRun
	// does for expired holders. Deterministic at every instant: leases
	// and Now are replay inputs, and only real runs count as closes
	// (s.Runs holds no synthesized runs yet). The write-side finish guard
	// mirrors this rule, refusing a late close once the run exists here —
	// one close per attempt.
	for _, c := range s.Claims {
		if c.Status != ClaimVoided || !leaseExpiredBy(in.Leases, c.ID, in.Now) {
			continue
		}
		if closedByRun(s.Runs, c) {
			continue
		}
		synthesized = append(synthesized, supersededRun(c))
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
		// Empty status is the v1 reading: every task minted before the
		// field existed (v2, 2026-07-31) was born open. Anything else
		// must be a known status — vocabulary is validated, transitions
		// never are (T3: no rejected-event edge cases).
		status := p.Status
		if status == "" {
			status = StatusOpen
		}
		if !knownStatus(status) {
			return malformed("unknown task status %q", p.Status)
		}
		t := &Task{
			ID: e.Task, Title: p.Title, Description: p.Description,
			Priority: p.Priority, Labels: p.Labels, DependsOn: p.DependsOn,
			Status: status, CreatedBy: e.Actor, CreatedAt: when,
		}
		// Born terminal (B12 migration shape): the creation event is the
		// closing event.
		if terminalStatus(status) {
			t.ClosedAt, t.ClosedBy = when, e.Actor
		}
		s.Tasks[e.Task] = t
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
		// ordering applied to curation). Each written field also records
		// its line in the task's edit history (2026-08-11: every edit
		// gets an entry) — old values read here, before the write.
		u := Update{ID: e.ID, Task: e.Task, Actor: e.Actor}
		if p.Title != nil {
			u.Fields = append(u.Fields, "retitled")
			t.Title = *p.Title
		}
		if p.Description != nil {
			u.Fields = append(u.Fields, "description edited")
			t.Description = *p.Description
		}
		if p.Status != nil {
			// Any known status may follow any other: transitions are
			// mechanically permissive (2026-07-31) — promote/pause
			// semantics are protocol, not replay rules.
			if !knownStatus(*p.Status) {
				return malformed("unknown task status %q", *p.Status)
			}
			u.Fields = append(u.Fields, "status "+t.Status+"→"+*p.Status)
			// Close metadata follows the status: the event *entering* a
			// terminal status is the closing event (re-asserting the same
			// terminal status keeps the original stamp); leaving terminal
			// clears it.
			switch prev := t.Status; {
			case !terminalStatus(*p.Status):
				t.ClosedAt, t.ClosedBy = time.Time{}, ""
			case *p.Status != prev:
				t.ClosedAt, t.ClosedBy = when, e.Actor
			}
			t.Status = *p.Status
		}
		if p.Priority != nil {
			u.Fields = append(u.Fields, "priority "+priorityLabel(t.Priority)+"→"+priorityLabel(p.Priority))
			t.Priority = p.Priority
		}
		if p.Labels != nil {
			u.Fields = append(u.Fields, listDelta("labels", t.Labels, *p.Labels, func(v string) string { return v }))
			t.Labels = *p.Labels
		}
		if p.DependsOn != nil {
			u.Fields = append(u.Fields, listDelta("depends_on", t.DependsOn, *p.DependsOn, event.ShortID))
			t.DependsOn = *p.DependsOn
		}
		s.Updates = append(s.Updates, u)

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

	case event.TypeClaimConfirmed:
		var p event.ClaimConfirmed
		if err := unmarshal(e, &p); err != nil {
			return err
		}
		c, ok := s.Claims[p.Claim]
		if !ok {
			return malformed("confirmation of unknown claim %s", p.Claim)
		}
		if c.Task != e.Task {
			return malformed("confirmation on task %s names claim %s, which is on task %s",
				e.Task, p.Claim, c.Task)
		}
		// D6 (2026-08-04): a confirmed claim wins its contest
		// unconditionally. A contest is one continuous hold on the task —
		// it opens when a claim takes the hold and closes when the hold
		// ends (finish, release, lease expiry) — and a confirmation binds
		// to one claim inside it, settling the race, not liveness: a
		// confirmed claim can still end, after which the task returns to
		// the pool and a NEW contest may legitimately mint a new
		// confirmation. One confirmation per contest, not one per task
		// forever.
		switch h := holder[e.Task]; {
		case h == c:
			// The provisional winner was confirmed — the usual case. A
			// duplicate confirmation keeps the earliest event ID.
			if c.Confirmation == "" {
				c.Confirmation = e.ID
			}
		case h != nil && h.Status == ClaimActive && h.Confirmation == "" && c.Status == ClaimVoided:
			// The referee confirmed a claim the mint-time rule had
			// voided: the confirmation beats the earlier-ULID
			// unconfirmed holder, which stands down as the loser.
			h.Status = ClaimVoided
			c.Status = ClaimActive
			c.Confirmation = e.ID
			holder[e.Task] = c
		default:
			// Fail-safe determinism, never fail-stop: a second
			// confirmation inside one contest (a corrupt ledger — the
			// writers' invariant refuses to carry one) loses to the
			// earlier confirmation, whose event sorted first and already
			// holds; a confirmation for a claim whose hold has ended
			// settles nothing — that contest is over. Both no-op
			// identically on every replay.
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
		// The event's own claim field (additive, 2026-08-27) is the real
		// linkage; a legacy event without one falls back to inferring the
		// holder's claim below. Never validated against s.Claims: a
		// dangling reference deterministically links nothing, and the
		// hold/status semantics never depend on it (T3: replay never
		// re-judges stored runs).
		run := Run{ID: e.ID, Task: e.Task, Claim: p.Claim, Actor: e.Actor,
			Machine: e.Machine, Outcome: p.Outcome, Branch: p.Branch, PR: p.PR,
			Commits: p.Commits, MergedAs: p.MergedAs, Summary: p.Summary,
			claimFromEvent: p.Claim != ""}
		if h := holder[e.Task]; h != nil && h.Status == ClaimActive && h.Actor == e.Actor {
			if run.Claim == "" {
				run.Claim = h.ID
			}
			h.Status = ClaimFinished
			holder[e.Task] = nil
			if p.Outcome == event.OutcomeDone {
				if t.Status != StatusDone {
					t.ClosedAt, t.ClosedBy = when, e.Actor
				}
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
		if esc.Task != e.Task {
			return malformed("answer on task %s names escalation %s, which is on task %s",
				e.Task, p.Escalation, esc.Task)
		}
		// Last answer wins: answers are human amendments, unlike claims.
		// Attribution goes to the payload's answered_by when present (a
		// relayed answer's envelope actor is the scribe, not the
		// answerer); events from before the field existed fall back to
		// the envelope actor.
		esc.Answer = p.Answer
		esc.AnsweredBy = e.Actor
		esc.RelayedBy = ""
		if p.AnsweredBy != "" {
			esc.AnsweredBy = p.AnsweredBy
			if p.AnsweredBy != e.Actor {
				esc.RelayedBy = e.Actor
			}
		}
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

// priorityLabel renders a nullable priority for edit-history lines:
// the number, or "none" for unprioritized (P0-highest flip,
// 2026-08-21).
func priorityLabel(p *int) string {
	if p == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *p)
}

// listDelta summarizes a list-field replacement as its membership
// delta: "+x" per addition (new-list order), "−y" per removal (old-list
// order), after the field name. A replacement that changes membership
// not at all (a reorder, or a re-send of the same list) reads
// name-only, like the text fields. render maps items for display —
// identity for labels, the short-ID form for depends_on edges (T7: the
// short form is the human contract; the grill's decided shape).
func listDelta(field string, from, to []string, render func(string) string) string {
	var parts []string
	for _, v := range to {
		if !slices.Contains(from, v) {
			parts = append(parts, "+"+render(v))
		}
	}
	for _, v := range from {
		if !slices.Contains(to, v) {
			parts = append(parts, "−"+render(v))
		}
	}
	if len(parts) == 0 {
		return field + " edited"
	}
	return field + " " + strings.Join(parts, " ")
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

// supersededRun is the trace a race loser that never reported leaves
// behind (D6, 2026-08-04). Branch-less by construction: the branch name
// is knowable only to the losing agent, so a synthesized close cannot
// carry one — salvage with a branch pointer exists only when the loser
// reported through finish_run.
func supersededRun(c *Claim) Run {
	return Run{ID: c.ID, Task: c.Task, Claim: c.ID, Actor: c.Actor,
		Machine: c.Machine, Outcome: event.OutcomeSuperseded,
		Summary: "claim lost its race; lease expired without a report", Synthesized: true}
}

// RunCloses reports whether run r closes the attempt under claim c —
// the one close-matching rule, shared with the daemon's write-side
// guard (attemptCloseLocked) so guard and synthesis can never disagree
// about whether a close is still owed. A run whose event named its
// claim (the additive run.finished claim field, 2026-08-27) closes
// exactly that claim — a later attempt by the same actor no longer
// double-books as an earlier lost attempt's close, erasing its
// superseded trace (D6 clause 3: one close per attempt) — and a
// synthesized run likewise binds to its claim. A legacy run, minted
// before the field existed, keeps the original heuristic — same task,
// same actor, minted after the claim — so stored history replays to
// the state it always had (T3).
func RunCloses(r *Run, c *Claim) bool {
	if r.Synthesized || r.claimFromEvent {
		return r.Claim == c.ID
	}
	return r.Task == c.Task && r.Actor == c.Actor && r.ID > c.ID
}

// closedByRun reports whether any run in runs closes the attempt under
// claim c. At its one call site runs holds only real runs — synthesis
// is deciding whether it still owes a close.
func closedByRun(runs []Run, c *Claim) bool {
	for i := range runs {
		if RunCloses(&runs[i], c) {
			return true
		}
	}
	return false
}

func unmarshal(e event.Event, dst any) error {
	if err := json.Unmarshal(e.Data, dst); err != nil {
		return &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
			Sentinel: ErrMalformedEvent, Reason: err.Error()}
	}
	return nil
}
