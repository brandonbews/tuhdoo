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

// leaseFile is the stored JSON: one field, an RFC3339 UTC expiry at
// second precision. The claim ID lives in the filename.
type leaseFile struct {
	Expires string `json:"expires"`
}

func leasePath(claimID string) (string, error) {
	if claimID == "" || strings.ContainsAny(claimID, "/\\") || strings.Contains(claimID, "..") {
		return "", fmt.Errorf("store: invalid claim id %q", claimID)
	}
	return "leases/" + claimID + ".json", nil
}

// WriteLease creates or overwrites the lease file for claimID. Expiry is
// stored in UTC at second precision.
func (s *Store) WriteLease(claimID string, expires time.Time) error {
	path, err := leasePath(claimID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(leaseFile{Expires: expires.UTC().Truncate(time.Second).Format(time.RFC3339)})
	if err != nil {
		return fmt.Errorf("store: write lease %s: %w", claimID, err)
	}
	return s.AppendBatch(Batch{Files: map[string][]byte{path: data}})
}

// DeleteLease removes the lease file for claimID. Deleting an absent
// lease is not an error.
func (s *Store) DeleteLease(claimID string) error {
	path, err := leasePath(claimID)
	if err != nil {
		return err
	}
	return s.AppendBatch(Batch{Delete: []string{path}})
}

// ReadLeases returns every lease as claim ID → expiry (UTC).
func (s *Store) ReadLeases() (map[string]time.Time, error) {
	head, err := s.git.ReadRef(s.ref)
	if err != nil {
		return nil, fmt.Errorf("store: read leases: %w", err)
	}
	entries, err := s.git.LsTree(head)
	if err != nil {
		return nil, fmt.Errorf("store: read leases: %w", err)
	}

	leases := make(map[string]time.Time)
	for _, entry := range entries {
		claimID, ok := strings.CutPrefix(entry.Path, "leases/")
		if !ok {
			continue
		}
		claimID, ok = strings.CutSuffix(claimID, ".json")
		if !ok || strings.Contains(claimID, "/") {
			continue
		}
		data, err := s.git.CatFile(entry.OID)
		if err != nil {
			return nil, fmt.Errorf("store: read leases: %s: %w", entry.Path, err)
		}
		var lf leaseFile
		if err := json.Unmarshal(data, &lf); err != nil {
			return nil, fmt.Errorf("store: read leases: %s: %w", entry.Path, err)
		}
		expires, err := time.Parse(time.RFC3339, lf.Expires)
		if err != nil {
			return nil, fmt.Errorf("store: read leases: %s: %w", entry.Path, err)
		}
		leases[claimID] = expires.UTC()
	}
	return leases, nil
}
