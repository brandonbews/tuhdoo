package main

// snapshot assembly and classification. The CLI buckets tasks the same
// way internal/views does: ready (priority-ordered) / in progress /
// blocked-with-reason / done / cancelled.

import (
	"fmt"
	"sort"
	"strings"
	"time"
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
		switch t.Status {
		case "done":
			b.done = append(b.done, t)
		case "cancelled":
			b.cancelled = append(b.cancelled, t)
		case "held":
			b.held = append(b.held, t)
		case "inbox":
			b.inbox = append(b.inbox, t)
		default: // open
			switch {
			case t.Holder != "":
				b.inProgress = append(b.inProgress, t)
			case s.claimable(t.ID):
				b.ready = append(b.ready, t)
			default:
				b.blocked = append(b.blocked, t)
			}
		}
	}
	// Highest priority first, creation (ULID) order within a priority —
	// the same ordering core.ReadyTasks serves claim_next from.
	sort.SliceStable(b.ready, func(i, j int) bool {
		return b.ready[i].Priority > b.ready[j].Priority
	})
	return b
}

// claimable mirrors core.State.Ready for an open, unclaimed task: every
// dependency done and no open blocking escalation.
func (s *snapshot) claimable(id string) bool {
	return !s.hasUnmetDeps(id) && s.blockingEscalation(id) == nil
}

// hasUnmetDeps reports whether the task has any not-yet-done
// dependency.
func (s *snapshot) hasUnmetDeps(id string) bool {
	for _, dep := range s.tasks[id].Task.DependsOn {
		if st, ok := s.statusOf(dep); ok && st != "done" {
			return true
		}
	}
	return false
}

// statusOf looks a task's status up in the state listing. Linear scan:
// boring wins at v0 volumes.
func (s *snapshot) statusOf(id string) (string, bool) {
	for _, t := range s.state.Tasks {
		if t.ID == id {
			return t.Status, true
		}
	}
	return "", false
}

// blockingEscalation returns the earliest open blocking escalation on a
// task, or nil.
func (s *snapshot) blockingEscalation(taskID string) *escalationJSON {
	escs := s.tasks[taskID].Escalations
	for i := range escs {
		if escs[i].Blocking && !escs[i].Answered {
			return &escs[i]
		}
	}
	return nil
}

// blockedReason names why a blocked task cannot be claimed: each unmet
// dependency, and/or an open blocking escalation. One-shot copy: full
// dependency IDs, and the escalation carries its question verbatim —
// one-shot output stands alone, with no Needs Input row to point at.
func (s *snapshot) blockedReason(id string) string {
	parts := s.unmetDeps(id, func(dep string) string { return dep })
	if e := s.blockingEscalation(id); e != nil {
		parts = append(parts, "escalation: "+oneLine(e.Question))
	}
	return strings.Join(parts, "; ")
}

// blockedReasonTUI is the dashboard's blockedReason: dependency IDs
// pass through disp (shortened and annotated for the screen), and only
// unmet deps are named — never the escalation. On this screen the Needs
// Input row is the single home for escalation blockage (grill cycle,
// 2026-07-31), and a task blocked by escalation alone renders no
// BLOCKED row at all (see buildRows).
func (s *snapshot) blockedReasonTUI(id string, disp func(string) string) string {
	return strings.Join(s.unmetDeps(id, disp), "; ")
}

// unmetDeps lists a task's not-yet-done dependencies as "depends on X"
// parts, IDs rendered through disp.
func (s *snapshot) unmetDeps(id string, disp func(string) string) []string {
	var parts []string
	for _, dep := range s.tasks[id].Task.DependsOn {
		if st, ok := s.statusOf(dep); ok && st != "done" {
			parts = append(parts, "depends on "+disp(dep))
		}
	}
	return parts
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
			return fmt.Sprintf("%s (%s — %s)", shortID(id), humanStatus(t.Status), ellipsize(oneLine(t.Title), 40))
		}
	}
	return shortID(id)
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
