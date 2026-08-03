package main

// Golden render tests for the dashboard's mock-a layout (task
// t-01KYVJ2607S5S390CVYSF3PVG4): full-width bars, the shared column
// grid, ellipsis rules, and chunk-safe windowing — all with injected
// width/height (T1: deterministic rendering, table-driven).

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// ansiColors is the real escape set, for tests that pin bar styling.
var ansiColors = colors{
	reset: "\x1b[0m", bold: "\x1b[1m", dim: "\x1b[2m", rev: "\x1b[7m",
	green: "\x1b[32m", yellow: "\x1b[33m", red: "\x1b[31m", magenta: "\x1b[35m",
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
		"▌ t-lic   !   choose a license",
		"▌             question: Which license?",
		"▌             brandon/a2 · 2026-07-29 14:03 UTC",
		"",
		" READY (2)                                                p priority · c cancel ",
		"  t-pars  p5  write the parser  · in t-epic · 1 dep",
		"  t-flor  p1  sweep the floor",
		"",
		" IN PROGRESS (1)                                                                ",
		"  t-flak      investigate the flake  ← brandon/a1",
		"",
		" BLOCKED (0)                                                                    ",
		"  none",
		"",
		" ON HOLD (1)                                                           c cancel ",
		"  t-park  p2  polish the docs",
		"",
		" INBOX (1)                                                 i capture · c cancel ",
		"  t-idea      idea: dark mode",
		"",
		// The armed legend grew "h history" (2026-08-02); at exactly 80
		// columns the done tally is the sacrificed right text (barLine's
		// rule) — it returns on wider terminals.
		" ↑/↓ (j/k) move · enter open · p priority · c cancel · h history · q quit       ",
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
		// The done tally survives only where the widened legend leaves
		// room (barLine drops the right text first): gone at 80, back at
		// 120.
		tally := ""
		if width >= 120 {
			tally = "1 done "
		}
		for _, bar := range []string{
			"\x1b[7m\x1b[1m" + pad(" tuhdoo · local-only", "acting as brandon ") + "\x1b[0m",
			"\x1b[30;45m" + pad(" NEEDS INPUT (1)", "enter answer ") + "\x1b[0m",
			"\x1b[30;42m" + pad(" READY (2)", "p priority · c cancel ") + "\x1b[0m",
			"\x1b[30;43m" + pad(" IN PROGRESS (1)", "") + "\x1b[0m",
			"\x1b[30;41m" + pad(" BLOCKED (0)", "") + "\x1b[0m",
			// The shelves (2026-07-31): reverse-dim bars, no section color
			// — present but never claiming the eye.
			"\x1b[7m\x1b[2m" + pad(" ON HOLD (1)", "c cancel ") + "\x1b[0m",
			"\x1b[7m\x1b[2m" + pad(" INBOX (1)", "i capture · c cancel ") + "\x1b[0m",
			"\x1b[7m\x1b[2m" + pad(" ↑/↓ (j/k) move · enter open · p priority · c cancel · h history · q quit", tally) + "\x1b[0m",
		} {
			if !strings.Contains(v, bar) {
				t.Errorf("width %d: view missing bar %q; view:\n%s", width, bar, v)
			}
		}
		// The blocking badge is red+bold in its own cell.
		if !strings.Contains(v, "\x1b[31m\x1b[1m! \x1b[0m") {
			t.Errorf("width %d: blocking badge not red+bold; view:\n%s", width, v)
		}
		// The question line's lead is foreground magenta — bgMagenta
		// stays the bar style.
		if !strings.Contains(v, "\x1b[35mquestion: \x1b[0m") {
			t.Errorf("width %d: question lead not magenta; view:\n%s", width, v)
		}
		// Shelf rows keep dim id and badge, but titles are bold in every
		// section (2026-07-31) — the shelves recede less, accepted.
		for _, row := range []string{
			"\x1b[2mt-park\x1b[0m  \x1b[2mp2\x1b[0m  \x1b[1mpolish the docs\x1b[0m",
			"\x1b[2mt-idea\x1b[0m      \x1b[1midea: dark mode\x1b[0m",
			"\x1b[2mt-pars\x1b[0m  \x1b[2mp5\x1b[0m  \x1b[1mwrite the parser\x1b[0m",
		} {
			if !strings.Contains(v, row) {
				t.Errorf("width %d: row styling wrong %q; view:\n%q", width, row, v)
			}
		}
	}
}

// The selection bar (2026-07-31): every line of the selected chunk
// opens with the bg code and the ▌ gutter, the bg re-applies after
// each internal reset, and each line pads to the full width — one
// continuous bar. Unselected rows carry neither bg nor gutter.
func TestTopGoldenSelectionBar(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.col = ansiColors
	m.col.selBG = "\x1b[48;5;236m"
	m.width, m.height = 80, 40
	v := m.View()
	var sel, rest []string
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "▌") {
			sel = append(sel, l)
		} else {
			rest = append(rest, l)
		}
	}
	if len(sel) != 3 {
		t.Fatalf("selected escalation chunk has %d gutter lines, want 3; view:\n%q", len(sel), v)
	}
	for _, l := range sel {
		if !strings.HasPrefix(l, "\x1b[48;5;236m▌ ") {
			t.Errorf("selected line does not open with bg+gutter: %q", l)
		}
		if !strings.HasSuffix(l, "\x1b[0m") {
			t.Errorf("selected line does not close with a reset: %q", l)
		}
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("selected line is %d cells, want the full 80: %q", w, l)
		}
	}
	// The title line carries styled spans, so the bar must re-apply its
	// bg after every internal reset or it drops out mid-line.
	if !strings.Contains(sel[0], "\x1b[0m\x1b[48;5;236m") {
		t.Errorf("bg not re-applied after an internal reset: %q", sel[0])
	}
	for _, l := range rest {
		if strings.Contains(l, "48;5;236") {
			t.Errorf("selection bg leaked onto an unselected line: %q", l)
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
	mustContain(t, v, "watch mode", "↑/↓ (j/k) move · enter open · h history · q quit")
	for _, absent := range []string{"enter answer", "p priority", "c cancel", "i capture"} {
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

// The task view at 80 columns, plain colors (task-view rework,
// 2026-08-01): the dashboard's header bar, the field block (bold names
// degrade to plain), the NEEDS INPUT section with the routed-to
// escalation as the selected row, DESCRIPTION and HISTORY bars, and the
// footer bar — byte-exact geometry, entered by pressing enter on the
// dashboard's Needs Input row.
func TestTopGoldenTaskViewPlain80(t *testing.T) {
	m := newTopModel(newFakeSteering())
	h := m.snap.tasks["t-lic"]
	h.Task.Status = "open"
	h.Task.CreatedAt = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	h.Task.CreatedBy = "brandon"
	m.snap.tasks["t-lic"] = h
	m.width, m.height = 80, 40
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-lic" || m.detailFocus != 2 {
		t.Fatalf("enter on the Needs Input row: mode %d detail %q focus %d, want t-lic with its escalation stop preselected",
			m.mode, m.detailID, m.detailFocus)
	}
	want := strings.Join([]string{
		" tuhdoo · local-only                                          acting as brandon ",
		"",
		"  t-lic — choose a license",
		"",
		"  id          t-lic",
		"  status      open",
		"  priority    0",
		"  created     2026-07-29 12:00 UTC by brandon",
		"",
		" NEEDS INPUT (1)                                                   enter answer ",
		"▌ !   Which license?",
		"▌     brandon/a2 · 2026-07-29 14:03 UTC",
		"",
		" DESCRIPTION                                                                    ",
		"  none",
		"",
		" HISTORY                                                                        ",
		"  no activity yet",
		"",
		" ↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit        ",
		"",
	}, "\n")
	got := m.View()
	if got != want {
		t.Errorf("plain 80-column task view diverged from golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("plain task view leaked ANSI escapes:\n%q", got)
	}
}

// A plain open (from a task row) focuses the title stop: at plain
// colors the selection degrades to the ▌ gutter alone, on the title
// line's two-cell mark column; every other stop keeps its blank mark
// cells (focused-field rendering, 2026-08-02).
func TestTopGoldenTaskViewFocusedTitlePlain80(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width, m.height = 80, 40
	m = openDetail(t, m, "t-pars")
	v := m.View()
	mustContain(t, v,
		"▌ t-pars — write the parser",
		"  priority    0", // an unfocused stop keeps the blank mark column
		"  none")          // the empty-description placeholder, unfocused
	if n := strings.Count(v, "▌"); n != 1 {
		t.Errorf("%d gutter lines, want exactly 1 (the title); view:\n%s", n, v)
	}
	if strings.Contains(v, "\x1b") {
		t.Errorf("plain render leaked ANSI escapes:\n%q", v)
	}
}

// Task-view composition with real colors: the NEEDS INPUT bar carries
// the dashboard's magenta, DESCRIPTION and HISTORY the reverse-dim
// bars, field names are bold on the grid, and the selected escalation
// row is the same full-height gutter bar as the list. Watch mode keeps
// the sections but drops the hint and the selection.
func TestTopGoldenTaskViewBarsAndSelection(t *testing.T) {
	m := newTopModel(newFakeSteering())
	h := m.snap.tasks["t-lic"]
	h.Task.Status = "open"
	m.snap.tasks["t-lic"] = h
	m.col = ansiColors
	m.col.selBG = "\x1b[48;5;236m"
	m.width, m.height = 80, 40
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	v := m.View()
	pad := func(left, right string) string {
		return left + strings.Repeat(" ", 80-len([]rune(left))-len([]rune(right))) + right
	}
	for _, want := range []string{
		"\x1b[30;45m" + pad(" NEEDS INPUT (1)", "enter answer ") + "\x1b[0m",
		"\x1b[7m\x1b[2m" + pad(" DESCRIPTION", "") + "\x1b[0m",
		"\x1b[7m\x1b[2m" + pad(" HISTORY", "") + "\x1b[0m",
		"\x1b[7m\x1b[2m" + pad(" ↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit", "") + "\x1b[0m",
		// Bold field names on the grid; the canonical id value stays dim.
		"  \x1b[1mid\x1b[0m          \x1b[2mt-lic\x1b[0m",
		"  \x1b[1mstatus\x1b[0m      open",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("task view missing %q; view:\n%q", want, v)
		}
	}
	// The selected escalation row: gutter and tint on both lines, each
	// padded to the full width — one continuous bar, like the list.
	var sel []string
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "▌") {
			sel = append(sel, l)
		}
	}
	if len(sel) != 2 {
		t.Fatalf("selected escalation row has %d gutter lines, want 2; view:\n%q", len(sel), v)
	}
	for _, l := range sel {
		if !strings.HasPrefix(l, "\x1b[48;5;236m▌ ") {
			t.Errorf("selected line does not open with bg+gutter: %q", l)
		}
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("selected line is %d cells, want the full 80: %q", w, l)
		}
	}
	// Watch mode: same view, no steering hint, nothing selected.
	w := newWatchModel()
	w.col = ansiColors
	w.width, w.height = 80, 40
	w, _ = press(t, w, keyOf(tea.KeyEnter))
	wv := w.View()
	if !strings.Contains(wv, "\x1b[30;45m"+pad(" NEEDS INPUT (1)", "")+"\x1b[0m") {
		t.Errorf("watch task view missing the hint-free NEEDS INPUT bar; view:\n%q", wv)
	}
	if strings.Contains(wv, "▌") {
		t.Errorf("watch task view renders a selected row; view:\n%q", wv)
	}
}

// History mode at 80 columns, plain colors (history view, 2026-08-02):
// the two-bar layout — DONE then CANCELLED, each newest close first —
// rows on the ready-row anatomy with the dim close stamp and closing
// actor appended, and the read-only footer legend. Byte-exact.
func TestTopGoldenHistoryPlain80(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	m.width, m.height = 80, 40
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	want := strings.Join([]string{
		" tuhdoo · local-only                                          acting as brandon ",
		"",
		" DONE (3)                                                                       ",
		"▌ t-ship      ship the tui  [tui]  · 2026-07-30 · brandon/claude-code-1",
		"  t-mgr8      migrate the backlog  · 1 dep  · 2026-07-29 · brandon",
		"  t-chor      old chore  · 2026-07-28 · brandon/impl-1",
		"",
		" CANCELLED (2)                                                                  ",
		"  t-zzzz      zombie idea  · 2026-07-31 · brandon/a2",
		"  t-drop      drop the wiki  · 2026-07-27 · brandon",
		"",
		" ↑/↓ (j/k) move · enter open · esc back · q quit                                ",
		"",
	}, "\n")
	got := m.View()
	if got != want {
		t.Errorf("plain 80-column history render diverged from golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("plain history render leaked ANSI escapes:\n%q", got)
	}
}

// History bar composition with real colors: DONE keeps the ready
// green, CANCELLED the shelves' reverse-dim, neither carries a hint,
// and a row's title stays bold while its close suffix rides dim with
// the labels.
func TestTopGoldenHistoryBars(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	m.col = ansiColors
	m.width, m.height = 80, 40
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	v := m.View()
	pad := func(left, right string) string {
		return left + strings.Repeat(" ", 80-len([]rune(left))-len([]rune(right))) + right
	}
	for _, want := range []string{
		"\x1b[30;42m" + pad(" DONE (3)", "") + "\x1b[0m",
		"\x1b[7m\x1b[2m" + pad(" CANCELLED (2)", "") + "\x1b[0m",
		"\x1b[7m\x1b[2m" + pad(" ↑/↓ (j/k) move · enter open · esc back · q quit", "") + "\x1b[0m",
		"\x1b[2mt-mgr8\x1b[0m      \x1b[1mmigrate the backlog\x1b[0m\x1b[2m  · 1 dep  · 2026-07-29 · brandon\x1b[0m",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("history view missing %q; view:\n%q", want, v)
		}
	}
}

// The task view of a cancelled task at 80 columns, plain colors,
// entered from history: the status line carries the close metadata,
// the unanswered escalation renders in HISTORY as record — no NEEDS
// INPUT section — and the armed footer advertises no p/c. Byte-exact.
func TestTopGoldenTaskViewTerminalPlain80(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	m.width, m.height = 80, 40
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = moveTo(t, m, "t-drop")
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-drop" {
		t.Fatalf("mode %d detail %q, want modeDetail t-drop", m.mode, m.detailID)
	}
	want := strings.Join([]string{
		" tuhdoo · local-only                                          acting as brandon ",
		"",
		"▌ t-drop — drop the wiki",
		"",
		"  id          t-drop",
		"  status      cancelled — 2026-07-27 by brandon",
		"  priority    0",
		"  created     2026-07-20 12:00 UTC by brandon",
		"",
		" DESCRIPTION                                                                    ",
		"  none",
		"",
		" HISTORY                                                                        ",
		"  2026-07-25 09:00 UTC  escalation from brandon/a7",
		"    Q: Keep the wiki export?",
		"    unanswered",
		"",
		" ↑/↓ (j/k) move · enter edit · esc back · q quit                                ",
		"",
	}, "\n")
	got := m.View()
	if got != want {
		t.Errorf("plain 80-column terminal task view diverged from golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("plain terminal task view leaked ANSI escapes:\n%q", got)
	}
}

// The task view's history entries with real colors (entry formatting,
// 2026-08-03): dim stamp, bold descriptor on all three entry kinds —
// the run outcome folds into its bold descriptor — and exactly one
// blank line between consecutive entries.
func TestTopGoldenTaskViewHistoryEntries(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.col = ansiColors
	m.width, m.height = 80, 40
	m = openDetail(t, m, "t-flak")
	v := m.View()
	// ULID order: escalation 01E2, then note 01N1, then run 01R1.
	want := "  \x1b[2m2026-07-29 15:30 UTC\x1b[0m  \x1b[1mescalation from brandon/a1\x1b[0m\n" +
		"    Q: Skip the flaky test until fixed?\n" +
		"    A (brandon, relayed by brandon/a1): Skip it, link the issue.\n" +
		"\n" +
		"  \x1b[2m2026-07-29 15:00 UTC\x1b[0m  \x1b[1mnote by brandon/a1\x1b[0m\n" +
		"    Repros only under -race.\n" +
		"\n" +
		"  \x1b[2m(unknown time)\x1b[0m  \x1b[1mrun by brandon/a1 — interrupted\x1b[0m\n" +
		"    Bisecting the flake."
	if !strings.Contains(v, want) {
		t.Errorf("task view history entries diverged.\nwant block:\n%q\nview:\n%q", want, v)
	}
}

// Cursor-following windowing over chunks: multi-line rows (the 3-line
// escalation row, blocked two-liners) are atomic — they never split
// across the window edge.
func TestTopGoldenWindowKeepsRowsWhole(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width = 80
	// head(2) + foot(2) + 4 available body lines: the NEEDS INPUT bar
	// plus the 3-line escalation row exactly fill the window.
	m.height = 8
	v := m.View()
	if !strings.Contains(v, "▌ t-lic ") {
		t.Fatalf("cursor row not visible in window; view:\n%s", v)
	}
	for _, line := range []string{"question: Which license?", "brandon/a2 · 2026-07-29 14:03 UTC"} {
		if !strings.Contains(v, line) {
			t.Errorf("escalation row split: %q missing; view:\n%s", line, v)
		}
	}
	// Walking to the bottom scrolls the escalation off whole: no
	// orphaned question or meta line survives without its title line.
	for i := 0; i < 5; i++ {
		m, _ = press(t, m, keyOf(tea.KeyDown))
	}
	v = m.View()
	if !strings.Contains(v, "▌ t-idea") {
		t.Fatalf("cursor row not visible after scrolling; view:\n%s", v)
	}
	if strings.Contains(v, "question:") || strings.Contains(v, "choose a license") {
		t.Errorf("escalation row partially visible after scrolling; view:\n%s", v)
	}
	// Window respects the height budget: head(2) + body(<=4) + foot(2).
	if n := strings.Count(v, "\n"); n > 8 {
		t.Errorf("view is %d lines, terminal is 8; view:\n%s", n, v)
	}
}
