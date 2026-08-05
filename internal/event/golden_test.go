package event

import (
	"bytes"
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

// goldenEvents is one fixed event per catalog type. Changing any fixture
// (or the encoder) must be a deliberate act that also updates testdata/.
func goldenEvents(t *testing.T) map[string]Event {
	t.Helper()

	str := func(s string) *string { return &s }
	num := func(n int) *int { return &n }

	fixtures := map[string]struct {
		fill    byte
		task    string
		payload any
	}{
		// task.created and task.updated are v2 (2026-07-31): status is a
		// payload field, and the new inbox/held values pin their bytes.
		TypeTaskCreated: {0x01, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", TaskCreated{
			Title:       "Write the event model",
			Description: "Envelope, canonical JSON, catalog. See docs/design/002-technology.md T3.",
			Status:      "inbox",
			Priority:    2,
			Labels:      []string{"core", "v0"},
			Parents:     []string{"t-01BX5ZZKBKACTAV9WEVGEMMVRX"},
			DependsOn:   []string{"t-01BX5ZZKBKACTAV9WEVGEMMVRW"},
		}},
		TypeTaskUpdated: {0x02, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", TaskUpdated{
			Status:   str("held"),
			Priority: num(1),
		}},
		TypeClaimMade: {0x03, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", ClaimMade{}},
		TypeClaimConfirmed: {0x09, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", ClaimConfirmed{
			Claim: "01BX5ZZKBK1HHHHHHHHHHHHHHH",
		}},
		TypeClaimReleased: {0x04, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", ClaimReleased{
			Reason: "context exhausted; see latest note for where work stopped",
		}},
		TypeRunFinished: {0x05, "t-01BX5ZZKBKACTAV9WEVGEMMVRY", RunFinished{
			Outcome: OutcomeDone,
			Branch:  "feat/event-model",
			PR:      "https://example.com/pr/42",
			Commits: []string{"a1b2c3d", "e4f5a6b"},
			Summary: "Implemented envelope + canonical encoder; all tests green.",
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
