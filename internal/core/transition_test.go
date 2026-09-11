package core

import (
	"testing"
	"time"
)

// NextTransition is the memoization boundary for read-time lease
// expiry (D6 clause 5, 2026-09-10): the earliest expiry strictly after
// now. Table-driven over the shapes the daemon meets — no leases, every
// lease already lapsed, a mix, released tombstones (which the store
// decodes as leases lapsed at their stand-down instant), and ties.
func TestNextTransition(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return now.Add(d) }

	tests := []struct {
		name   string
		leases map[string]time.Time
		want   time.Time
		ok     bool
	}{
		{name: "empty", leases: nil},
		{name: "all past", leases: map[string]time.Time{
			"a": at(-time.Minute), "b": at(-time.Hour),
		}},
		{name: "expiring exactly now is not a transition", leases: map[string]time.Time{
			"a": now,
		}},
		{name: "one future", leases: map[string]time.Time{
			"a": at(5 * time.Minute),
		}, want: at(5 * time.Minute), ok: true},
		{name: "mixed picks the earliest future", leases: map[string]time.Time{
			"past":  at(-time.Minute),
			"later": at(15 * time.Minute),
			"soon":  at(2 * time.Minute),
			"now":   now,
		}, want: at(2 * time.Minute), ok: true},
		{name: "released tombstones are past by construction", leases: map[string]time.Time{
			// A stand-down pins the lease to the release instant; the
			// store hands replay that instant as an ordinary expiry.
			"released": at(-time.Second),
			"live":     at(10 * time.Minute),
		}, want: at(10 * time.Minute), ok: true},
		{name: "ties collapse to one instant", leases: map[string]time.Time{
			"a": at(3 * time.Minute), "b": at(3 * time.Minute), "c": at(4 * time.Minute),
		}, want: at(3 * time.Minute), ok: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NextTransition(tc.leases, now)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (got %v)", ok, tc.ok, got)
			}
			if ok && !got.Equal(tc.want) {
				t.Fatalf("next = %v, want %v", got, tc.want)
			}
			if !ok && !got.IsZero() {
				t.Fatalf("no transition should return the zero time, got %v", got)
			}
		})
	}
}
