package event

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// -update regenerates the golden files from the fixtures below.
// Run: go test ./internal/event -run TestGolden -update
var update = flag.Bool("update", false, "rewrite golden files")

// fixedID returns a deterministic ULID for test fixtures: fixed time,
// fixed entropy bytes.
func fixedID(t *testing.T, when time.Time, fill byte) string {
	t.Helper()
	entropy := bytes.NewReader(bytes.Repeat([]byte{fill}, 10))
	id, err := NewID(when, entropy)
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	return id
}

var goldenTime = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

// Pointer literals for task.updated's optional fields.
func str(s string) *string { return &s }
func num(n int) *int       { return &n }

// goldenEvents is one fixed event per catalog type. Changing any fixture
// (or the encoder) must be a deliberate act that also updates testdata/.
func goldenEvents(t *testing.T) map[string]Event {
	t.Helper()

	fixtures := map[string]struct {
		fill    byte
		task    string
		payload any
	}{
		// task.created and task.updated are v3 (2026-08-21, P0-highest
		// flip): priority is nullable — these fixtures pin explicit
		// numbers; the v2 fixtures' bytes live on through the upcaster
		// tests in internal/core.
		TypeTaskCreated: {0x01, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", TaskCreated{
			Title:       "Write the event model",
			Description: "Envelope, canonical JSON, catalog. See docs/design/002-technology.md T3.",
			Status:      "inbox",
			Priority:    num(2),
			Labels:      []string{"core", "v0"},
			DependsOn:   []string{"t-01BX5ZZKBKACTAV9WEVGEMMVRW"},
		}},
		TypeTaskUpdated: {0x02, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", TaskUpdated{
			Status:   str("held"),
			Priority: SetPriority(1),
		}},
		TypeClaimMade: {0x03, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", ClaimMade{}},
		TypeClaimConfirmed: {0x09, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", ClaimConfirmed{
			Claim: "01BX5ZZKBK1HHHHHHHHHHHHHHH",
		}},
		TypeClaimReleased: {0x04, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", ClaimReleased{
			Reason: "context exhausted; see latest note for where work stopped",
		}},
		TypeRunFinished: {0x05, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", RunFinished{
			Outcome:  OutcomeDone,
			Claim:    "01BX5ZZKBK2HHHHHHHHHHHHHHH",
			Branch:   "feat/event-model",
			PR:       "https://example.com/pr/42",
			Commits:  []string{"a1b2c3d", "e4f5a6b"},
			MergedAs: []string{"9c8d7e6"},
			Summary:  "Implemented envelope + canonical encoder; all tests green.",
		}},
		TypeEscalationRaised: {0x06, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", EscalationRaised{
			Question: "Should task.updated support edge removal in v1?",
			Context:  "The catalog only records additions today.",
			Blocking: true,
		}},
		TypeEscalationAnswered: {0x07, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", EscalationAnswered{
			Answer:     "No — additions only in v1; removal is a Cycle 3 question.",
			AnsweredBy: "brandon", // relayed: the envelope actor below is the scribe
			Escalation: "01BX5ZZKBKACTAV9WEVGEMMVS0",
		}},
		TypeNoteAdded: {0x08, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", NoteAdded{
			Text: "Checkpoint: encoder done, starting on decode. Unicode survives: héllo — ✓",
		}},
	}

	events := make(map[string]Event, len(fixtures))
	for typ, f := range fixtures {
		e, err := New(fixedID(t, goldenTime, f.fill), typ, Versions[typ],
			"brandon/impl-2", "m-3f9a", f.task, f.payload)
		if err != nil {
			t.Fatalf("New(%s): %v", typ, err)
		}
		events[typ] = e
	}
	return events
}

func goldenPath(typ string) string {
	return filepath.Join("testdata", typ+".golden.json")
}

func TestGoldenEncode(t *testing.T) {
	for typ, e := range goldenEvents(t) {
		got, err := Encode(e)
		if err != nil {
			t.Fatalf("Encode(%s): %v", typ, err)
		}
		if *update {
			if err := os.MkdirAll("testdata", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(goldenPath(typ), got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(goldenPath(typ))
		if err != nil {
			t.Fatalf("read golden for %s (run with -update to create): %v", typ, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: encoded bytes differ from golden\n got: %s\nwant: %s", typ, got, want)
		}
	}
}

// TestGoldenRoundTrip proves stored bytes survive decode → encode
// untouched for every catalog type.
func TestGoldenRoundTrip(t *testing.T) {
	if *update {
		t.Skip("updating golden files")
	}
	for typ := range goldenEvents(t) {
		stored, err := os.ReadFile(goldenPath(typ))
		if err != nil {
			t.Fatal(err)
		}
		e, err := Decode(stored)
		if err != nil {
			t.Fatalf("Decode(%s): %v", typ, err)
		}
		again, err := Encode(e)
		if err != nil {
			t.Fatalf("re-Encode(%s): %v", typ, err)
		}
		if !bytes.Equal(stored, again) {
			t.Errorf("%s: decode→encode changed bytes\n got: %s\nwant: %s", typ, again, stored)
		}
	}
}

// task.updated's priority is tri-state since v4 (2026-09-11): absent =
// unchanged, null = clear, number = set. The fixtures below pin all
// three states on the wire — absent must stay absent, which a plain
// *int cannot do — and the v3 fixtures (null meant unchanged; a number
// set) prove old bytes survive the struct untouched. Unlike
// TestGoldenRoundTrip, which never looks inside Data, these round-trip
// the payload THROUGH the typed struct: decode → TaskUpdated →
// re-marshal → canonical encode must reproduce the stored bytes.
func TestTaskUpdatedPriorityGolden(t *testing.T) {
	const task = "t-01BX5ZZKBKACTAV9WEVGEMMVRY"
	fixtures := []struct {
		name    string
		payload *TaskUpdated // nil: a stored-bytes-only fixture, never regenerated
	}{
		{"v4-set", &TaskUpdated{Status: str("held"), Priority: SetPriority(1)}},
		{"v4-clear", &TaskUpdated{Status: str("held"), Priority: ClearPriority()}},
		{"v4-unchanged", &TaskUpdated{Status: str("held")}},
		{"v3-set", nil},
		{"v3-null", nil},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			path := goldenPath(TypeTaskUpdated + "." + f.name)
			if f.payload != nil {
				e, err := New(fixedID(t, goldenTime, 0x02), TypeTaskUpdated, Versions[TypeTaskUpdated],
					"brandon/impl-2", "m-3f9a", task, *f.payload)
				if err != nil {
					t.Fatal(err)
				}
				got, err := Encode(e)
				if err != nil {
					t.Fatal(err)
				}
				if *update {
					if err := os.WriteFile(path, got, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read golden (run with -update to create): %v", err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("encoded bytes differ from golden\n got: %s\nwant: %s", got, want)
				}
			}
			stored, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			e, err := Decode(stored)
			if err != nil {
				t.Fatal(err)
			}
			var p TaskUpdated
			if err := json.Unmarshal(e.Data, &p); err != nil {
				t.Fatalf("payload: %v", err)
			}
			again, err := New(e.ID, e.Type, e.V, e.Actor, e.Machine, e.Task, p)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Encode(again)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(stored, got) {
				t.Errorf("decode → struct → encode changed bytes\n got: %s\nwant: %s", got, stored)
			}
		})
	}
}

// The three wire states decode to three distinct Go values, and the
// v3 null (unchanged) is NOT the same value as v4 absent: the
// distinction is the upcaster's job (internal/core), not the codec's.
func TestPriorityChangeDecode(t *testing.T) {
	tests := []struct {
		in   string
		want PriorityChange
	}{
		{`{}`, PriorityChange{}},
		{`{"priority":null}`, ClearPriority()},
		{`{"priority":2}`, SetPriority(2)},
	}
	for _, tt := range tests {
		var p TaskUpdated
		if err := json.Unmarshal([]byte(tt.in), &p); err != nil {
			t.Fatalf("%s: %v", tt.in, err)
		}
		if p.Priority.Present != tt.want.Present ||
			(p.Priority.Value == nil) != (tt.want.Value == nil) ||
			(p.Priority.Value != nil && *p.Priority.Value != *tt.want.Value) {
			t.Errorf("%s: priority = %+v, want %+v", tt.in, p.Priority, tt.want)
		}
		if p.Unknown != nil {
			t.Errorf("%s: unknown = %v, want none", tt.in, p.Unknown)
		}
	}
}
