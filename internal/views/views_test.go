package views_test

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/views"
)

var update = flag.Bool("update", false, "rewrite golden files")

var (
	base    = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	testNow = base.Add(2 * time.Hour)
)

// tick and evt mirror the internal/core test helpers: deterministic
// ULIDs minted n minutes after base, so larger n sorts later.
func tick(t *testing.T, n int) string {
	t.Helper()
	entropy := make([]byte, 10)
	entropy[9] = byte(n)
	id, err := event.NewID(base.Add(time.Duration(n)*time.Minute), bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func evt(t *testing.T, n int, typ, actor, task string, payload any) event.Event {
	t.Helper()
	e, err := event.New(tick(t, n), typ, 1, actor, "m-test", task, payload)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// goldenInput builds a representative event history covering every
// rendering path: tasks in all statuses, an active claim, an expired
// claim with a synthesized interrupted run, a blocking open escalation,
// an answered escalation, notes, and DAG edges.
func goldenInput(t *testing.T) core.Input {
	t.Helper()
	events := []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "t-epic", event.TaskCreated{
			Title:       "v0 build-out",
			Description: "Umbrella task for the v0 milestone.",
			Priority:    1,
		}),
		evt(t, 2, event.TypeTaskCreated, "brandon", "t-core", event.TaskCreated{
			Title:       "Build the replay engine",
			Description: "Pure function: event set -> state.\n\nAcceptance:\n- order-insensitive replay\n- lease expiry returns tasks to the pool",
			Priority:    5,
			Labels:      []string{"core", "go"},
			Parents:     []string{"t-epic"},
		}),
		evt(t, 3, event.TypeTaskCreated, "brandon", "t-views", event.TaskCreated{
			Title:     "Render markdown views",
			Priority:  5,
			Labels:    []string{"core"},
			Parents:   []string{"t-epic"},
			DependsOn: []string{"t-core"},
		}),
		evt(t, 4, event.TypeTaskCreated, "brandon", "t-sync", event.TaskCreated{
			Title:     "Sync loop | app-level merge",
			Priority:  2,
			DependsOn: []string{"t-views"},
		}),
		evt(t, 5, event.TypeTaskCreated, "brandon", "t-daemon", event.TaskCreated{
			Title:    "Daemon skeleton",
			Priority: 4,
			Parents:  []string{"t-epic"},
		}),
		evt(t, 6, event.TypeTaskCreated, "brandon", "t-old", event.TaskCreated{
			Title: "Spike: evaluate go-git",
		}),
		evt(t, 7, event.TypeTaskCreated, "brandon", "t-flaky", event.TaskCreated{
			Title:    "Fix flaky TestFoo",
			Priority: 8,
			Labels:   []string{"tests"},
		}),
		evt(t, 8, event.TypeClaimMade, "brandon/impl-1", "t-core", event.ClaimMade{}),
		evt(t, 9, event.TypeNoteAdded, "brandon/impl-1", "t-core", event.NoteAdded{
			Text: "Found the ordering bug: replay sorted by insertion, not ULID.\nFix in progress.",
		}),
		evt(t, 10, event.TypeEscalationRaised, "brandon/impl-1", "t-core", event.EscalationRaised{
			Question: "Should upcasters live in core or in a separate package?",
			Context:  "T3 says in-memory only; either placement satisfies that.",
			Blocking: false,
		}),
		evt(t, 11, event.TypeRunFinished, "brandon/impl-1", "t-core", event.RunFinished{
			Outcome: event.OutcomeDone,
			Branch:  "feat/replay-engine",
			PR:      "https://example.com/pr/12",
			Commits: []string{"a1b2c3d", "e4f5a6b"},
			Summary: "Replay engine with winner rule and lease expiry; 24-permutation order-insensitivity test green.",
		}),
		evt(t, 12, event.TypeEscalationAnswered, "brandon", "t-core", event.EscalationAnswered{
			Answer:     "Keep them in core; they are part of honest replay.",
			Escalation: tick(t, 10),
		}),
		evt(t, 13, event.TypeTaskUpdated, "brandon", "t-old", event.TaskUpdated{
			Status: ptr(core.StatusCancelled),
		}),
		evt(t, 14, event.TypeClaimMade, "sarah/impl-9", "t-daemon", event.ClaimMade{}),
		evt(t, 15, event.TypeNoteAdded, "sarah/impl-9", "t-daemon", event.NoteAdded{
			Text: "Socket path decided; writing the lockfile next.",
		}),
		evt(t, 16, event.TypeClaimMade, "brandon/impl-2", "t-flaky", event.ClaimMade{}),
		evt(t, 17, event.TypeEscalationRaised, "brandon/impl-2", "t-flaky", event.EscalationRaised{
			Question: "TestFoo depends on wall-clock timing — rewrite or delete?",
			Context:  "It races a 10ms sleep against the scheduler. Rewriting means faking the clock.",
			Blocking: true,
		}),
		// An out-of-band answer relayed by an agent (T5 relay_answer):
		// envelope actor is the scribe, answered_by the attribution.
		evt(t, 18, event.TypeEscalationRaised, "sarah/impl-9", "t-daemon", event.EscalationRaised{
			Question: "Reuse the repo lockfile for the daemon singleton?",
			Blocking: false,
		}),
		evt(t, 19, event.TypeEscalationAnswered, "sarah/impl-9", "t-daemon", event.EscalationAnswered{
			Answer:     "Yes — one lockfile, one meaning.",
			AnsweredBy: "sarah",
			Escalation: tick(t, 18),
		}),
	}
	leases := map[string]time.Time{
		tick(t, 14): testNow.Add(time.Hour),     // t-daemon claim alive
		tick(t, 16): base.Add(30 * time.Minute), // t-flaky lease lapsed → interrupted
	}
	return core.Input{Events: events, Leases: leases, Now: testNow}
}

func ptr[T any](v T) *T { return &v }

func goldenState(t *testing.T) *core.State {
	t.Helper()
	s, err := core.NewReplayer().Replay(goldenInput(t))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return s
}

func TestRenderGolden(t *testing.T) {
	got := views.Render(goldenState(t))

	dir := filepath.Join("testdata", "golden")
	if *update {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		for path, b := range got {
			full := filepath.Join(dir, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Every rendered path must match its golden file.
	for path, b := range got {
		want, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("missing golden file for %s (run: go test ./internal/views -update): %v", path, err)
			continue
		}
		if !bytes.Equal(b, want) {
			t.Errorf("%s differs from golden (run: go test ./internal/views -update)\n--- got ---\n%s", path, b)
		}
	}

	// And no stale golden files for paths Render no longer produces.
	err := filepath.WalkDir(dir, func(full string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, full)
		if err != nil {
			return err
		}
		if _, ok := got[filepath.ToSlash(rel)]; !ok {
			t.Errorf("stale golden file %s: Render no longer produces it", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderProducesExactPathSet(t *testing.T) {
	s := goldenState(t)
	got := views.Render(s)

	want := map[string]bool{
		"README.md":      true,
		"backlog.md":     true,
		"escalations.md": true,
		views.MetaPath:   true,
	}
	for _, id := range s.TaskOrder {
		want["tasks/"+id+".md"] = true
	}
	for path := range got {
		if !want[path] {
			t.Errorf("unexpected rendered path %s", path)
		}
	}
	for path := range want {
		if _, ok := got[path]; !ok {
			t.Errorf("missing rendered path %s", path)
		}
	}
	if meta := string(got[views.MetaPath]); meta != "{\"format\":2}\n" {
		t.Errorf("meta stamp = %q", meta)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	in := goldenInput(t)
	s1, err := core.NewReplayer().Replay(in)
	if err != nil {
		t.Fatal(err)
	}

	// Same State rendered twice.
	a := views.Render(s1)
	b := views.Render(s1)
	assertSameMaps(t, a, b, "second render of the same State")

	// Same event set in a different order (reversed) replayed and
	// rendered: replay is order-insensitive, so views must be too.
	shuffled := make([]event.Event, len(in.Events))
	for i, e := range in.Events {
		shuffled[len(in.Events)-1-i] = e
	}
	s2, err := core.NewReplayer().Replay(core.Input{Events: shuffled, Leases: in.Leases, Now: in.Now})
	if err != nil {
		t.Fatal(err)
	}
	assertSameMaps(t, a, views.Render(s2), "render of a shuffled-events replay")
}

func assertSameMaps(t *testing.T, a, b map[string][]byte, what string) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("%s: %d paths vs %d", what, len(a), len(b))
	}
	for path, ab := range a {
		bb, ok := b[path]
		if !ok {
			t.Errorf("%s: missing path %s", what, path)
			continue
		}
		if !bytes.Equal(ab, bb) {
			t.Errorf("%s: bytes differ at %s", what, path)
		}
	}
}

func TestCanWrite(t *testing.T) {
	tests := []struct {
		name string
		meta []byte
		want bool
	}{
		{"absent", nil, true},
		{"empty", []byte{}, true},
		{"garbage", []byte("not json at all"), true},
		{"wrong shape", []byte(`[1,2,3]`), true},
		{"no format field", []byte(`{}`), true},
		{"lower", []byte(`{"format":1}`), true},
		{"equal", []byte(`{"format":2}`), true},
		{"higher", []byte(`{"format":3}`), false},
		{"much higher with extras", []byte(`{"format":99,"generator":"tuhdoo v9"}`), false},
	}
	for _, tt := range tests {
		if got := views.CanWrite(tt.meta); got != tt.want {
			t.Errorf("%s: CanWrite(%q) = %v, want %v", tt.name, tt.meta, got, tt.want)
		}
	}
}

// TestNoClockReads enforces the prime directive structurally: the
// package source must never read a clock, and the output must never
// contain relative-time phrasing. Grepping source is crude but honest —
// a time.Now call anywhere would eventually leak nondeterminism.
func TestNoClockReads(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(src, []byte("time.Now")) {
			t.Errorf("%s calls time.Now: views must be a pure function of State", name)
		}
	}

	for path, b := range views.Render(goldenState(t)) {
		if bytes.Contains(b, []byte(" ago")) {
			t.Errorf("%s contains relative-time phrasing (\" ago\")", path)
		}
	}
}
