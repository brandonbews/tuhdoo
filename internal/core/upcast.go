package core

import (
	"encoding/json"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// Upcaster lifts a payload one schema version: v → v+1, in memory only.
// Stored bytes are never rewritten (T3).
type Upcaster func(data json.RawMessage) (json.RawMessage, error)

type upKey struct {
	typ  string
	from int
}

// RegisterUpcaster installs the lift for typ payloads at version from.
// Ladders compose: v0→v1 then v1→v2 both fire for a v0 event when the
// current version is 2.
func (r *Replayer) RegisterUpcaster(typ string, from int, fn Upcaster) {
	r.upcasters[upKey{typ, from}] = fn
}

// registerCatalogUpcasters installs the standard ladder every replayer
// carries — NewReplayer calls it, so no caller can forget it and
// compute a different truth from the same log.
//
// task.created/task.updated v1→v2 (2026-07-31, inbox/held statuses):
// the payload shapes are forward-compatible — a v1 task.created has no
// status field (read as "open"; replay defaults the empty value), and a
// v1 task.updated carries only status values v2 also knows — so both
// lifts are the identity. The bump exists for the OLD side, not this
// one: a v1-only binary decoding a task.created that carries
// status:"inbox" would silently drop the unknown field and mint the
// task open and claimable (verified 2026-07-31), which is mis-bucketing,
// not additive reading. Writing such payloads at v2 turns that into the
// honest T3 outcome — "schema version above this binary's — upgrade
// tuhdoo", fail-safe read-only mode.
// task.created/task.updated v2→v3 (2026-08-21, P0-highest priority
// flip): v3 makes priority nullable (null = unprioritized, sorts last)
// and flips the number line (lower = more urgent, 0 the most). The lift
// translates v2 semantics faithfully: an explicit 0 — v2's "default,
// unprioritized" — becomes null, which for task.updated reads as
// "unchanged" (v3 deliberately cannot express "set to none"; the
// closest honest translation of a v2 reset-to-default is not-an-edit).
// Nonzero values carry through numerically: their v2 intent (higher =
// more urgent) cannot be renumbered mechanically on an unbounded
// scale, so live nonzero priorities are hand-corrected by ordinary
// task.updated events at migration instead (the 2026-08-21 flip plan).
func registerCatalogUpcasters(r *Replayer) {
	identity := func(data json.RawMessage) (json.RawMessage, error) { return data, nil }
	r.RegisterUpcaster(event.TypeTaskCreated, 1, identity)
	r.RegisterUpcaster(event.TypeTaskUpdated, 1, identity)
	r.RegisterUpcaster(event.TypeTaskCreated, 2, zeroPriorityToNull)
	r.RegisterUpcaster(event.TypeTaskUpdated, 2, zeroPriorityToNull)
}

// zeroPriorityToNull rewrites a payload's "priority": 0 to null, in
// memory only, leaving every other field byte-for-byte alone.
func zeroPriorityToNull(data json.RawMessage) (json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	raw, ok := doc["priority"]
	if !ok {
		return data, nil
	}
	var p *int
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if p == nil || *p != 0 {
		return data, nil
	}
	doc["priority"] = json.RawMessage("null")
	return json.Marshal(doc)
}

// upcast brings one event to the current schema version of its type, or
// fails with ErrCannotReplay. This is the fail-safe gate (T3): events
// from a newer daemon (unknown type, or version above ours) and old
// events with a broken upcaster ladder both stop replay honestly rather
// than degrade it silently.
func (r *Replayer) upcast(e event.Event) (event.Event, error) {
	current, known := event.Versions[e.Type]
	if !known {
		return event.Event{}, &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
			Sentinel: ErrCannotReplay, Reason: "unknown event type (newer daemon?) — upgrade tuhdoo"}
	}
	if e.V > current {
		return event.Event{}, &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
			Sentinel: ErrCannotReplay, Reason: "schema version above this binary's — upgrade tuhdoo"}
	}
	for e.V < current {
		fn, ok := r.upcasters[upKey{e.Type, e.V}]
		if !ok {
			return event.Event{}, &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
				Sentinel: ErrCannotReplay, Reason: "no upcaster from this version"}
		}
		lifted, err := fn(e.Data)
		if err != nil {
			return event.Event{}, &ReplayError{EventID: e.ID, Type: e.Type, V: e.V,
				Sentinel: ErrCannotReplay, Reason: "upcaster failed: " + err.Error()}
		}
		e.Data = lifted
		e.V++
	}
	return e, nil
}
