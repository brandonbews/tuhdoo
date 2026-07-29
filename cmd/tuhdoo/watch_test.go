package main

// Unit tests for the watch dashboard: a Bubble Tea model renders from a
// snapshot without a TTY or a daemon, so View is tested directly. No
// PTY tests, by design.

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func watchSnapshot() *snapshot {
	raised := time.Date(2026, 7, 29, 14, 3, 0, 0, time.UTC)
	esc := escalationJSON{
		ID: "01E1", Task: "t-2", Actor: "brandon/a2",
		Question: "Which license?", Blocking: true, RaisedAt: raised,
	}
	return &snapshot{
		state: stateResp{
			Sync: syncJSON{Mode: "local-only"},
			Tasks: []stateTask{
				{ID: "t-1", Title: "write the parser", Status: "open", Priority: 5},
				{ID: "t-2", Title: "investigate the flake", Status: "open", Holder: "brandon/a1"},
				{ID: "t-3", Title: "old chore", Status: "done"},
			},
			OpenEscalations: []escalationJSON{esc},
		},
		tasks: map[string]hydratedTask{
			"t-1": {Task: taskJSON{ID: "t-1", Title: "write the parser"}},
			"t-2": {Task: taskJSON{ID: "t-2", Title: "investigate the flake"}, Escalations: []escalationJSON{esc}},
			"t-3": {Task: taskJSON{ID: "t-3", Title: "old chore"}},
		},
	}
}

func TestWatchViewRendersSeededState(t *testing.T) {
	m := watchModel{snap: watchSnapshot()}
	v := m.View()
	for _, want := range []string{
		"local-only",
		"1 ready", "1 in progress", "1 done", "1 escalation",
		"write the parser",
		"investigate the flake", "brandon/a1",
		"Which license?", "[blocking]", "raised 2026-07-29 14:03 UTC",
		"q quits",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q; view:\n%s", want, v)
		}
	}
}

func TestWatchViewBeforeFirstSnapshotAndOnError(t *testing.T) {
	var m watchModel
	if v := m.View(); !strings.Contains(v, "loading") {
		t.Errorf("empty model view should say loading; view:\n%s", v)
	}
	m.err = errors.New("dial unix: connection refused")
	if v := m.View(); !strings.Contains(v, "daemon unreachable") || !strings.Contains(v, "retrying") {
		t.Errorf("error view should report and keep retrying; view:\n%s", v)
	}
}

func TestWatchQuitKeysOnly(t *testing.T) {
	m := watchModel{snap: watchSnapshot()}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Errorf("key %q should quit", key.String())
			continue
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("key %q produced %T, want tea.QuitMsg", key.String(), cmd())
		}
	}
	// Any other key is ignored: zero input handling except quit (T7).
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Errorf("key x should be ignored, got a command")
	}
}
