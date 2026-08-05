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
	dimRed:    "\x1b[2;31m",
	bgMagenta: "\x1b[30;45m", bgGreen: "\x1b[30;42m",
	bgYellow: "\x1b[30;43m", bgRed: "\x1b[2;41m",
	bgGray: "\x1b[2;100m", bgWhite: "\x1b[30;107m",
}

// legendKey and legendSep compose the expected bytes of the unfilled
// footer legend (chrome hierarchy, 2026-08-03): bold key token, dim
// label, dim · separators.
func legendKey(key, label string) string {
	return "\x1b[1m" + key + "\x1b[0m\x1b[2m " + label + "\x1b[0m"
}

const legendSep = "\x1b[2m · \x1b[0m"

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
	}, "\n") + "\n" +
		// The footer pins to the bottom row (chrome hierarchy,
		// 2026-08-03): rows 23-39 are the blank pad, row 40 the footer.
		strings.Repeat("\n", 17) +
		// The armed legend grew "h history" (2026-08-02); at exactly 80
		// columns the done tally is the sacrificed right text (barLine's
		// rule, kept by segLine) — it returns on wider terminals. The
		// pinned frame's last line is unterminated: bubbletea drops
		// overflow from the top, so a trailing newline (an extra empty
		// split-line) would cost the header row.
		" ↑/↓ (j/k) move · enter open · p priority · c cancel · h history · q quit       "
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
		// The unfilled footer (chrome hierarchy, 2026-08-03): bold keys,
		// dim labels and separators, an unstyled space fill. The done
		// tally survives only where the widened legend leaves room
		// (segLine keeps barLine's drop-the-right-first rule): gone at
		// 80, back at 120. The legend's visible width is 73 cells.
		foot := " " + legendKey("↑/↓ (j/k)", "move") + legendSep + legendKey("enter", "open") +
			legendSep + legendKey("p", "priority") + legendSep + legendKey("c", "cancel") +
			legendSep + legendKey("h", "history") + legendSep + legendKey("q", "quit")
		if width >= 120 {
			foot += strings.Repeat(" ", width-73-7) + "\x1b[2m1 done \x1b[0m"
		} else {
			foot += strings.Repeat(" ", width-73)
		}
		for _, bar := range []string{
			// The unfilled header (chrome hierarchy, 2026-08-03): tuhdoo
			// bold, sync text dim, the armed badge at normal weight — no
			// fill, so the frame stops competing with content.
			"\x1b[1m tuhdoo\x1b[0m\x1b[2m · local-only\x1b[0m" +
				strings.Repeat(" ", width-38) + "acting as brandon ",
			"\x1b[30;45m" + pad(" NEEDS INPUT (1)", "enter answer ") + "\x1b[0m",
			"\x1b[30;42m" + pad(" READY (2)", "p priority · c cancel ") + "\x1b[0m",
			"\x1b[30;43m" + pad(" IN PROGRESS (1)", "") + "\x1b[0m",
			// BLOCKED is dim red (bar recolors, 2026-08-04): unmet deps
			// are sequencing, not a fire.
			"\x1b[2;41m" + pad(" BLOCKED (0)", "") + "\x1b[0m",
			// The shelves split (chrome hierarchy, 2026-08-03): ON HOLD is
			// shelved and takes the dark-gray bar (dim on bright black);
			// INBOX awaits attention and takes black on bright-white (bar
			// recolors, 2026-08-04 — was reverse-dim).
			"\x1b[2;100m" + pad(" ON HOLD (1)", "c cancel ") + "\x1b[0m",
			"\x1b[30;107m" + pad(" INBOX (1)", "i capture · c cancel ") + "\x1b[0m",
			foot,
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

// The blocked row's waiting: lead is dim red, matching its section bar
// (bar recolors, 2026-08-04): a full-brightness red lead would be
// louder than the bar above it.
func TestTopGoldenBlockedWaitingLead(t *testing.T) {
	s := topSnapshot()
	s.state.Tasks = append(s.state.Tasks,
		stateTask{ID: "t-wait", Title: "build on the idea", Status: "open",
			Situation: "blocked", UnmetDeps: []string{"t-flor"}})
	s.tasks["t-wait"] = hydratedTask{Task: taskJSON{
		ID: "t-wait", Title: "build on the idea", DependsOn: []string{"t-flor"},
	}}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s),
		col: ansiColors, width: 120, height: 60}
	v := m.View()
	if !strings.Contains(v, "\x1b[2;31mwaiting: \x1b[0m\x1b[2m") {
		t.Errorf("blocked waiting: lead is not dim red; view:\n%q", v)
	}
	if strings.Contains(v, "\x1b[31mwaiting:") {
		t.Errorf("blocked waiting: lead still full-brightness red; view:\n%q", v)
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
			{ID: "t-lng1", Title: long, Status: "open", Priority: 1, Situation: "ready"},
			{ID: "t-mid1", Title: mid, Status: "open", Priority: 1, Labels: []string{"quality", "testing", "golang"}, Situation: "ready"},
			{ID: "t-nea1", Title: near, Status: "open", Priority: 1, Labels: []string{"quality"}, Situation: "ready"},
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
	if m.mode != modeDetail || m.detailID != "t-lic" || m.detailFocus != 3 {
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
		// The labels line always renders — the dim placeholder degrades
		// to plain "none" here (labels editable, 2026-08-05).
		"  labels      none",
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
	}, "\n") + "\n" +
		// Footer pinned to row 40 (chrome hierarchy, 2026-08-03): rows
		// 20-39 are the blank pad.
		strings.Repeat("\n", 20) +
		" ↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit        "
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
		"  priority    0",    // an unfocused stop keeps the blank mark column
		"  labels      none", // the always-rendered labels line, unfocused
		"  none")             // the empty-description placeholder, unfocused
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
		// DESCRIPTION and HISTORY keep reverse-dim: neutral structural
		// chrome, not shelf (chrome hierarchy, 2026-08-03).
		"\x1b[7m\x1b[2m" + pad(" DESCRIPTION", "") + "\x1b[0m",
		"\x1b[7m\x1b[2m" + pad(" HISTORY", "") + "\x1b[0m",
		// The unfilled footer: bold keys, dim labels, no fill. The
		// legend's visible width is 72 cells, so 8 cells of plain pad.
		" " + legendKey("↑/↓ (j/k)", "move") + legendSep + legendKey("enter", "edit") +
			legendSep + legendKey("p", "priority") + legendSep + legendKey("c", "cancel") +
			legendSep + legendKey("esc", "back") + legendSep + legendKey("q", "quit") +
			strings.Repeat(" ", 8),
		// Bold field names on the grid; the canonical id value stays dim.
		"  \x1b[1mid\x1b[0m          \x1b[2mt-lic\x1b[0m",
		"  \x1b[1mstatus\x1b[0m      open",
		// The empty labels line: the placeholder is dim, like the empty
		// description body (labels editable, 2026-08-05).
		"  \x1b[1mlabels\x1b[0m      \x1b[2mnone\x1b[0m",
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
	}, "\n") + "\n" +
		// Footer pinned to row 40 (chrome hierarchy, 2026-08-03): rows
		// 11-39 are the blank pad.
		strings.Repeat("\n", 29) +
		" ↑/↓ (j/k) move · enter open · esc back · q quit                                "
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
		// CANCELLED is shelved: the dark-gray bar (chrome hierarchy,
		// 2026-08-03), same as ON HOLD on the dashboard.
		"\x1b[2;100m" + pad(" CANCELLED (2)", "") + "\x1b[0m",
		// The unfilled footer: bold keys, dim labels, no fill. The
		// legend's visible width is 48 cells, so 32 cells of plain pad.
		" " + legendKey("↑/↓ (j/k)", "move") + legendSep + legendKey("enter", "open") +
			legendSep + legendKey("esc", "back") + legendSep + legendKey("q", "quit") +
			strings.Repeat(" ", 32),
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
		// Rendered on terminal tasks too — one uniform rule — but never
		// a stop there (labels editable, 2026-08-05).
		"  labels      none",
		"  created     2026-07-20 12:00 UTC by brandon",
		"",
		" DESCRIPTION                                                                    ",
		"  none",
		"",
		" HISTORY                                                                        ",
		"  2026-07-25 09:00 UTC  escalation from brandon/a7",
		"    Q: Keep the wiki export?",
		"    unanswered",
	}, "\n") + "\n" +
		// Footer pinned to row 40 (chrome hierarchy, 2026-08-03): rows
		// 18-39 are the blank pad.
		strings.Repeat("\n", 22) +
		" ↑/↓ (j/k) move · enter edit · esc back · q quit                                "
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

// The footer — and a live input prompt riding its slot — pins to the
// bottom row whenever the height is known (chrome hierarchy,
// 2026-08-03): a short body pads with blank lines to a full-height
// frame, on all three screens. Before the first WindowSizeMsg the
// footer floats after the body, as it always did. The invariant is
// bubbletea's, not an abstract one: the renderer splits the view on \n
// and drops overflow from the TOP, so a pinned frame must split into
// exactly height lines with the footer last and unterminated — one
// trailing newline is an extra empty split-line that costs the header
// row (the off-by-one shipped in the first chrome-hierarchy cut).
func TestTopGoldenFooterPinned(t *testing.T) {
	bottom := func(t *testing.T, v string, height int, want string) {
		t.Helper()
		lines := strings.Split(v, "\n")
		if len(lines) != height {
			t.Errorf("frame splits into %d lines, want exactly %d; view:\n%s", len(lines), height, v)
		}
		if last := lines[len(lines)-1]; !strings.Contains(last, want) {
			t.Errorf("bottom row is %q, want it to carry %q", last, want)
		}
	}
	// The list.
	m := newTopModel(newFakeSteering())
	m.width, m.height = 80, 40
	bottom(t, m.View(), 40, "q quit")
	// A live input prompt (priority, opened from a ready row) rides the
	// same pinned slot: the widget's hint line is the bottom row.
	mp, _ := press(t, m, keyOf(tea.KeyDown), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if mp.mode != modePriority {
		t.Fatalf("p on a ready row: mode %d, want modePriority", mp.mode)
	}
	bottom(t, mp.View(), 40, "enter submits · esc cancels")
	// History.
	mh, _ := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	bottom(t, mh.View(), 40, "q quit")
	// The task view, and an edit prompt opened from it.
	md := openDetail(t, m, "t-pars")
	bottom(t, md.View(), 40, "q quit")
	md, _ = press(t, md, keyOf(tea.KeyEnter)) // the focused title stop's editor
	if md.mode != modeEditTitle {
		t.Fatalf("enter on the title stop: mode %d, want modeEditTitle", md.mode)
	}
	bottom(t, md.View(), 40, "enter saves · esc cancels")
	// Full-height content — the terminal shorter than the list, so
	// visibleChunks windows the body — keeps the same invariant: no
	// pad, and still no line lost off the top.
	ms := newTopModel(newFakeSteering())
	ms.width, ms.height = 80, 12
	v := ms.View()
	bottom(t, v, 12, "q quit")
	if !strings.Contains(strings.Split(v, "\n")[0], "tuhdoo") {
		t.Errorf("full window: header missing from the top row; view:\n%s", v)
	}
	// No WindowSizeMsg yet: the footer floats — no blank pad appears.
	mf := newTopModel(newFakeSteering())
	mf.width = 80
	if v := mf.View(); strings.Contains(v, "\n\n\n") {
		t.Errorf("height unknown: footer should float, found a blank pad; view:\n%s", v)
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
