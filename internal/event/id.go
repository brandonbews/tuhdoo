package event

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewID returns a new event ID: a 26-character uppercase ULID whose
// timestamp part is taken from now. Time and entropy are injected so the
// function stays pure and testable; production callers pass time.Now()
// and a crypto/rand-backed reader at the call site, never inside here.
func NewID(now time.Time, entropy io.Reader) (string, error) {
	id, err := ulid.New(ulid.Timestamp(now), entropy)
	if err != nil {
		return "", fmt.Errorf("event: new id: %w", err)
	}
	return id.String(), nil
}

// IDTime returns the UTC timestamp embedded in an event ID. Replay uses
// it for the claim winner rule (D6): deterministic given the ID, whether
// or not the writing machine's clock was truthful.
func IDTime(id string) (time.Time, error) {
	u, err := ulid.ParseStrict(id)
	if err != nil {
		return time.Time{}, fmt.Errorf("event: id time: invalid id %q: %w", id, err)
	}
	return ulid.Time(u.Time()).UTC(), nil
}

// ShortID abbreviates a prefixed task ID for display: the ID's own type
// prefix (everything through the first hyphen — `tuh-` for tasks minted
// after the 2026-07-31 rebrand, `t-` for older ones) plus the ULID's
// last four characters, lowercased (`tuh-d83w`). The tail is where
// same-batch ULIDs actually differ — their timestamp prefixes match —
// so abbreviation comes from the right-hand end. Display and input
// sugar only (T7): stored and transmitted IDs stay full-length.
func ShortID(id string) string {
	i := strings.Index(id, "-")
	tail := id[i+1:]
	if len(tail) <= 4 {
		return id
	}
	return id[:i+1] + strings.ToLower(tail[len(tail)-4:])
}

// Path derives the date-sharded storage path for an event ID:
// "events/YYYY/MM/DD/<ULID>.json", using the UTC date embedded in the
// ULID's timestamp. It is a pure function of the ID alone.
func Path(id string) (string, error) {
	u, err := ulid.ParseStrict(id)
	if err != nil {
		return "", fmt.Errorf("event: path: invalid id %q: %w", id, err)
	}
	t := ulid.Time(u.Time()).UTC()
	return fmt.Sprintf("events/%04d/%02d/%02d/%s.json",
		t.Year(), int(t.Month()), t.Day(), u.String()), nil
}
