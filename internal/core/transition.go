package core

import "time"

// NextTransition answers the next instant at which any lease's liveness
// changes: the earliest expiry strictly after now, and whether there is
// one (D6 clause 5, 2026-09-10 — read-time expiry, memoized between
// lease transitions). A lease is live until its expiry and lapsed from
// it onward (the same "!exp.After(now)" replay applies), so a state
// replayed at now stays exactly right until the instant returned here;
// the daemon re-replays once then instead of on every read.
//
// The input is the same claim ID → expiry map replay consumes. Released
// tombstones need no special case: the store decodes a tombstone as an
// ordinary lease lapsed at its stand-down instant, which is in the past
// by the time anyone asks, and a past expiry is never a transition —
// its lapse already happened. Ties collapse to one instant. Pure: no
// clock, no I/O, deterministic over the same inputs.
func NextTransition(leases map[string]time.Time, now time.Time) (time.Time, bool) {
	var next time.Time
	found := false
	for _, exp := range leases {
		if !exp.After(now) {
			continue
		}
		if !found || exp.Before(next) {
			next = exp
			found = true
		}
	}
	return next, found
}
