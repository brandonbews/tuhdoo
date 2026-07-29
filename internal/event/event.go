package event

import (
	"encoding/json"
	"fmt"
)

// Event is the decoded form of one stored event: the T3 envelope fields,
// the raw payload, and any envelope-level fields this binary does not
// know about. Unknown fields are carried as raw bytes so that
// Decode → Encode round-trips them byte-identically (additive-first
// versioning: readers ignore unknown fields, rewriters preserve them).
type Event struct {
	ID      string                     // ULID; the sole ordering key
	Type    string                     // "noun.verb", e.g. "task.created"
	V       int                        // schema version of this type (not of the format)
	Actor   string                     // D7 principal, e.g. "brandon/impl-2"
	Machine string                     // stable per-machine id
	Task    string                     // subject task id; empty means no subject task
	Sig     json.RawMessage            // reserved (D7); preserved verbatim; null on new events
	Data    json.RawMessage            // type-specific payload
	Unknown map[string]json.RawMessage // envelope fields we don't recognize
}

// nullJSON is the stored form of the reserved sig field in v1.
var nullJSON = json.RawMessage("null")

// New builds an Event for a v1-catalog payload, marshaling the payload
// into Data and setting Sig to null as T3 requires for new events.
// task may be empty for events with no subject task.
func New(id, typ string, v int, actor, machine, task string, payload any) (Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("event: marshal %s payload: %w", typ, err)
	}
	return Event{
		ID:      id,
		Type:    typ,
		V:       v,
		Actor:   actor,
		Machine: machine,
		Task:    task,
		Sig:     nullJSON,
		Data:    data,
	}, nil
}

// Encode renders e as canonical bytes — the exact bytes stored on the
// data branch. The same logical event always encodes to identical bytes.
//
// Envelope-level canonical rules (payload and value rules live on
// Canonicalize): "task" is omitted when empty; "sig" is emitted exactly
// as held (New sets it to null; Decode preserves what was stored, so an
// absent sig stays absent); unknown envelope fields are re-emitted in
// sorted-key position like any other field.
func Encode(e Event) ([]byte, error) {
	if e.ID == "" || e.Type == "" {
		return nil, fmt.Errorf("event: encode: id and type are required")
	}
	if e.Data == nil {
		return nil, fmt.Errorf("event: encode %s: data is required", e.Type)
	}

	doc := make(map[string]json.RawMessage, len(e.Unknown)+8)
	for k, v := range e.Unknown {
		doc[k] = v
	}
	doc["id"] = mustMarshal(e.ID)
	doc["type"] = mustMarshal(e.Type)
	doc["v"] = mustMarshal(e.V)
	doc["actor"] = mustMarshal(e.Actor)
	doc["machine"] = mustMarshal(e.Machine)
	if e.Task != "" {
		doc["task"] = mustMarshal(e.Task)
	}
	if e.Sig != nil {
		doc["sig"] = e.Sig
	}
	doc["data"] = e.Data

	rough, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("event: encode %s: %w", e.Type, err)
	}
	out, err := Canonicalize(rough)
	if err != nil {
		return nil, fmt.Errorf("event: encode %s: %w", e.Type, err)
	}
	return out, nil
}

// Decode parses stored event bytes into an Event, keeping every field it
// does not recognize in Unknown so Encode can reproduce the input
// byte-identically (for input that was canonical, as all stored events are).
func Decode(b []byte) (Event, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return Event{}, fmt.Errorf("event: decode: %w", err)
	}

	var e Event
	if err := takeString(doc, "id", true, &e.ID); err != nil {
		return Event{}, err
	}
	if err := takeString(doc, "type", true, &e.Type); err != nil {
		return Event{}, err
	}
	if raw, ok := doc["v"]; ok {
		if err := json.Unmarshal(raw, &e.V); err != nil {
			return Event{}, fmt.Errorf("event: decode: field %q: %w", "v", err)
		}
		delete(doc, "v")
	} else {
		return Event{}, fmt.Errorf("event: decode: missing field %q", "v")
	}
	if err := takeString(doc, "actor", false, &e.Actor); err != nil {
		return Event{}, err
	}
	if err := takeString(doc, "machine", false, &e.Machine); err != nil {
		return Event{}, err
	}
	if err := takeString(doc, "task", false, &e.Task); err != nil {
		return Event{}, err
	}
	if raw, ok := doc["sig"]; ok {
		e.Sig = raw
		delete(doc, "sig")
	}
	raw, ok := doc["data"]
	if !ok {
		return Event{}, fmt.Errorf("event: decode: missing field %q", "data")
	}
	e.Data = raw
	delete(doc, "data")

	if len(doc) > 0 {
		e.Unknown = doc
	}
	return e, nil
}

// takeString moves a string field out of doc into dst. Missing optional
// fields leave dst empty.
func takeString(doc map[string]json.RawMessage, key string, required bool, dst *string) error {
	raw, ok := doc[key]
	if !ok {
		if required {
			return fmt.Errorf("event: decode: missing field %q", key)
		}
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("event: decode: field %q: %w", key, err)
	}
	delete(doc, key)
	return nil
}

// mustMarshal marshals values that cannot fail (strings and ints).
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // unreachable for string and int inputs
	}
	return b
}
