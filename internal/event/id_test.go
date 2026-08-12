package event

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestNewIDDeterministic(t *testing.T) {
	when := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	entropy := func() *bytes.Reader {
		return bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	}

	a, err := NewID(when, entropy())
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewID(when, entropy())
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("same time+entropy produced different IDs: %s vs %s", a, b)
	}
	if len(a) != 26 {
		t.Errorf("ID length = %d, want 26: %s", len(a), a)
	}
}

func TestNewIDEntropyExhausted(t *testing.T) {
	if _, err := NewID(time.Unix(0, 0).UTC(), bytes.NewReader(nil)); err == nil {
		t.Error("NewID with empty entropy should fail")
	}
}

// TestPathIsPureFunctionOfID: property-style sweep — for many IDs built
// from known timestamps, the path is stable across calls and its date
// part is exactly the ID's embedded UTC date.
func TestPathIsPureFunctionOfID(t *testing.T) {
	times := []time.Time{
		time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1999, 12, 31, 23, 59, 59, 999e6, time.UTC),
		time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 59, 999e6, time.UTC),
		time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2100, 2, 28, 6, 30, 0, 0, time.UTC),
		// Non-UTC wall clock: the path must use the UTC date (Jun 1
		// 03:30 in UTC+11 is May 31 16:30 UTC).
		time.Date(2026, 6, 1, 3, 30, 0, 0, time.FixedZone("UTC+11", 11*3600)),
	}

	for _, when := range times {
		for fill := byte(0); fill < 8; fill++ {
			entropy := bytes.Repeat([]byte{fill}, 10)
			id, err := NewID(when, bytes.NewReader(entropy))
			if err != nil {
				t.Fatal(err)
			}

			p1, err := Path(id)
			if err != nil {
				t.Fatalf("Path(%s): %v", id, err)
			}
			p2, err := Path(id)
			if err != nil {
				t.Fatal(err)
			}
			if p1 != p2 {
				t.Errorf("Path not stable for %s: %s vs %s", id, p1, p2)
			}

			// ULID timestamps are millisecond-truncated UTC.
			day := when.UTC().Truncate(time.Millisecond)
			want := fmt.Sprintf("events/%04d/%02d/%02d/%s.json",
				day.Year(), int(day.Month()), day.Day(), id)
			if p1 != want {
				t.Errorf("Path(%s) = %s, want %s", id, p1, want)
			}
		}
	}
}

func TestPathKnownValue(t *testing.T) {
	// The timestamp prefix 01BX5ZZKBK decodes to 1508808576371 ms after
	// the epoch: 2017-10-24T02:09:36.371Z.
	got, err := Path("01BX5ZZKBKACTAV9WEVGEMMVRY")
	if err != nil {
		t.Fatal(err)
	}
	want := "events/2017/10/24/01BX5ZZKBKACTAV9WEVGEMMVRY.json"
	if got != want {
		t.Errorf("Path = %s, want %s", got, want)
	}
}

// ShortID's documented contract: the prefix comes from the ID itself
// (both the tuh- era and the older t- era), the tail is the ULID's last
// four characters lowercased, and inputs too short to abbreviate pass
// through whole. A bare ULID with no hyphen loses its (absent) prefix
// and keeps only the lowercased tail — pinned as current behavior.
func TestShortIDAbbreviation(t *testing.T) {
	tests := []struct{ in, want string }{
		{"tuh-01KYRMFV10W1N28TCN5ZZ9Z2C1", "tuh-z2c1"},
		{"t-01BX5ZZKBKACTAV9WEVGEMMVRY", "t-mvry"},
		{"esc-01ABCDEFGH", "esc-efgh"},
		{"t-abc", "t-abc"},
		{"01BX5ZZKBKACTAV9WEVGEMMVRY", "mvry"},
	}
	for _, tt := range tests {
		if got := ShortID(tt.in); got != tt.want {
			t.Errorf("ShortID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPathRejectsInvalidID(t *testing.T) {
	for _, id := range []string{"", "not-a-ulid", "01BX5ZZKBK", "01BX5ZZKBKACTAV9WEVGEMMVRI"} {
		if _, err := Path(id); err == nil {
			t.Errorf("Path(%q) accepted an invalid ID", id)
		}
	}
}
