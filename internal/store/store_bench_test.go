package store

// Scaling benchmarks for the branch load path, the measured bottleneck
// of the daemon's refresh (t-01KYRMFV10W1N28TCN5ZZ9Z2C1): "cold" is a
// Store with empty decode caches — the pre-cache behavior, one `git
// cat-file` subprocess per blob — and "warm" is the steady state, one
// rev-parse plus one ls-tree regardless of event count. Real git, real
// subprocesses; setup for the 1000-event repo takes several seconds, so
// run these deliberately:
//
//	go test ./internal/store -bench BenchmarkLoadReplayInput -benchtime 3x -run '^$'
//
// Measured on the dogfood machine (Apple M4, 2026-07-30): cold ~6.5ms
// per event (100 events ≈ 0.69s, 1000 ≈ 6.5s); warm ≈ 12-13ms flat at
// both sizes.

import (
	"fmt"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

func BenchmarkLoadReplayInput(b *testing.B) {
	for _, size := range []int{100, 1000} {
		b.Run(fmt.Sprintf("events=%d", size), func(b *testing.B) {
			s, _ := newStore(b)
			evs := make([]event.Event, size)
			for i := range evs {
				evs[i] = newEvent(b, i)
			}
			if err := s.AppendBatch(Batch{Events: evs}); err != nil {
				b.Fatalf("AppendBatch: %v", err)
			}
			expires := time.Now().Add(time.Hour)
			for i := 0; i < 5; i++ {
				if err := s.WriteLease(fmt.Sprintf("c%d", i), expires); err != nil {
					b.Fatalf("WriteLease: %v", err)
				}
			}

			b.Run("cold", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					fresh := New(s.git, s.ref, s.ident)
					if _, _, err := fresh.LoadReplayInput(); err != nil {
						b.Fatalf("LoadReplayInput: %v", err)
					}
				}
			})
			b.Run("warm", func(b *testing.B) {
				if _, _, err := s.LoadReplayInput(); err != nil {
					b.Fatalf("LoadReplayInput: %v", err)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, _, err := s.LoadReplayInput(); err != nil {
						b.Fatalf("LoadReplayInput: %v", err)
					}
				}
			})
		})
	}
}
