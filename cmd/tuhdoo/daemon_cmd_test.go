package main

import (
	"testing"
	"time"
)

// TestLeaseTTLFromEnv: the harness knob's contract — absent means the
// daemon's own default (zero), a duration is honoured, and garbage or
// a non-positive value is an error rather than a silent default.
func TestLeaseTTLFromEnv(t *testing.T) {
	cases := []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{value: "", want: 0},
		{value: "4m", want: 4 * time.Minute},
		{value: "90s", want: 90 * time.Second},
		{value: "0", wantErr: true},
		{value: "-1m", wantErr: true},
		{value: "soon", wantErr: true},
		{value: "15", wantErr: true},
	}
	for _, c := range cases {
		got, err := leaseTTLFromEnv(c.value)
		if c.wantErr {
			if err == nil {
				t.Errorf("leaseTTLFromEnv(%q) = %s, want an error", c.value, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("leaseTTLFromEnv(%q) = %s, %v; want %s", c.value, got, err, c.want)
		}
	}
}
