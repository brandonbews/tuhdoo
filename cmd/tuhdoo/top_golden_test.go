package main

// Golden render tests for the dashboard's mock-a layout (task
// t-01KYVJ2607S5S390CVYSF3PVG4): full-width bars, the shared column
// grid, ellipsis rules, and chunk-safe windowing — all with injected
// width/height (T1: deterministic rendering, table-driven).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ansiColors is the real escape set, for tests that pin bar styling.
var ansiColors = colors{
	reset: "\x1b[0m", bold: "\x1b[1m", dim: "\x1b[2m", rev: "\x1b[7m",
	green: "\x1b[32m", yellow: "\x1b[33m", red: "\x1b[31m",
	bgMagenta: "\x1b[30;45m", bgGreen: "\x1b[30;42m",
	bgYellow: "\x1b[30;43m", bgRed: "\x1b[30;41m",
}

// The seeded fake at 80 columns, plain colors (the NO_COLOR / non-TTY
// degradation): byte-exact geometry, no escapes, no fill.
func TestTopGoldenPlain80(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width, m.height = 80, 40
	want := strings.Join([]string{
		" tuhdoo · local-only                                          acting as brandon ",
		"",
		" NEEDS INPUT (1)                                                   enter answer ",
		"▸ t-lic   !   Which license?",
		"              blocking · brandon/a2 · 2026-07-29 14:03 UTC",
		"",
		" READY (2)                                               p priority · c archive ",
		"  t-pars  p5  write the parser  · in t-epic · 1 dep",
		"  t-flor  p1  sweep the floor",
		"",
		" IN PROGRESS (1)                                                                ",
		"  t-flak      investigate the flake  ← brandon/a1",
		"",
		" BLOCKED (1)                                                                    ",
		"  t-lic       choose a license",
		"              waiting: needs input (above)",
		"",
		" ON HOLD (1)                                                          c archive ",
		"  t-park  p2  polish the docs",
		"",
		" INBOX (1)                                                i capture · c archive ",
		"  t-idea      idea: dark mode",
		"",
		" ↑/↓ (j/k) move · enter answer/open · p priority · c archive · q quit    1 done ",
		"",
	}, "\n")
	got := m.View()
	if got != want {
		t.Errorf("plain 80-column render diverged from golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("plain render leaked ANSI escapes:\n%q", got)
	}
}

// Bar composition with real colors, at 80 and 120 columns: one style
// wraps the whole padded line; counts ride the left edge, steering
// hints the right.
func TestTopGoldenBars(t *testing.T) {
	for _, width := range []int{80, 120} {
		m := newTopModel(newFakeSteering())
		m.col = ansiColors
		m.width, m.height = width, 60
		v := m.View()
		pad := func(left, right string) string {
			return left + strings.Repeat(" ", width-len([]rune(left))-len([]rune(right))) + right
		}
		for _, bar := range []string{
			"\x1b[7m\x1b[1m" + pad(" tuhdoo · local-only", "acting as brandon ") + "\x1b[0m",
			"\x1b[30;45m" + pad(" NEEDS INPUT (1)", "enter answer ") + "\x1b[0m",
			"\x1b[30;42m" + pad(" READY (2)", "p priority · c archive ") + "\x1b[0m",
			"\x1b[30;43m" + pad(" IN PROGRESS (1)", "") + "\x1b[0m",
			"\x1b[30;41m" + pad(" BLOCKED (1)", "") + "\x1b[0m",
			// The shelves (2026-07-31): reverse-dim bars, no section color
			// — present but never claiming the eye.
			"\x1b[7m\x1b[2m" + pad(" ON HOLD (1)", "c archive ") + "\x1b[0m",
			"\x1b[7m\x1b[2m" + pad(" INBOX (1)", "i capture · c archive ") + "\x1b[0m",
			"\x1b[7m\x1b[2m" + pad(" ↑/↓ (j/k) move · enter answer/open · p priority · c archive · q quit", "1 done ") + "\x1b[0m",
		} {
			if !strings.Contains(v, bar) {
				t.Errorf("width %d: view missing bar %q; view:\n%s", width, bar, v)
			}
		}
		// The blocking badge is red+bold in its own cell.
		if !strings.Contains(v, "\x1b[31m\x1b[1m! \x1b[0m") {
			t.Errorf("width %d: blocking badge not red+bold; view:\n%s", width, v)
		}
		// Shelf rows are dim: id, badge, and title all under col.dim —
		// unless the cursor lands there (bold wins; tested elsewhere).
		for _, row := range []string{
			"\x1b[2mt-park\x1b[0m  \x1b[2mp2\x1b[0m  \x1b[2mpolish the docs\x1b[0m",
			"\x1b[2mt-idea\x1b[0m      \x1b[2midea: dark mode\x1b[0m",
		} {
			if !strings.Contains(v, row) {
				t.Errorf("width %d: shelf row not dim %q; view:\n%q", width, row, v)
			}
		}
	}
}

// Watch mode: bars carry no steering hints, the header carries the
// badge, and the footer legend is the disarmed one.
func TestTopGoldenWatchBars(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.armed = false
	m.width, m.height = 80, 40
	v := m.View()
	mustContain(t, v, "watch mode", "↑/↓ (j/k) move · enter open · q quit")
	for _, absent := range []string{"enter answer", "p priority", "c archive", "i capture"} {
		if strings.Contains(v, absent) {
			t.Errorf("watch mode advertises steering key %q; view:\n%s", absent, v)
		}
	}
}

// The one-line-per-row economy: titles ellipsize with …, and the dim
// label suffix loses first — ellipsized into the remainder while the
// title has room, dropped outright when the title is near-full.
func TestTopGoldenEllipsisAndLabels(t *testing.T) {
	long := strings.Repeat("A", 70)
	mid := strings.Repeat("B", 40)
	near := strings.Repeat("C", 60)
	s := &snapshot{
		state: stateResp{Tasks: []stateTask{
			{ID: "t-lng1", Title: long, Status: "open", Priority: 1},
			{ID: "t-mid1", Title: mid, Status: "open", Priority: 1, Labels: []string{"quality", "testing", "golang"}},
			{ID: "t-nea1", Title: near, Status: "open", Priority: 1, Labels: []string{"quality"}},
		}},
		tasks: map[string]hydratedTask{
			"t-lng1": {Task: taskJSON{ID: "t-lng1"}},
			"t-mid1": {Task: taskJSON{ID: "t-mid1"}},
			"t-nea1": {Task: taskJSON{ID: "t-nea1"}},
		},
	}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 80, height: 40}
	v := m.View()
	// 80 cols - 14 grid = 66 title cells.
	mustContain(t, v,
		strings.Repeat("A", 65)+"…",      // title alone: ellipsized at 66
		mid+"  [quality, testing, gola…", // labels ellipsized into the remainder
		near)                             // near-full title renders whole...
	if strings.Contains(v, "[quality]") {
		t.Errorf("labels survived on a near-full title; view:\n%s", v)
	}
	if strings.Contains(v, strings.Repeat("A", 66)) {
		t.Errorf("over-wide title did not ellipsize; view:\n%s", v)
	}
	// No list line exceeds the terminal width.
	for _, line := range strings.Split(strings.TrimRight(v, "\n"), "\n") {
		if n := len([]rune(line)); n > 80 {
			t.Errorf("line is %d runes wide (>80): %q", n, line)
		}
	}
}

// Cursor-following windowing over chunks: two-line rows (escalations,
// blocked) are atomic — they never split across the window edge.
func TestTopGoldenWindowKeepsRowsWhole(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width = 80
	// head(2) + foot(2) + 4 available body lines.
	m.height = 8
	for i := 0; i < 4; i++ {
		m, _ = press(t, m, keyOf(tea.KeyDown))
	}
	v := m.View()
	if !strings.Contains(v, "▸ t-lic ") {
		t.Fatalf("cursor row not visible in window; view:\n%s", v)
	}
	if !strings.Contains(v, "waiting: needs input (above)") {
		t.Errorf("blocked cursor row split: second line missing; view:\n%s", v)
	}
	// The escalation two-liner obeys the same rule wherever it appears:
	// its first line present implies its second line present.
	if strings.Contains(v, "Which license?\n") && !strings.Contains(v, "blocking · brandon/a2") {
		if strings.Contains(v, "!   Which license?") {
			t.Errorf("escalation row split across the window edge; view:\n%s", v)
		}
	}
	// Window respects the height budget: head(2) + body(<=4) + foot(2).
	if n := strings.Count(v, "\n"); n > 8 {
		t.Errorf("view is %d lines, terminal is 8; view:\n%s", n, v)
	}
}
