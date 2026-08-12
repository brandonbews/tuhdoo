package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Leases live as mutable per-claim files under leases/ (D9): heartbeat
// churn stays out of the permanent event log, and only the owning
// machine's daemon touches its own claims' files. Lease writes are still
// ordinary commits — they just carry no events.
//
// Lease files are never deleted, only overwritten (2026-08-04 grill).
// Replay reads a missing lease as "lapsed at every instant", so deleting
// a lease rewrites history — past claim contests re-adjudicate — and the
// sync layer's union merge resurrects one-sided deletions anyway. A
// lease that is over is closed by overwriting it with a released
// tombstone: the same file, expiry pinned to the release instant, plus
// an explicit "released" marker.

// leaseFile is the stored JSON: an RFC3339 UTC expiry at second
// precision, plus a released marker on tombstones (absent on ordinary
// leases, so their bytes are unchanged). The claim ID lives in the
// filename.
type leaseFile struct {
	Expires  string `json:"expires"`
	Released bool   `json:"released,omitempty"`
}

func leasePath(claimID string) (string, error) {
	if claimID == "" || strings.ContainsAny(claimID, "/\\") || strings.Contains(claimID, "..") {
		return "", fmt.Errorf("store: invalid claim id %q", claimID)
	}
	return "leases/" + claimID + ".json", nil
}

// encodeLease renders lease-file bytes for an expiry (UTC, second
// precision).
func encodeLease(expires time.Time) []byte {
	return encodeLeaseFile(expires, false)
}

// encodeLeaseTombstone renders released-tombstone bytes: the lease
// ended at expires, on purpose. Binaries that predate the marker ignore
// the unknown field and read an ordinary lease lapsed at that instant —
// the same verdict, minus the merge preference (leases are mutable
// files, so this is acceptable degradation, not a T3 concern).
func encodeLeaseTombstone(expires time.Time) []byte {
	return encodeLeaseFile(expires, true)
}

func encodeLeaseFile(expires time.Time, released bool) []byte {
	data, err := json.Marshal(leaseFile{
		Expires:  expires.UTC().Truncate(time.Second).Format(time.RFC3339),
		Released: released,
	})
	if err != nil {
		panic(err) // unreachable: a string and a bool cannot fail to marshal
	}
	return data
}

// DecodeLease parses lease-file bytes to the expiry they declare. A
// released tombstone reads as an ordinary lease lapsed at its instant —
// exactly what replay needs, so replay never sees the marker.
func DecodeLease(data []byte) (time.Time, error) {
	expires, _, err := DecodeLeaseState(data)
	return expires, err
}

// DecodeLeaseState parses lease-file bytes to the expiry they declare
// and whether the file is a released tombstone. The sync layer's merge
// rule is the one reader that needs the marker; files without it (the
// old format) decode as plain leases.
func DecodeLeaseState(data []byte) (time.Time, bool, error) {
	var lf leaseFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return time.Time{}, false, fmt.Errorf("store: decode lease: %w", err)
	}
	expires, err := time.Parse(time.RFC3339, lf.Expires)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("store: decode lease: %w", err)
	}
	return expires.UTC(), lf.Released, nil
}

// WriteLease creates or overwrites the lease file for claimID. Expiry is
// stored in UTC at second precision.
func (s *Store) WriteLease(claimID string, expires time.Time) error {
	path, err := leasePath(claimID)
	if err != nil {
		return err
	}
	return s.AppendBatch(Batch{Files: map[string][]byte{path: encodeLease(expires)}})
}

// ReleaseLease overwrites the lease file for claimID with a released
// tombstone stamped at the given instant: the lease was live before it
// and lapsed from it onward, at every future replay. This is the only
// way a lease ends on purpose — lease files are never deleted (see the
// package comment above).
func (s *Store) ReleaseLease(claimID string, at time.Time) error {
	path, err := leasePath(claimID)
	if err != nil {
		return err
	}
	return s.AppendBatch(Batch{Files: map[string][]byte{path: encodeLeaseTombstone(at)}})
}

// ReadLeases returns every lease as claim ID → expiry (UTC).
func (s *Store) ReadLeases() (map[string]time.Time, error) {
	_, leases, err := s.LoadReplayInput()
	return leases, err
}
