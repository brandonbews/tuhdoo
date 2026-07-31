package core

// Scaling benchmarks for the pure replay core (T1), added while working
// the "retire full-replay-per-write" question: the daemon replays the
// full event set on every refresh, so the growth curve of Replay is what
// decides when incremental apply or snapshot-bounded replay would ever
// be worth their complexity. Run with:
//
//	go test ./internal/core -bench BenchmarkReplay -benchmem
//
// Measured on the dogfood machine (Apple M4, 2026-07-30): ~2.5µs per
// event, linear — 1k events ≈ 2.5ms, 10k ≈ 25ms, 100k ≈ 253ms per full
// replay. The event log grows by ~50 events per active dogfood day, so
// full replay stays in single-digit milliseconds for a year of use and
// under the daemon's slow-refresh threshold for several.

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// buildEventSet returns n synthetic events shaped like real dogfood
// traffic: repeating task.created / claim.made / note.added /
// run.finished cycles, with an escalation raised and answered on every
// tenth task. IDs are minted with strictly increasing timestamps so
// ULID order matches mint order, as it does live.
func buildEventSet(tb testing.TB, n int) []event.Event {
	tb.Helper()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	seq := 0
	mint := func(typ, actor, task string, payload any) event.Event {
		seq++
		id, err := event.NewID(base.Add(time.Duration(seq)*time.Millisecond), rand.Reader)
		if err != nil {
			tb.Fatalf("NewID: %v", err)
		}
		e, err := event.New(id, typ, event.Versions[typ], actor, "m-bench", task, payload)
		if err != nil {
			tb.Fatalf("event.New: %v", err)
		}
		return e
	}

	events := make([]event.Event, 0, n)
	add := func(e event.Event) bool {
		if len(events) == n {
			return false
		}
		events = append(events, e)
		return true
	}
	for i := 0; len(events) < n; i++ {
		task := fmt.Sprintf("t-bench-%06d", i)
		actor := fmt.Sprintf("bench/agent-%d", i%8)
		if !add(mint(event.TypeTaskCreated, "bench/planner", task, event.TaskCreated{
			Title:       fmt.Sprintf("benchmark task %d", i),
			Description: "synthetic task for replay scaling benchmarks",
			Priority:    i % 3,
			Labels:      []string{"bench"},
		})) {
			break
		}
		if !add(mint(event.TypeClaimMade, actor, task, event.ClaimMade{})) {
			break
		}
		if i%10 == 0 {
			raised := mint(event.TypeEscalationRaised, actor, task, event.EscalationRaised{
				Question: "which flavor?", Context: "benchmark", Blocking: false,
			})
			if !add(raised) {
				break
			}
			if !add(mint(event.TypeEscalationAnswered, "bench", task, event.EscalationAnswered{
				Answer: "that one", Escalation: raised.ID,
			})) {
				break
			}
		} else {
			if !add(mint(event.TypeNoteAdded, actor, task, event.NoteAdded{Text: "checkpoint"})) {
				break
			}
		}
		if !add(mint(event.TypeRunFinished, actor, task, event.RunFinished{
			Outcome: event.OutcomeDone, Summary: "done",
		})) {
			break
		}
	}
	return events
}

func BenchmarkReplay(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("events=%d", size), func(b *testing.B) {
			in := Input{
				Events: buildEventSet(b, size),
				Leases: map[string]time.Time{},
				Now:    time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			}
			r := NewReplayer()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := r.Replay(in); err != nil {
					b.Fatalf("Replay: %v", err)
				}
			}
		})
	}
}
