package main

// snapshot assembly and classification. The CLI buckets tasks the same
// way internal/views does: ready (priority-ordered) / in progress /
// blocked-with-reason / done / cancelled.

import (
	"fmt"
	"github.com/brandonbews/tuhdoo/internal/event"
	"sort"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/views"
)

// fetchState reads /v0/state, briefly waiting out the sync loop's
// "starting" mode: a freshly auto-spawned daemon can answer before its
// first sync cycle (milliseconds) has decided local-only vs syncing.
func fetchState(c *client) (stateResp, error) {
	var st stateResp
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := c.get("/v0/state", &st); err != nil {
			return st, err
		}
		if st.Sync.Mode != "starting" || time.Now().After(deadline) {
			return st, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// snapshot is one consistent-enough picture of daemon state: /v0/state
// plus a hydration of every task. /v0/state alone cannot say *why* a
// task is blocked (it carries no dependency edges) nor show answered
// escalations, so we hydrate; N+1 reads over a local unix socket are
// cheap at v0 volumes.
type snapshot struct {
	state stateResp
	tasks map[string]hydratedTask
}

func fetchSnapshot(c *client) (*snapshot, error) {
	st, err := fetchState(c)
	if err != nil {
		return nil, err
	}
	s := &snapshot{state: st, tasks: make(map[string]hydratedTask, len(st.Tasks))}
	for _, t := range st.Tasks {
		var h hydratedTask
		if err := c.get("/v0/tasks/"+t.ID, &h); err != nil {
			return nil, err
		}
		s.tasks[t.ID] = h
	}
	return s, nil
}

// buckets partitions tasks exactly like internal/views' classify: every
// open task lands in exactly one of ready / inProgress / blocked, and
// held/inbox tasks (2026-07-31) shelve separately — parked and captured
// work, never claimable.
type buckets struct {
	ready      []stateTask // claimable now, highest priority first
	inProgress []stateTask // actively claimed, creation order
	blocked    []stateTask // open but not claimable, creation order
	held       []stateTask // triaged, deliberately paused; creation order
	inbox      []stateTask // untriaged captures; creation order
	done       []stateTask
	cancelled  []stateTask
}

func (s *snapshot) classify() buckets {
	var b buckets
	for _, t := range s.state.Tasks {
		switch t.Situation {
		case "ready":
			b.ready = append(b.ready, t)
		case "in_progress":
			b.inProgress = append(b.inProgress, t)
		case "blocked":
			b.blocked = append(b.blocked, t)
		case "done":
			b.done = append(b.done, t)
		case "cancelled":
			b.cancelled = append(b.cancelled, t)
		case "held":
			b.held = append(b.held, t)
		case "inbox":
			b.inbox = append(b.inbox, t)
		}
	}
	// Highest priority first, creation (ULID) order within a priority —
	// the same ordering core.ReadyTasks serves claim_next from.
	sort.SliceStable(b.ready, func(i, j int) bool {
		return b.ready[i].Priority > b.ready[j].Priority
	})
	return b
}

// terminalStatus reports whether a status closes a task (D5: done and
// cancelled end work; nothing ever deletes it).
func terminalStatus(status string) bool {
	return status == "done" || status == "cancelled"
}

// waitingOn condenses why a blocked task cannot be claimed into one
// column cell: dep:<task-id> per unmet dependency, esc:<escalation-id>
// per open blocking escalation, comma-joined — IDs, never prose
// (T7, 2026-07-31: the serialized backlog is grep fodder; the story
// lives in `tuhdoo task <id>`). "-" when nothing is waited on. The
// lists are the daemon's verdict (one classifier, 2026-08-03) — this
// only serializes them.
func waitingOn(t stateTask) string {
	var parts []string
	for _, dep := range t.UnmetDeps {
		parts = append(parts, "dep:"+dep)
	}
	for _, esc := range t.BlockingEscalations {
		parts = append(parts, "esc:"+esc)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

// blockedReasonTUI names why a blocked task cannot be claimed, for the
// dashboard: dependency IDs pass through disp (shortened and annotated
// for the screen), and only unmet deps are named — never the
// escalation. On this screen the Needs Input row is the single home for
// escalation blockage (grill cycle, 2026-07-31), and a task blocked by
// escalation alone renders no BLOCKED row at all (see buildRows).
func blockedReasonTUI(t stateTask, disp func(string) string) string {
	parts := make([]string, 0, len(t.UnmetDeps))
	for _, dep := range t.UnmetDeps {
		parts = append(parts, "depends on "+disp(dep))
	}
	return strings.Join(parts, "; ")
}

// taskRef renders one task reference for TUI display: the short form,
// annotated with the human-facing status and title when the ID
// resolves in the snapshot. The state listing carries every task
// including done and cancelled ones — they render no rows, so the
// annotation is what proves an edge pointing at them isn't dangling.
// Unresolvable IDs render bare — never invent status.
func (s *snapshot) taskRef(id string) string {
	for _, t := range s.state.Tasks {
		if t.ID == id {
			return fmt.Sprintf("%s (%s — %s)", event.ShortID(id), views.HumanStatus(t.Status), ellipsize(oneLine(t.Title), 40))
		}
	}
	return event.ShortID(id)
}

// allEscalations returns every escalation across all tasks in raise
// (ULID) order.
func (s *snapshot) allEscalations() []escalationJSON {
	var all []escalationJSON
	for _, t := range s.state.Tasks {
		all = append(all, s.tasks[t.ID].Escalations...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all
}
