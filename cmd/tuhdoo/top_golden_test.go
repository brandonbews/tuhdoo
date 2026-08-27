package main

// Golden render tests for the dashboard's mock-a layout (task
// t-01KYVJ2607S5S390CVYSF3PVG4): full-width bars, the shared column
// grid, ellipsis rules, and chunk-safe windowing — all with injected
// width/height (T1: deterministic rendering, table-driven).

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// ansiColors is the real escape set of the 16-color floor, for tests
// that pin bar styling.
var ansiColors = colors{
	reset: "\x1b[0m", bold: "\x1b[1m", dim: "\x1b[90m", rev: "\x1b[7m",
	green: "\x1b[32m", yellow: "\x1b[33m", red: "\x1b[31m", magenta: "\x1b[35m",
	brightRed: "\x1b[91m",
	dimRed:    "\x1b[2;31m",
	bgMagenta: "\x1b[30;45m", bgGreen: "\x1b[30;42m",
	bgYellow: "\x1b[30;43m", bgRed: "\x1b[30;101m",
	bgGray: "\x1b[30;47m", bgWhite: "\x1b[30;107m",
	bgDarkGray: "\x1b[100m",
}

// rungColors is the same palette resolved on the 256-color rung: the
// one faint survivor — dimRed — swaps SGR-2, which mosh renders as a
// no-op, for an indexed code, and orange exists. dim is theme-derived
// ANSI 90 on every rung (fourth revision, 2026-08-26) and bgDarkGray
// stays the floor's plain 100 here — runTUI re-resolves it down the
// chromeBG ladder (steering, 2026-08-27).
var rungColors = colors{
	reset: "\x1b[0m", bold: "\x1b[1m", dim: "\x1b[90m", rev: "\x1b[7m",
	green: "\x1b[32m", yellow: "\x1b[33m", red: "\x1b[31m", magenta: "\x1b[35m",
	brightRed: "\x1b[91m",
	dimRed:    "\x1b[38;5;131m",
	bgMagenta: "\x1b[30;45m", bgGreen: "\x1b[30;42m",
	bgYellow: "\x1b[30;43m", bgRed: "\x1b[30;101m",
	bgGray: "\x1b[30;47m", bgWhite: "\x1b[30;107m",
	bgDarkGray: "\x1b[100m",
	orange:     "\x1b[38;5;208m",
}

// noFaint asserts no SGR-2 byte sequence survives anywhere in a rung
// render — the whole point of the ladder: mosh renders faint as a
// no-op, so on the rung "muted" must never be spelled with it.
func noFaint(t *testing.T, surface, v string) {
	t.Helper()
	for _, sgr2 := range []string{"\x1b[2m", "\x1b[2;"} {
		if strings.Contains(v, sgr2) {
			t.Errorf("%s: SGR-2 faint %q leaked onto the 256-color rung; view:\n%q", surface, sgr2, v)
		}
	}
}

// legendKey and legendSep compose the expected bytes of the unfilled
// footer legend (chrome hierarchy, 2026-08-03): bold key token, dim
// label, dim · separators.
func legendKey(key, label string) string {
	return "\x1b[1m" + key + "\x1b[0m\x1b[90m " + label + "\x1b[0m"
}

const legendSep = "\x1b[90m · \x1b[0m"

// padBar pads left..right with spaces to the given rune width — the
// full-width bar shape the golden assertions rebuild.
func padBar(width int, left, right string) string {
	return left + strings.Repeat(" ", width-len([]rune(left))-len([]rune(right))) + right
}

// The seeded fake at 80 columns, plain colors (the NO_COLOR / non-TTY
// degradation): byte-exact geometry, no escapes, no fill.
func TestTopGoldenPlain80(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width, m.height = 80, 40
	want := strings.Join([]string{
		// The header (chrome pass, 2026-08-21): the bold repo-root
		// basename — this fixture's repo is called tuhdoo — dim
		// separator and sync word, and no badge at all: an armed pane
		// acting as its own derived identity is the normal state.
		" tuhdoo · local-only                                                            ",
		"",
		" NEEDS INPUT (1)                                                   enter answer ",
		"▌ t-lic   !   choose a license",
		"▌             question: Which license?",
		"▌             brandon/a2 · 2026-07-29 14:03 UTC",
		"",
		" READY (2)                                                p priority · c cancel ",
		// Two-line rows (grill 2026-08-05): full title, dim meta line —
		// and a bare row (t-flor) stays one line, the "no labels, no
		// edges" signal. P0-highest (2026-08-21): p1 leads p5.
		"  t-flor  p1  sweep the floor",
		"  t-pars  p5  write the parser",
		"              2 deps",
		"",
		" IN PROGRESS (1)                                                                ",
		"  t-flak      investigate the flake",
		"              ← brandon/a1",
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
		// 2026-08-03): rows 25-39 are the blank pad, row 40 the footer.
		strings.Repeat("\n", 15) +
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
// The ready-row priority badge ramp (P0-highest grill, 2026-08-21):
// p0 bright red (contrast ramp, 2026-08-25 — normal red reads
// low-contrast on dark themes; negatives too, the int is unbounded and
// lower is more urgent), p1 orange on the 256-color rung and yellow on
// the 16-color floor (the p1/p2 collision there is the accepted cost
// of not faking orange), p2 yellow, p3+ dim, no badge at all without a
// priority. Since 2026-08-25 (steering) the ramp colors the badge in
// every section — held rows included.
func TestTopGoldenPriorityBadgeRamp(t *testing.T) {
	const orange208 = "\x1b[38;5;208m"
	mk := func(id string, p *int) stateTask {
		return stateTask{ID: id, Title: "task " + id, Status: "open", Priority: p, Situation: "ready"}
	}
	s := &snapshot{state: stateResp{Tasks: []stateTask{
		mk("t-neg1", pint(-1)),
		mk("t-pri0", pint(0)),
		mk("t-pri1", pint(1)),
		mk("t-pri2", pint(2)),
		mk("t-pri3", pint(3)),
		mk("t-bare", nil),
		{ID: "t-parkd", Title: "task t-parkd", Status: "held", Priority: pint(0), Situation: "held"},
	}}}

	col := ansiColors
	col.orange = orange208
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 80, height: 60, col: col}
	v := m.View()
	for _, want := range []string{
		"\x1b[91mp-1\x1b[0m ", // more urgent than p0: bright red like it
		"\x1b[91mp0\x1b[0m ",
		orange208 + "p1\x1b[0m ",
		"\x1b[33mp2\x1b[0m ",
		"\x1b[90mp3\x1b[0m ",
		// Held takes the ramp too (steering, 2026-08-25): a parked p0 is
		// still a p0. (t-parkd renders as its short form t-arkd — prefix
		// plus 4-char tail.)
		"\x1b[90mt-arkd\x1b[0m  \x1b[91mp0\x1b[0m ",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("ramp view missing %q; view:\n%q", want, v)
		}
	}
	// The unprioritized row renders a blank badge cell, never p-anything.
	if !strings.Contains(v, "\x1b[90mt-bare\x1b[0m      \x1b[1mtask t-bare\x1b[0m") {
		t.Errorf("unprioritized row grew a badge; view:\n%q", v)
	}

	// The 16-color floor: no orange available, p1 falls back to yellow;
	// p0 keeps bright red — 91 is in the 16-color palette.
	m.col.orange = ""
	floor := m.View()
	if !strings.Contains(floor, "\x1b[33mp1\x1b[0m ") {
		t.Errorf("floor p1 not yellow; view:\n%q", floor)
	}
	if !strings.Contains(floor, "\x1b[91mp0\x1b[0m ") {
		t.Errorf("floor p0 not bright red; view:\n%q", floor)
	}
	if strings.Contains(floor, "38;5;208") {
		t.Errorf("orange leaked onto the 16-color floor; view:\n%q", floor)
	}
}

// termColors is newColors' TERM resolution (contrast ramp 2026-08-25;
// dim theme-derived and bars darkened 2026-08-26): every 16-color TERM
// gets the floor palette — dim is ANSI 90 there too, theme-derived on
// every rung — while a TERM advertising 256color swaps dimRed's faint
// for an indexed code (mosh renders SGR-2 as a no-op) and earns
// orange, which runTUI used to resolve; bgDarkGray is runTUI's to
// re-resolve (chromeBG), so newColors keeps it at the floor's 100.
// COLORTERM never unlocks the rung — its signature takes only TERM:
// mosh advertises truecolor it won't honor (the 2026-07-31 finding).
func TestTermColorsLadder(t *testing.T) {
	for _, term := range []string{"xterm", "screen", "vt100", ""} {
		if got := termColors(term); got != ansiColors {
			t.Errorf("TERM %q: palette diverged from the 16-color floor:\ngot  %+v\nwant %+v", term, got, ansiColors)
		}
	}
	for _, term := range []string{"xterm-256color", "screen-256color", "tmux-256color"} {
		if got := termColors(term); got != rungColors {
			t.Errorf("TERM %q: palette diverged from the 256-color rung:\ngot  %+v\nwant %+v", term, got, rungColors)
		}
	}
}

// The dashboard on the 256-color rung (contrast ramp + bar recolors
// II, 2026-08-25; dim theme-derived 2026-08-26): the muted-by-design
// row surfaces read muted without SGR-2 — ids, p3+ badges, and meta
// lines take the theme's own bright-black (ANSI 90), the blocked
// row's waiting: lead takes muted red 131 — while the bars carry the
// same black-on-color bytes as the floor (BLOCKED black on bright red,
// ON HOLD black on slot-7 gray) and badges ride the ramp in every
// section. No SGR-2 byte survives anywhere in the frame.
func TestTopGoldenRungMutedList(t *testing.T) {
	s := topSnapshot()
	s.state.Tasks = append(s.state.Tasks,
		stateTask{ID: "t-wait", Title: "build on the idea", Status: "open",
			Priority: pint(1), Situation: "blocked", UnmetDeps: []string{"t-flor"}})
	s.tasks["t-wait"] = hydratedTask{Task: taskJSON{
		ID: "t-wait", Title: "build on the idea",
		Labels: []string{"infra"}, DependsOn: []string{"t-flor"},
	}}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s),
		col: rungColors, width: 120, height: 60}
	v := m.View()
	for _, want := range []string{
		// Ready-row anatomy: gray id, gray p3+ badge, bold title, gray
		// meta line.
		"\x1b[90mt-pars\x1b[0m  \x1b[90mp5\x1b[0m  \x1b[1mwrite the parser\x1b[0m",
		"              \x1b[90m2 deps\x1b[0m",
		// The held row's badge rides the ramp (steering, 2026-08-25):
		// p2 yellow, on the otherwise dim shelf row.
		"\x1b[90mt-park\x1b[0m  \x1b[33mp2\x1b[0m  \x1b[1mpolish the docs\x1b[0m",
		// A blocked row's badge rides it too: p1 orange on the rung.
		"\x1b[90mt-wait\x1b[0m  \x1b[38;5;208mp1\x1b[0m  \x1b[1mbuild on the idea\x1b[0m",
		// Bar recolors II: black on bright red, black on slot-7 gray —
		// in-palette on every rung.
		"\x1b[30;101m" + padBar(120, " BLOCKED (1)", "") + "\x1b[0m",
		"\x1b[30;47m" + padBar(120, " ON HOLD (1)", "c cancel ") + "\x1b[0m",
		// The blocked stack: muted-red lead, gray reason — the loud bar
		// carries the urgency, the reason line never reads as an alarm.
		"              \x1b[38;5;131mwaiting: \x1b[0m\x1b[90mdepends on t-flor (open — sweep the floor)\x1b[0m",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("rung view missing %q; view:\n%q", want, v)
		}
	}
	if strings.Contains(v, "\x1b[31mwaiting:") {
		t.Errorf("blocked waiting: lead is full-brightness red on the rung; view:\n%q", v)
	}
	noFaint(t, "dashboard", v)
}

// The task view on the mosh rung: runTUI resolves bgDarkGray down the
// chromeBG ladder — an unanswered dark 256color TERM lands on the
// neutral bg 238 with no pinned foreground, the theme's own fg riding
// the bar (steering, 2026-08-27) — and the id value and placeholders
// take the theme-derived muted gray. No SGR-2 anywhere.
func TestTopGoldenRungTaskViewBars(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.col = rungColors
	m.col.bgDarkGray = chromeBG(bgAnswer{}, "xterm-256color", true)
	m.width, m.height = 80, 40
	m = openDetail(t, m, "t-pars")
	v := m.View()
	for _, want := range []string{
		"\x1b[48;5;238m" + padBar(80, " DEPENDS ON (2)", "enter open ") + "\x1b[0m",
		"\x1b[48;5;238m" + padBar(80, " DESCRIPTION", "") + "\x1b[0m",
		"\x1b[48;5;238m" + padBar(80, " HISTORY", "") + "\x1b[0m",
		"  \x1b[1mid\x1b[0m          \x1b[90mt-pars\x1b[0m",
		"  \x1b[1mlabels\x1b[0m      \x1b[90mnone\x1b[0m",
		"\n  \x1b[90mnone\x1b[0m", // the empty-description placeholder
	} {
		if !strings.Contains(v, want) {
			t.Errorf("rung task view missing %q; view:\n%q", want, v)
		}
	}
	noFaint(t, "task view", v)
}

// The CLI read commands inherit the ladder through newColors with zero
// call-site churn: on the rung, status' claims line and the task
// view's history stamps carry the same theme-derived ANSI 90, and
// no SGR-2 byte survives in either command's output.
func TestRungCLIReadSurfaces(t *testing.T) {
	s := topSnapshot()
	var b strings.Builder
	printStatus(&b, rungColors, "abc1234", s)
	v := b.String()
	claim := "  \x1b[90mt-flak\x1b[0m  investigate the flake  \x1b[33m← brandon/a1\x1b[0m"
	if !strings.Contains(v, claim) {
		t.Errorf("status claims line not laddered gray; want %q; output:\n%q", claim, v)
	}
	noFaint(t, "printStatus", v)

	b.Reset()
	printTask(&b, rungColors, s.tasks["t-flak"], s.stateTaskOf("t-flak"), s)
	v = b.String()
	stamp := "  \x1b[90m2026-07-29 15:00 UTC\x1b[0m  \x1b[1mnote by brandon/a1\x1b[0m"
	if !strings.Contains(v, stamp) {
		t.Errorf("task history stamp not laddered gray; want %q; output:\n%q", stamp, v)
	}
	noFaint(t, "printTask", v)
}

// Badges ride the ramp in every section (steering, 2026-08-25 —
// reversing the 2026-08-21 no-badge-in-inbox/blocked/history and
// held-dim consequences): an in-progress p0 is bright red beside its
// yellow holder tail, an inbox capture's p2 is yellow, and history
// rows carry their stored priority — a closed p0 stays bright red.
// Blocked and held badges are pinned by the rung golden; unprioritized
// rows everywhere stay bare, pinned by the ramp golden.
func TestTopGoldenBadgesEverySection(t *testing.T) {
	s := topSnapshot()
	for i := range s.state.Tasks {
		switch s.state.Tasks[i].ID {
		case "t-flak":
			s.state.Tasks[i].Priority = pint(0)
		case "t-idea":
			s.state.Tasks[i].Priority = pint(2)
		}
	}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s),
		col: ansiColors, width: 80, height: 60}
	v := m.View()
	for _, want := range []string{
		"\x1b[90mt-flak\x1b[0m  \x1b[91mp0\x1b[0m  \x1b[1minvestigate the flake\x1b[0m",
		"\x1b[90mt-idea\x1b[0m  \x1b[33mp2\x1b[0m  \x1b[1midea: dark mode\x1b[0m",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("dashboard missing ramp badge %q; view:\n%q", want, v)
		}
	}

	// History mode: the close metadata keeps the meta line; the badge
	// keeps the ramp.
	hm := newHistoryModel(newFakeSteering())
	for i := range hm.snap.state.Tasks {
		switch hm.snap.state.Tasks[i].ID {
		case "t-ship":
			hm.snap.state.Tasks[i].Priority = pint(1)
		case "t-drop":
			hm.snap.state.Tasks[i].Priority = pint(0)
		}
	}
	hm.col = ansiColors
	hm.width, hm.height = 80, 40
	hm, _ = press(t, hm, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	hv := hm.View()
	for _, want := range []string{
		"\x1b[90mt-ship\x1b[0m  \x1b[33mp1\x1b[0m  \x1b[1mship the tui\x1b[0m",
		"\x1b[90mt-drop\x1b[0m  \x1b[91mp0\x1b[0m  \x1b[1mdrop the wiki\x1b[0m",
	} {
		if !strings.Contains(hv, want) {
			t.Errorf("history missing ramp badge %q; view:\n%q", want, hv)
		}
	}
}

func TestTopGoldenBars(t *testing.T) {
	for _, width := range []int{80, 120} {
		m := newTopModel(newFakeSteering())
		m.col = ansiColors
		m.width, m.height = width, 60
		v := m.View()
		// The unfilled footer (chrome hierarchy, 2026-08-03): bold keys,
		// dim labels and separators, an unstyled space fill. The done
		// tally survives only where the widened legend leaves room
		// (segLine keeps barLine's drop-the-right-first rule): gone at
		// 80, back at 120. The legend's visible width is 73 cells.
		foot := " " + legendKey("↑/↓ (j/k)", "move") + legendSep + legendKey("enter", "open") +
			legendSep + legendKey("p", "priority") + legendSep + legendKey("c", "cancel") +
			legendSep + legendKey("h", "history") + legendSep + legendKey("q", "quit")
		if width >= 120 {
			foot += strings.Repeat(" ", width-73-7) + "\x1b[90m1 done \x1b[0m"
		} else {
			foot += strings.Repeat(" ", width-73)
		}
		for _, bar := range []string{
			// The unfilled header (chrome hierarchy, 2026-08-03; chrome
			// pass, 2026-08-21): the repo-root basename bold, separator
			// and sync word dim, and no badge — armed as your own derived
			// identity is unmarked, the normal state.
			"\x1b[1m tuhdoo\x1b[0m\x1b[90m · \x1b[0m\x1b[90mlocal-only\x1b[0m" +
				strings.Repeat(" ", width-20),
			"\x1b[30;45m" + padBar(width, " NEEDS INPUT (1)", "enter answer ") + "\x1b[0m",
			"\x1b[30;42m" + padBar(width, " READY (2)", "p priority · c cancel ") + "\x1b[0m",
			"\x1b[30;43m" + padBar(width, " IN PROGRESS (1)", "") + "\x1b[0m",
			// Bar recolors II (2026-08-25): BLOCKED is black on bright
			// red — the p0 badge's background twin — and ON HOLD black on
			// slot-7 gray, so every dashboard bar reads black-on-color;
			// INBOX keeps black on bright-white (bar recolors,
			// 2026-08-04).
			"\x1b[30;101m" + padBar(width, " BLOCKED (0)", "") + "\x1b[0m",
			"\x1b[30;47m" + padBar(width, " ON HOLD (1)", "c cancel ") + "\x1b[0m",
			"\x1b[30;107m" + padBar(width, " INBOX (1)", "i capture · c cancel ") + "\x1b[0m",
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
		// Shelf rows keep the dim id, but titles are bold in every
		// section (2026-07-31) and badges ride the ramp everywhere
		// (steering, 2026-08-25): the held p2 is yellow now.
		for _, row := range []string{
			"\x1b[90mt-park\x1b[0m  \x1b[33mp2\x1b[0m  \x1b[1mpolish the docs\x1b[0m",
			"\x1b[90mt-idea\x1b[0m      \x1b[1midea: dark mode\x1b[0m",
			"\x1b[90mt-pars\x1b[0m  \x1b[90mp5\x1b[0m  \x1b[1mwrite the parser\x1b[0m",
		} {
			if !strings.Contains(v, row) {
				t.Errorf("width %d: row styling wrong %q; view:\n%q", width, row, v)
			}
		}
	}
}

// The blocked row stacks title / meta / waiting (two-line rows,
// 2026-08-05): the waiting: line keeps its own dim-red lead below the
// dim meta line — a reason, not metadata — matching its section bar
// (bar recolors, 2026-08-04): a full-brightness red lead would be
// louder than the bar above it.
func TestTopGoldenBlockedStack(t *testing.T) {
	s := topSnapshot()
	s.state.Tasks = append(s.state.Tasks,
		stateTask{ID: "t-wait", Title: "build on the idea", Status: "open",
			Situation: "blocked", UnmetDeps: []string{"t-flor"}})
	s.tasks["t-wait"] = hydratedTask{Task: taskJSON{
		ID: "t-wait", Title: "build on the idea",
		Labels: []string{"infra"}, DependsOn: []string{"t-flor"},
	}}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s),
		col: ansiColors, width: 120, height: 60}
	v := m.View()
	want := "  \x1b[90mt-wait\x1b[0m      \x1b[1mbuild on the idea\x1b[0m\n" +
		"              \x1b[90m[infra] · 1 dep\x1b[0m\n" +
		"              \x1b[2;31mwaiting: \x1b[0m\x1b[90mdepends on t-flor (open — sweep the floor)\x1b[0m"
	if !strings.Contains(v, want) {
		t.Errorf("blocked row does not stack title / meta / waiting; want:\n%q\nview:\n%q", want, v)
	}
	if strings.Contains(v, "\x1b[31mwaiting:") {
		t.Errorf("blocked waiting: lead still full-brightness red; view:\n%q", v)
	}
}

// The in-progress mode tail (two-line rows, 2026-08-05): "← holder"
// renders yellow at the end of the otherwise dim meta line — the " · "
// joiner before it stays dim — and a holder with no labels or edges
// still earns the meta line, tail alone.
func TestTopGoldenHolderTailYellow(t *testing.T) {
	s := topSnapshot()
	h := s.tasks["t-flak"]
	h.Task.Labels = []string{"ci"}
	s.tasks["t-flak"] = h
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s),
		col: ansiColors, width: 80, height: 40}
	want := "  \x1b[90mt-flak\x1b[0m      \x1b[1minvestigate the flake\x1b[0m\n" +
		"              \x1b[90m[ci] · \x1b[0m\x1b[33m← brandon/a1\x1b[0m"
	if v := m.View(); !strings.Contains(v, want) {
		t.Errorf("holder tail not yellow on the dim meta line; want:\n%q\nview:\n%q", want, v)
	}
	bare := topSnapshot()
	mb := topModel{armed: true, actor: "brandon", snap: bare, rows: buildRows(bare),
		col: ansiColors, width: 80, height: 40}
	if v := mb.View(); !strings.Contains(v, "\n              \x1b[33m← brandon/a1\x1b[0m") {
		t.Errorf("bare in-progress row lost its holder meta line; view:\n%q", v)
	}
}

// An escalation row stays three lines, its meta line extended by the
// same one meta-line rule: [labels] · edges · actor · stamp (two-line
// rows, 2026-08-05) — the metadata sits on line 3 because the question
// outranks it.
func TestTopGoldenEscalationExtendedMeta(t *testing.T) {
	s := topSnapshot()
	h := s.tasks["t-lic"]
	h.Task.Labels = []string{"legal"}
	h.Task.DependsOn = []string{"t-chor"}
	s.tasks["t-lic"] = h
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 80, height: 40}
	want := "▌ t-lic   !   choose a license\n" +
		"▌             question: Which license?\n" +
		"▌             [legal] · 1 dep · brandon/a2 · 2026-07-29 14:03 UTC"
	if v := m.View(); !strings.Contains(v, want) {
		t.Errorf("escalation row's extended meta line diverged; want:\n%q\nview:\n%s", want, v)
	}
}

// Gutter alignment (amendment, 2026-08-05): a list mixing t- and tuh-
// prefixes derives one ID column from the snapshot's widest short ID
// (floor gridIDW), so every title, meta line, and waiting: line sits
// on the same derived column — and the width is snapshot-stable, so
// scrolling cannot change it.
func TestTopGoldenMixedIDColumns(t *testing.T) {
	newID := "tuh-01KYT63MB28Z535SMJC9B0D83W" // tuh-d83w: 8 wide, sets the column
	oldID := "t-01KYT63MB28Z535SMJCA63RQJM"   // t-rqjm: 6 wide, padded to 8
	blkID := "t-01KYT63MB28Z535SMJCBC7SY1P"   // t-sy1p
	s := &snapshot{
		state: stateResp{Tasks: []stateTask{
			{ID: newID, Title: "the new-era task", Status: "open", Priority: pint(2), Situation: "ready"},
			{ID: oldID, Title: "the old-era task", Status: "open", Priority: pint(1), Situation: "ready"},
			{ID: blkID, Title: "the waiting task", Status: "open", Situation: "blocked", UnmetDeps: []string{oldID}},
		}},
		tasks: map[string]hydratedTask{
			newID: {Task: taskJSON{ID: newID, Title: "the new-era task",
				Labels: []string{"era"}, DependsOn: []string{oldID}}},
			oldID: {Task: taskJSON{ID: oldID, Title: "the old-era task", Labels: []string{"infra"}}},
			blkID: {Task: taskJSON{ID: blkID, Title: "the waiting task", DependsOn: []string{oldID}}},
		},
	}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 80, height: 40}
	v := m.View()
	// Titles start at 2+8+2+2+2 = 16 on every row; second lines indent
	// to the same column (the selected row's gutter replaces the first
	// two cells).
	mustContain(t, v,
		// P0-highest (2026-08-21): the p1 row leads ready and carries
		// the selection bar on both of its lines.
		"▌ t-rqjm    p1  the old-era task",
		"▌               [infra]",
		"  tuh-d83w  p2  the new-era task",
		"                [era] · 1 dep",
		"  t-sy1p        the waiting task",
		"                1 dep",
		"                waiting: depends on t-rqjm (open — the old-era task)")
	// The un-derived 6-wide floor column must not appear anywhere.
	if strings.Contains(v, "t-rqjm  p1") {
		t.Errorf("t- row fell back to the 6-wide column; view:\n%s", v)
	}
	// Scrolling cannot change the column: with a window too short for
	// the list, the bottom row renders at the same derived column as it
	// did in the full view.
	m.height = 9 // head(2) + foot(2) + 5 body lines
	for i := 0; i < 4; i++ {
		m, _ = press(t, m, keyOf(tea.KeyDown))
	}
	sv := m.View()
	mustContain(t, sv,
		"▌ t-sy1p        the waiting task",
		"▌               1 dep",
		"▌               waiting: depends on t-rqjm (open — the old-era task)")
	if strings.Contains(sv, "t-sy1p      the") {
		t.Errorf("scrolling narrowed the derived column; view:\n%s", sv)
	}
}

// Every line of the selected chunk — all three of an escalation's, both
// of a two-line task row's — opens with the bg code and the ▌ gutter,
// the bg re-applies after each internal reset, and each line pads to
// the full width — one continuous bar. Unselected rows carry neither
// bg nor gutter.
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
	// A two-line task row (t-pars: title + meta line) carries the bar on
	// both lines — the selection covers all of a row's lines, whatever
	// its height (two-line rows, 2026-08-05). Two steps: the one-line
	// t-flor leads ready under P0-highest.
	m, _ = press(t, m, keyOf(tea.KeyDown), keyOf(tea.KeyDown))
	sel = nil
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(l, "▌") {
			sel = append(sel, l)
		}
	}
	if len(sel) != 2 {
		t.Fatalf("selected two-line task row has %d gutter lines, want 2; view:\n%q", len(sel), m.View())
	}
	for _, l := range sel {
		if !strings.HasPrefix(l, "\x1b[48;5;236m▌ ") {
			t.Errorf("selected line does not open with bg+gutter: %q", l)
		}
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("selected line is %d cells, want the full 80: %q", w, l)
		}
	}
}

// Watch mode: bars carry no steering hints, the header carries the
// dim "watch" badge (chrome pass, 2026-08-21 — was "watch mode"), and
// the footer legend is the disarmed one.
func TestTopGoldenWatchBars(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.armed = false
	m.width, m.height = 80, 40
	v := m.View()
	mustContain(t, v, "watch ", "↑/↓ (j/k) move · enter open · h history · q quit")
	for _, absent := range []string{"enter answer", "p priority", "c cancel", "i capture"} {
		if strings.Contains(v, absent) {
			t.Errorf("watch mode advertises steering key %q; view:\n%s", absent, v)
		}
	}
}

// The header chrome pass (T7, 2026-08-21), pinned state by state. The
// bold name is the repo-root basename — no product name anywhere in
// the frame: you know what you launched; you need to know which ledger
// this is. Healthy sync (a fetch within two T8 fetch intervals) is a
// static dim ⇅ glyph alone — no timestamp, no ticking relative time,
// no spinner: sync is a ~60s background cycle, and motion would claim
// a liveness the screen cannot honestly show. Stale sync adds the
// relative age in yellow; local-only stays a word (remoteless is a
// normal mode — a bare missing glyph would look broken); errors stay
// loud text. Badge only when special: dim "watch" disarmed, "as
// <principal>" at normal weight only when --as overrode the derived
// identity, and nothing at all on an armed pane acting as its own
// derived identity — absence is the normal state (the vim convention).
func TestTopGoldenHeaderChrome(t *testing.T) {
	newModel := func(sync syncJSON) topModel {
		s := topSnapshot()
		s.state.Sync = sync
		return topModel{armed: true, actor: "brandon", repoName: "webapp",
			snap: s, rows: buildRows(s), col: ansiColors, width: 80, height: 40}
	}
	head := func(m topModel) string { return strings.Split(m.View(), "\n")[0] }

	// Healthy: the dim glyph after the bold repo name, nothing else —
	// and no badge: the line is bare fill to the right edge.
	fresh := time.Now().UTC().Format(time.RFC3339)
	m := newModel(syncJSON{Mode: "syncing", Remote: "origin", LastFetch: fresh})
	h := head(m)
	if want := "\x1b[1m webapp\x1b[0m\x1b[90m · \x1b[0m\x1b[90m⇅\x1b[0m"; !strings.HasPrefix(h, want) {
		t.Errorf("healthy header = %q, want it to open with %q", h, want)
	}
	for _, absent := range []string{"last fetch", "syncing", "origin", "as brandon", "watch"} {
		if strings.Contains(h, absent) {
			t.Errorf("healthy header still carries %q: %q", absent, h)
		}
	}
	if v := m.View(); strings.Contains(v, "tuhdoo") {
		t.Errorf("the product name survives somewhere in the frame; view:\n%q", v)
	}

	// Stale: the last fetch fell behind the ~60s cycle, so the glyph
	// grows a relative age, yellow — the one relative time the TUI
	// shows: a live screen redrawing every 2s cannot rot the way
	// written views do (the render.go stamp discipline).
	old := time.Now().Add(-8 * time.Minute).UTC().Format(time.RFC3339)
	h = head(newModel(syncJSON{Mode: "syncing", Remote: "origin", LastFetch: old}))
	if want := "\x1b[33m⇅ 8m\x1b[0m"; !strings.Contains(h, want) {
		t.Errorf("stale header missing %q: %q", want, h)
	}
	// A syncing daemon with no parseable fetch stamp: stale of unknown
	// age, glyph alone.
	h = head(newModel(syncJSON{Mode: "syncing", Remote: "origin"}))
	if want := "\x1b[33m⇅\x1b[0m"; !strings.Contains(h, want) {
		t.Errorf("no-stamp header missing %q: %q", want, h)
	}

	// local-only stays a dim word.
	h = head(newModel(syncJSON{Mode: "local-only"}))
	if want := "\x1b[90m · \x1b[0m\x1b[90mlocal-only\x1b[0m"; !strings.Contains(h, want) {
		t.Errorf("local-only header missing %q: %q", want, h)
	}

	// Errors stay loud text.
	h = head(newModel(syncJSON{Mode: "error", Remote: "origin", LastError: "connection refused"}))
	if want := "\x1b[31msync error (remote \"origin\"): connection refused\x1b[0m"; !strings.Contains(h, want) {
		t.Errorf("error header missing %q: %q", want, h)
	}

	// The special badges: an --as override at normal weight, watch dim.
	ma := newModel(syncJSON{Mode: "local-only"})
	ma.asOverride = true
	if h = head(ma); !strings.HasSuffix(h, " as brandon ") {
		t.Errorf("as-override header = %q, want the normal-weight as badge at the right edge", h)
	}
	mw := newModel(syncJSON{Mode: "local-only"})
	mw.armed, mw.actor = false, ""
	if h = head(mw); !strings.HasSuffix(h, "\x1b[90mwatch \x1b[0m") {
		t.Errorf("watch header = %q, want the dim watch badge at the right edge", h)
	}
}

// The status strip (feedback only, chrome pass 2026-08-21): the line
// below the header carries feedback alone — in-flight markers and
// validation on the quiet chrome bar, errors on the loud bgRed bar,
// full-width so feedback reads as frame, never as content — and a
// success renders nothing at all: the screen updating is the
// confirmation.
func TestTopGoldenStatusStrip(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m.col = ansiColors
	m.width, m.height = 80, 40

	// In-flight: the updating… marker rides the quiet chrome strip
	// until the write's actionMsg lands.
	m, _ = press(t, m,
		keyOf(tea.KeyDown), // t-flor (p1 leads ready)
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m, cmd := press(t, m, append(runes("2"), keyOf(tea.KeyEnter))...)
	v := m.View()
	if want := "\x1b[100m" + padBar(80, " updating…", "") + "\x1b[0m"; !strings.Contains(v, want) {
		t.Errorf("in-flight strip missing %q; view:\n%q", want, v)
	}

	// Success: the strip vanishes and no confirmation replaces it.
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	v = m.View()
	for _, absent := range []string{"updating…", "set t-flor to p2", "\x1b[100m"} {
		if strings.Contains(v, absent) {
			t.Errorf("view after success still carries %q; view:\n%q", absent, v)
		}
	}

	// Validation rides the quiet strip too.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}, keyOf(tea.KeyEnter))
	v = m.View()
	if want := "\x1b[100m" + padBar(80, " title cannot be empty", "") + "\x1b[0m"; !strings.Contains(v, want) {
		t.Errorf("validation strip missing %q; view:\n%q", want, v)
	}

	// Errors take the loud bar.
	fake.err = errors.New("writes rejected (fail-safe read-only)")
	m, _ = press(t, m, keyOf(tea.KeyEsc)) // abandon the capture
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m, cmd = press(t, m, append(runes("9"), keyOf(tea.KeyEnter))...)
	am = cmd().(actionMsg)
	if am.err == nil {
		t.Fatal("expected an action error")
	}
	mm, _ = m.Update(am)
	m = mm.(topModel)
	v = m.View()
	want := "\x1b[30;101m" + padBar(80, " error: writes rejected (fail-safe read-only)", "") + "\x1b[0m"
	if !strings.Contains(v, want) {
		t.Errorf("error strip missing %q; view:\n%q", want, v)
	}
}

// Two-line ellipsis rules (grill 2026-08-05): titles plain-ellipsize
// to the width with no suffix fight — labels live on the meta line and
// never lose to a long title; the meta line ellipsizes independently.
func TestTopGoldenEllipsisAndLabels(t *testing.T) {
	long := strings.Repeat("A", 70)
	near := strings.Repeat("C", 60)
	s := &snapshot{
		state: stateResp{Tasks: []stateTask{
			{ID: "t-lng1", Title: long, Status: "open", Priority: pint(1), Situation: "ready"},
			{ID: "t-nea1", Title: near, Status: "open", Priority: pint(1), Labels: []string{"quality"}, Situation: "ready"},
			{ID: "t-wide", Title: "wide meta", Status: "open", Priority: pint(1), Situation: "ready"},
		}},
		tasks: map[string]hydratedTask{
			"t-lng1": {Task: taskJSON{ID: "t-lng1"}},
			"t-nea1": {Task: taskJSON{ID: "t-nea1", Labels: []string{"quality"}}},
			"t-wide": {Task: taskJSON{ID: "t-wide",
				Labels: []string{strings.Repeat("L", 40), strings.Repeat("M", 40)}}},
		},
	}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 80, height: 40}
	v := m.View()
	// 80 cols - 14 grid = 66 title cells; the meta line gets the same 66.
	mustContain(t, v,
		strings.Repeat("A", 65)+"…",      // title alone: ellipsized at 66
		near+"\n              [quality]", // near-full title whole, labels intact below
		"              ["+strings.Repeat("L", 40)+", "+strings.Repeat("M", 22)+"…") // meta ellipsized at 66
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
		" tuhdoo · local-only                                                            ",
		"",
		"  t-lic — choose a license",
		"",
		"  id          t-lic",
		"  status      open",
		"  priority    none",
		// The labels line always renders — the dim placeholder degrades
		// to plain "none" here (labels editable, 2026-08-05).
		"  labels      none",
		"  created     2026-07-29 12:00 UTC by brandon",
		"",
		// The section bar carries the context-toggle hint alongside the
		// answer key (escalation readability, 2026-08-11).
		" NEEDS INPUT (1)                                       enter answer · e context ",
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
		"  priority    none", // an unfocused stop keeps the blank mark column
		"  labels      none", // the always-rendered labels line, unfocused
		"  none")             // the empty-description placeholder, unfocused
	if n := strings.Count(v, "▌"); n != 1 {
		t.Errorf("%d gutter lines, want exactly 1 (the title); view:\n%s", n, v)
	}
	if strings.Contains(v, "\x1b") {
		t.Errorf("plain render leaked ANSI escapes:\n%q", v)
	}
}

// The task view's edge sections at 80 columns, plain colors (edge
// rows, 2026-08-11): DEPENDS ON renders one row per edge — short ID,
// status word, title on one aligned grid, statuses padded to the
// widest word — under a structural bar carrying the armed "enter open"
// hint; the old depends-on field line is gone from the header block.
// Byte-exact geometry.
func TestTopGoldenTaskViewEdgesPlain80(t *testing.T) {
	s := edgeSnapshot()
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, repoName: "tuhdoo", snap: s, rows: buildRows(s)}
	m.width, m.height = 80, 40
	m = openDetail(t, m, "t-epic")
	want := strings.Join([]string{
		" tuhdoo · local-only                                                            ",
		"",
		"▌ t-epic — ship the epic",
		"",
		"  id          t-epic",
		"  status      open",
		"  priority    none",
		"  labels      none",
		"  created     2026-08-10 12:00 UTC by brandon",
		"",
		padBar(80, " DEPENDS ON (4)", "enter open "),
		"  t-aaaa  done       first child",
		"  t-bbbb  open       second child",
		"  t-cccc  on hold    third child",
		"  t-dddd  cancelled  dropped child",
		"",
		padBar(80, " DESCRIPTION", ""),
		"  none",
		"",
		padBar(80, " HISTORY", ""),
		"  no activity yet",
	}, "\n") + "\n" +
		// Footer pinned to row 40 (chrome hierarchy, 2026-08-03): rows
		// 22-39 are the blank pad.
		strings.Repeat("\n", 18) +
		" ↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit        "
	got := m.View()
	if got != want {
		t.Errorf("plain 80-column edge sections diverged from golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("plain edge-section render leaked ANSI escapes:\n%q", got)
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
	for _, want := range []string{
		"\x1b[30;45m" + padBar(80, " NEEDS INPUT (1)", "enter answer · e context ") + "\x1b[0m",
		// DESCRIPTION and HISTORY are quiet chrome — bgDarkGray, white
		// on bright-black (bar recolors II, 2026-08-25; was
		// reverse-dim): shelf, not queue.
		"\x1b[100m" + padBar(80, " DESCRIPTION", "") + "\x1b[0m",
		"\x1b[100m" + padBar(80, " HISTORY", "") + "\x1b[0m",
		// The unfilled footer: bold keys, dim labels, no fill. The
		// legend's visible width is 72 cells, so 8 cells of plain pad.
		" " + legendKey("↑/↓ (j/k)", "move") + legendSep + legendKey("enter", "edit") +
			legendSep + legendKey("p", "priority") + legendSep + legendKey("c", "cancel") +
			legendSep + legendKey("esc", "back") + legendSep + legendKey("q", "quit") +
			strings.Repeat(" ", 8),
		// Bold field names on the grid; the canonical id value stays dim.
		"  \x1b[1mid\x1b[0m          \x1b[90mt-lic\x1b[0m",
		"  \x1b[1mstatus\x1b[0m      open",
		// The empty labels line: the placeholder is dim, like the empty
		// description body (labels editable, 2026-08-05).
		"  \x1b[1mlabels\x1b[0m      \x1b[90mnone\x1b[0m",
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
	// Watch mode: same view, no steering hint, nothing selected. The
	// context toggle is reading, not steering, so its hint stays — alone
	// (escalation readability, 2026-08-11).
	w := newWatchModel()
	w.col = ansiColors
	w.width, w.height = 80, 40
	w, _ = press(t, w, keyOf(tea.KeyEnter))
	wv := w.View()
	if !strings.Contains(wv, "\x1b[30;45m"+padBar(80, " NEEDS INPUT (1)", "e context ")+"\x1b[0m") {
		t.Errorf("watch task view missing the answer-hint-free NEEDS INPUT bar; view:\n%q", wv)
	}
	if strings.Contains(wv, "enter answer") {
		t.Errorf("watch task view advertises answering; view:\n%q", wv)
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
		" tuhdoo · local-only                                                            ",
		"",
		" DONE (3)                                                                       ",
		// Two-line rows (grill 2026-08-05): the close stamp · closing
		// actor is the mode tail of the dim meta line, off line 1.
		"▌ t-ship      ship the tui",
		"▌             [tui] · 2026-07-30 · brandon/claude-code-1",
		"  t-mgr8      migrate the backlog",
		"              1 dep · 2026-07-29 · brandon",
		"  t-chor      old chore",
		"              2026-07-28 · brandon/impl-1",
		"",
		" CANCELLED (2)                                                                  ",
		"  t-zzzz      zombie idea",
		"              2026-07-31 · brandon/a2",
		"  t-drop      drop the wiki",
		"              2026-07-27 · brandon",
	}, "\n") + "\n" +
		// Footer pinned to row 40 (chrome hierarchy, 2026-08-03): rows
		// 16-39 are the blank pad.
		strings.Repeat("\n", 24) +
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
// and a row's title stays bold while its close metadata rides the dim
// meta line below (two-line rows, 2026-08-05).
func TestTopGoldenHistoryBars(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	m.col = ansiColors
	m.width, m.height = 80, 40
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	v := m.View()
	for _, want := range []string{
		"\x1b[30;42m" + padBar(80, " DONE (3)", "") + "\x1b[0m",
		// CANCELLED is quiet chrome — bgDarkGray, white on bright-black
		// (bar recolors II, 2026-08-25): the brightened bgGray belongs
		// to the dashboard's ON HOLD alone.
		"\x1b[100m" + padBar(80, " CANCELLED (2)", "") + "\x1b[0m",
		// The unfilled footer: bold keys, dim labels, no fill. The
		// legend's visible width is 48 cells, so 32 cells of plain pad.
		" " + legendKey("↑/↓ (j/k)", "move") + legendSep + legendKey("enter", "open") +
			legendSep + legendKey("esc", "back") + legendSep + legendKey("q", "quit") +
			strings.Repeat(" ", 32),
		// The title line ends bold; the close metadata rides the dim
		// meta line below it (two-line rows, 2026-08-05).
		"\x1b[90mt-mgr8\x1b[0m      \x1b[1mmigrate the backlog\x1b[0m\n" +
			"              \x1b[90m1 dep · 2026-07-29 · brandon\x1b[0m",
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
		" tuhdoo · local-only                                                            ",
		"",
		"▌ t-drop — drop the wiki",
		"",
		"  id          t-drop",
		"  status      cancelled — 2026-07-27 by brandon",
		"  priority    none",
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
	want := "  \x1b[90m2026-07-29 15:30 UTC\x1b[0m  \x1b[1mescalation from brandon/a1\x1b[0m\n" +
		"    Q: Skip the flaky test until fixed?\n" +
		"    A (brandon, relayed by brandon/a1): Skip it, link the issue.\n" +
		"\n" +
		"  \x1b[90m2026-07-29 15:00 UTC\x1b[0m  \x1b[1mnote by brandon/a1\x1b[0m\n" +
		"    Repros only under -race.\n" +
		"\n" +
		"  \x1b[90m(unknown time)\x1b[0m  \x1b[1mrun by brandon/a1 — interrupted\x1b[0m\n" +
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
