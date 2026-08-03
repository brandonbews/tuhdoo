package main

// Golden tests for the serialized one-shot output (T7, 2026-07-31:
// serialization, not design). Byte-exact, table-style fixtures, pure
// data-in/bytes-out (T1): printBacklog and printEscalations take a
// snapshot and a writer — no TTY probe, no color state — so the bytes
// here are the bytes every consumer gets, terminal or pipe.

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// oneshotSnapshot covers every backlog state and both escalation
// states, including a relayed answer.
func oneshotSnapshot() *snapshot {
	const (
		parser  = "tuh-01K1G0000000000000000PARS"
		floor   = "tuh-01K1G0000000000000000SWEP"
		flake   = "tuh-01K1G0000000000000000FAKE"
		docs    = "tuh-01K1G0000000000000000D0CS"
		license = "tuh-01K1G0000000000000000CENS"
		parked  = "tuh-01K1G0000000000000000PARK"
		idea    = "tuh-01K1G0000000000000000QC01"
		chore   = "t-01K1G00000000000000000CH0R"
		wrong   = "tuh-01K1G0000000000000000WRNG"
	)
	eLic := escalationJSON{
		ID: "01K1G000000000000000000ESCB", Task: license, Actor: "brandon/a2",
		Question: "Which license do we ship under?", Blocking: true,
		RaisedAt: time.Date(2026, 7, 29, 14, 3, 0, 0, time.UTC),
	}
	eUni := escalationJSON{
		ID: "01K1G000000000000000000ESCA", Task: parser, Actor: "brandon/a3",
		Question: "Do we need unicode?", Blocking: false,
		RaisedAt: time.Date(2026, 7, 29, 9, 30, 0, 0, time.UTC),
		Answered: true, Answer: "ASCII first.",
		AnsweredBy: "brandon", RelayedBy: "brandon/a1",
	}
	return &snapshot{
		state: stateResp{Tasks: []stateTask{
			{ID: parser, Title: "write the parser", Status: "open", Priority: 5, Labels: []string{"go", "parser"}, Situation: "ready"},
			{ID: docs, Title: "ship the docs", Status: "open", Situation: "blocked", UnmetDeps: []string{parser}},
			{ID: flake, Title: "investigate the flake", Status: "open", Holder: "brandon/a1", Situation: "in_progress"},
			{ID: license, Title: "choose a license", Status: "open", Situation: "blocked", BlockingEscalations: []string{eLic.ID}},
			{ID: parked, Title: "polish the docs", Status: "held", Priority: 2, Labels: []string{"docs"}, Situation: "held"},
			{ID: idea, Title: "idea: dark mode", Status: "inbox", Situation: "inbox"},
			{ID: chore, Title: "old chore", Status: "done", Situation: "done"},
			{ID: wrong, Title: "wrong idea", Status: "cancelled", Situation: "cancelled"},
			{ID: floor, Title: "sweep the floor", Status: "open", Priority: 1, Situation: "ready"},
		}},
		tasks: map[string]hydratedTask{
			parser:  {Task: taskJSON{ID: parser}, Escalations: []escalationJSON{eUni}},
			docs:    {Task: taskJSON{ID: docs, DependsOn: []string{parser}}},
			flake:   {Task: taskJSON{ID: flake}},
			license: {Task: taskJSON{ID: license}, Escalations: []escalationJSON{eLic}},
			parked:  {Task: taskJSON{ID: parked}},
			idea:    {Task: taskJSON{ID: idea}},
			chore:   {Task: taskJSON{ID: chore}},
			wrong:   {Task: taskJSON{ID: wrong}},
			floor:   {Task: taskJSON{ID: floor}},
		},
	}
}

// The backlog serialization: header row, one aligned record per task,
// STATE column instead of sections, waiting-reasons as dep:/esc: IDs,
// "-" for empty cells, no ANSI ever.
func TestOneshotBacklogGolden(t *testing.T) {
	var buf bytes.Buffer
	printBacklog(&buf, oneshotSnapshot())
	want := strings.Join([]string{
		"ID                             STATE        PRI  HOLDER      LABELS     WAITING                            TITLE",
		"tuh-01K1G0000000000000000PARS  ready        5    -           go,parser  -                                  write the parser",
		"tuh-01K1G0000000000000000SWEP  ready        1    -           -          -                                  sweep the floor",
		"tuh-01K1G0000000000000000FAKE  in-progress  0    brandon/a1  -          -                                  investigate the flake",
		"tuh-01K1G0000000000000000D0CS  blocked      0    -           -          dep:tuh-01K1G0000000000000000PARS  ship the docs",
		"tuh-01K1G0000000000000000CENS  blocked      0    -           -          esc:01K1G000000000000000000ESCB    choose a license",
		"tuh-01K1G0000000000000000PARK  on-hold      2    -           docs       -                                  polish the docs",
		"tuh-01K1G0000000000000000QC01  inbox        0    -           -          -                                  idea: dark mode",
		"t-01K1G00000000000000000CH0R   done         0    -           -          -                                  old chore",
		"tuh-01K1G0000000000000000WRNG  cancelled    0    -           -          -                                  wrong idea",
		"",
	}, "\n")
	got := buf.String()
	if got != want {
		t.Errorf("backlog serialization diverged from golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("serialized backlog contains ANSI escapes:\n%q", got)
	}
	// The STATE column is the grep contract: a state name selects
	// exactly that state's rows.
	for name, n := range map[string]int{
		"ready": 2, "in-progress": 1, "blocked": 2,
		"on-hold": 1, "inbox": 1, "done": 1, "cancelled": 1,
	} {
		var hits int
		for _, l := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
			if strings.Contains(l, name) {
				hits++
			}
		}
		if hits != n {
			t.Errorf("grep %q selects %d lines, want %d:\n%s", name, hits, n, got)
		}
	}
}

// The escalations serialization: open rows before answered ones, the
// blocking flag its own column, relayed attribution preserved.
func TestOneshotEscalationsGolden(t *testing.T) {
	var buf bytes.Buffer
	printEscalations(&buf, oneshotSnapshot())
	want := strings.Join([]string{
		"ID                           STATE     BLOCKING  TASK                           ASKED-BY    RAISED             ANSWERED-BY  RELAYED-BY  QUESTION                         ANSWER",
		"01K1G000000000000000000ESCB  open      blocking  tuh-01K1G0000000000000000CENS  brandon/a2  2026-07-29T14:03Z  -            -           Which license do we ship under?  -",
		"01K1G000000000000000000ESCA  answered  -         tuh-01K1G0000000000000000PARS  brandon/a3  2026-07-29T09:30Z  brandon      brandon/a1  Do we need unicode?              ASCII first.",
		"",
	}, "\n")
	got := buf.String()
	if got != want {
		t.Errorf("escalations serialization diverged from golden.\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("serialized escalations contains ANSI escapes:\n%q", got)
	}
}
