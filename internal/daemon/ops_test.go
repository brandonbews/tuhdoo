package daemon

// Unit tests for pure helpers in ops.go. Integration coverage of the
// same behavior over the wire lives in daemon_test.go.

import (
	"testing"

	"github.com/brandonbews/tuhdoo/internal/core"
)

// hasAllLabels is claim_next's label filter: all-of matching, exactly as
// documented in docs/agent-protocol.md (loop step 1).
func TestHasAllLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string // on the task
		want   []string // requested by the claim
		match  bool
	}{
		{"exact all-of match", []string{"frontend", "urgent"}, []string{"frontend", "urgent"}, true},
		{"single requested label present", []string{"frontend", "urgent"}, []string{"frontend"}, true},
		{"extra labels on the task still match", []string{"frontend", "urgent", "q3"}, []string{"frontend", "urgent"}, true},
		{"one requested label missing", []string{"frontend"}, []string{"frontend", "urgent"}, false},
		{"unlabelled task never matches a labelled request", nil, []string{"frontend"}, false},
		{"empty request matches a labelled task", []string{"frontend"}, nil, true},
		{"empty request matches an unlabelled task", nil, nil, true},
	}
	for _, tt := range tests {
		task := &core.Task{Labels: tt.labels}
		if got := hasAllLabels(task, tt.want); got != tt.match {
			t.Errorf("%s: hasAllLabels(labels=%v, want=%v) = %v, want %v",
				tt.name, tt.labels, tt.want, got, tt.match)
		}
	}
}
