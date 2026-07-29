package event

import (
	"fmt"
	"io"
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
