package event

import (
	"encoding/json"
	"fmt"
)

// The event-type catalog (T3, T5). Additive payload changes never bump
// a version, so these only move on a breaking change (rare by design).
// "Additive" means an old reader ignoring the field still computes
// correct state — a field whose absence implied a default that its
// presence can contradict (task.created's status, 2026-07-31) is
// breaking in additive clothing, and bumps.
const (
	TypeTaskCreated        = "task.created"
	TypeTaskUpdated        = "task.updated"
	TypeClaimMade          = "claim.made"
	TypeClaimConfirmed     = "claim.confirmed"
	TypeClaimReleased      = "claim.released"
	TypeRunFinished        = "run.finished"
	TypeEscalationRaised   = "escalation.raised"
	TypeEscalationAnswered = "escalation.answered"
	TypeNoteAdded          = "note.added"
)

// Versions maps each catalog type to the schema version this binary
// writes and fully understands.
//
// task.created/task.updated moved to v2 on 2026-07-31 (inbox/held task
// statuses): v2 task.created carries a status field, and v2 task.updated
// may carry the new status values. v1-only binaries would mint a
// status-carrying task.created as open-and-claimable (unknown fields are
// ignored — verified mis-bucketing) and reject an inbox/held
// task.updated as malformed rather than as needing an upgrade; at v2
// they instead fail safe with "upgrade tuhdoo" (T3 read-only mode). The
// v1→v2 upcasters are the identity (internal/core/upcast.go).
// claim.confirmed is new on 2026-08-04 (D6 confirmation gate): a new
// type, not a version bump — additive-first (T3). Older binaries meeting
// one enter read-only fail-safe with the "upgrade tuhdoo" message.
var Versions = map[string]int{
	TypeTaskCreated:        2,
	TypeTaskUpdated:        2,
	TypeClaimMade:          1,
	TypeClaimConfirmed:     1,
	TypeClaimReleased:      1,
	TypeRunFinished:        1,
	TypeEscalationRaised:   1,
	TypeEscalationAnswered: 1,
	TypeNoteAdded:          1,
}

// Run outcomes for RunFinished: the agent-reported set (T5 finish_run)
// plus two daemon-only outcomes — "interrupted" for runs orphaned by
// lease expiry and "superseded" for work voided by losing a cross-machine
// claim race (D6).
const (
	OutcomeDone        = "done"
	OutcomeFailed      = "failed"
	OutcomeAbandoned   = "abandoned"
	OutcomeBlocked     = "blocked"
	OutcomeInterrupted = "interrupted"
	OutcomeSuperseded  = "superseded"
)

// Payload structs. Fields deliberately carry no omitempty: every known
// field is always present in the encoded payload, which keeps the
// canonical bytes of a payload a function of its values alone. A nil
// slice or pointer encodes as null, meaning "none"/"unchanged".
//
// Each struct has an Unknown map holding payload fields this binary does
// not recognize, so decoded payloads re-encode byte-identically.

// TaskCreated is the payload of "task.created". Parents and DependsOn
// are DAG edges: task IDs of parent tasks and prerequisite tasks.
// Status (v2, 2026-07-31) is the task's initial status; empty means
// "open", the only reading v1 events could carry.
type TaskCreated struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	Parents     []string `json:"parents"`
	DependsOn   []string `json:"depends_on"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// TaskUpdated is the payload of "task.updated". Only changed fields are
// non-null; null means "unchanged".
type TaskUpdated struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Status      *string   `json:"status"`
	Priority    *int      `json:"priority"`
	Labels      *[]string `json:"labels"`
	Parents     *[]string `json:"parents"`
	DependsOn   *[]string `json:"depends_on"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// ClaimMade is the payload of "claim.made". The claimed task and the
// claiming principal live on the envelope; v1 has no extra payload.
type ClaimMade struct {
	Unknown map[string]json.RawMessage `json:"-"`
}

// ClaimConfirmed is the payload of "claim.confirmed" (D6, 2026-08-04):
// the referee's irrevocable verdict, won through the remote's ref
// compare-and-swap. Claim names the claim.made event it confirms; the
// task, confirming actor, and machine live on the envelope as usual.
type ClaimConfirmed struct {
	Claim string `json:"claim"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// ClaimReleased is the payload of "claim.released": a voluntary
// stand-down with the reason recorded.
type ClaimReleased struct {
	Reason string `json:"reason"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// RunFinished is the payload of "run.finished". Outcome is one of the
// Outcome* constants. Branch, PR, and Commits are stored strings, never
// dereferenced (T2: host-agnostic).
type RunFinished struct {
	Outcome string   `json:"outcome"`
	Branch  string   `json:"branch"`
	PR      string   `json:"pr"`
	Commits []string `json:"commits"`
	Summary string   `json:"summary"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// EscalationRaised is the payload of "escalation.raised".
type EscalationRaised struct {
	Question string `json:"question"`
	Context  string `json:"context"`
	Blocking bool   `json:"blocking"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// EscalationAnswered is the payload of "escalation.answered".
// Escalation is the event ID of the escalation.raised being answered.
// AnsweredBy is the principal the answer is attributed to — the
// envelope actor's root when relayed through relay_answer (T5,
// 2026-07-30 revision). Additive: events written before the field
// existed carry none, and replay falls back to the envelope actor.
type EscalationAnswered struct {
	Answer     string `json:"answer"`
	AnsweredBy string `json:"answered_by"`
	Escalation string `json:"escalation"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// NoteAdded is the payload of "note.added": optional mid-flight
// checkpoints — continuity is carried by the typed transition events
// (T5, notes doctrine revised 2026-07-30).
type NoteAdded struct {
	Text string `json:"text"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// The MarshalJSON/UnmarshalJSON pairs below all follow one boring
// pattern: a local alias type drops the custom methods so encoding/json
// handles the tagged fields, and marshalWithUnknown/splitUnknown carry
// the unrecognized fields across.

func (p TaskCreated) MarshalJSON() ([]byte, error) {
	type alias TaskCreated
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *TaskCreated) UnmarshalJSON(b []byte) error {
	type alias TaskCreated
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = TaskCreated(a)
	p.Unknown = unknown
	return nil
}

func (p TaskUpdated) MarshalJSON() ([]byte, error) {
	type alias TaskUpdated
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *TaskUpdated) UnmarshalJSON(b []byte) error {
	type alias TaskUpdated
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = TaskUpdated(a)
	p.Unknown = unknown
	return nil
}

func (p ClaimMade) MarshalJSON() ([]byte, error) {
	type alias ClaimMade
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *ClaimMade) UnmarshalJSON(b []byte) error {
	type alias ClaimMade
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = ClaimMade(a)
	p.Unknown = unknown
	return nil
}

func (p ClaimConfirmed) MarshalJSON() ([]byte, error) {
	type alias ClaimConfirmed
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *ClaimConfirmed) UnmarshalJSON(b []byte) error {
	type alias ClaimConfirmed
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = ClaimConfirmed(a)
	p.Unknown = unknown
	return nil
}

func (p ClaimReleased) MarshalJSON() ([]byte, error) {
	type alias ClaimReleased
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *ClaimReleased) UnmarshalJSON(b []byte) error {
	type alias ClaimReleased
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = ClaimReleased(a)
	p.Unknown = unknown
	return nil
}

func (p RunFinished) MarshalJSON() ([]byte, error) {
	type alias RunFinished
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *RunFinished) UnmarshalJSON(b []byte) error {
	type alias RunFinished
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = RunFinished(a)
	p.Unknown = unknown
	return nil
}

func (p EscalationRaised) MarshalJSON() ([]byte, error) {
	type alias EscalationRaised
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *EscalationRaised) UnmarshalJSON(b []byte) error {
	type alias EscalationRaised
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = EscalationRaised(a)
	p.Unknown = unknown
	return nil
}

func (p EscalationAnswered) MarshalJSON() ([]byte, error) {
	type alias EscalationAnswered
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *EscalationAnswered) UnmarshalJSON(b []byte) error {
	type alias EscalationAnswered
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = EscalationAnswered(a)
	p.Unknown = unknown
	return nil
}

func (p NoteAdded) MarshalJSON() ([]byte, error) {
	type alias NoteAdded
	return marshalWithUnknown(alias(p), p.Unknown)
}

func (p *NoteAdded) UnmarshalJSON(b []byte) error {
	type alias NoteAdded
	var a alias
	unknown, err := splitUnknown(b, &a)
	if err != nil {
		return err
	}
	*p = NoteAdded(a)
	p.Unknown = unknown
	return nil
}

// marshalWithUnknown marshals a payload's known fields (via its alias
// type) and merges the preserved unknown fields back in. The result is
// not yet canonical; Encode canonicalizes the whole event at the end.
func marshalWithUnknown(known any, unknown map[string]json.RawMessage) ([]byte, error) {
	b, err := json.Marshal(known)
	if err != nil {
		return nil, err
	}
	if len(unknown) == 0 {
		return b, nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	for k, v := range unknown {
		if _, exists := doc[k]; exists {
			return nil, fmt.Errorf("event: unknown payload field %q collides with a known field", k)
		}
		doc[k] = v
	}
	return json.Marshal(doc)
}

// splitUnknown unmarshals payload bytes into the known fields of dst (a
// pointer to an alias struct) and returns the fields dst has no place
// for. Known keys are discovered by re-marshaling dst: payload structs
// carry no omitempty, so every known key appears.
func splitUnknown(b []byte, dst any) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(b, dst); err != nil {
		return nil, err
	}
	knownJSON, err := json.Marshal(dst)
	if err != nil {
		return nil, err
	}
	var known map[string]json.RawMessage
	if err := json.Unmarshal(knownJSON, &known); err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	for k := range known {
		delete(doc, k)
	}
	if len(doc) == 0 {
		return nil, nil
	}
	return doc, nil
}
