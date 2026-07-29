package core

import (
	"encoding/json"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// jsonUnmarshal is the one JSON entry point for payload decoding, named
// so replay.go reads without an import alias.
func jsonUnmarshal(data []byte, dst any) error { return json.Unmarshal(data, dst) }

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
