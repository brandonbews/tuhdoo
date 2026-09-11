package main

// snapshot assembly and classification. The CLI buckets tasks the same
// way internal/views does: ready (priority-ordered) / in progress /
// blocked-with-reason / done / cancelled.

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/views"
)

// readSnapshot is one GET /v0/snapshot?since=N&wait=D, decoded as
// served: the placeholder, the unchanged answer, and the full
// snapshot all come back here for fetchSnapshotSince to sort out.
func readSnapshot(c *client, since uint64, wait time.Duration) (snapshotResp, error) {
	var resp snapshotResp
	err := c.get(fmt.Sprintf("/v0/snapshot?since=%d&wait=%s", since, wait), &resp)
	return resp, err
}

// startupBudget bounds how long a read waits out a daemon's startup
// shapes before reporting them: the placeholder served until the
// first replay lands, and the sync loop's undecided first mode.
const startupBudget = 3 * time.Second

// snapshot is one consistent picture of daemon state at one version:
// the listing (daemon-wide facts and one row per task) and every task
// fully hydrated — one request (T4, 2026-09-10), never a hydration per
// task. version is what the next long poll asks about; unchanged
// marks the wait-elapsed answer, which carries nothing to render.
type snapshot struct {
	version   uint64
	unchanged bool
	state     stateResp
	tasks     map[string]hydratedTask
}

// fetchSnapshot reads the current snapshot at once: the one-shot
// commands' read, and the TUI's on-demand refresh.
func fetchSnapshot(c *client) (*snapshot, error) {
	return fetchSnapshotSince(c, 0, 0)
}

// fetchSnapshotSince reads the snapshot, parking up to wait while the
// daemon is still at version since (the TUI's long poll; wait 0
// answers at once). Two startup shapes are waited out within
// startupBudget: the placeholder a daemon serves until its first
// replay lands (loaded false — T4 startup order; ensureDaemon already
// waited this out once, but a daemon restarting under a long-lived
// screen serves it again) by parking on since=0 through the load, and
// the sync loop's "starting" mode on a freshly loaded daemon whose
// first cycle (milliseconds) has not yet decided local-only vs
// syncing — that decision bumps no version, so it is re-read at once
// on a short cadence. At the budget an undecided sync mode is
// returned as is; a placeholder never is — it is an error, so a
// screen polling through a daemon restart shows "retrying" rather
// than installing an empty board.
func fetchSnapshotSince(c *client, since uint64, wait time.Duration) (*snapshot, error) {
	resp, err := readSnapshot(c, since, wait)
	if err != nil {
		return nil, err
	}
	if resp.Unchanged {
		return &snapshot{version: resp.Version, unchanged: true}, nil
	}
	deadline := time.Now().Add(startupBudget)
	for !resp.Loaded {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, errors.New("daemon is still loading the ledger; try again in a moment")
		}
		if resp, err = readSnapshot(c, 0, remaining); err != nil {
			return nil, err
		}
	}
	for resp.Sync.Mode == "starting" && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		if resp, err = readSnapshot(c, resp.Version, 0); err != nil {
			return nil, err
		}
	}
	return snapshotOf(resp), nil
}

// snapshotOf splits a served snapshot into the listing the render
// code walks and the hydrations it looks up by ID.
func snapshotOf(resp snapshotResp) *snapshot {
	s := &snapshot{
		version: resp.Version,
		state: stateResp{
			Degraded:        resp.Degraded,
			Sync:            resp.Sync,
			Tasks:           make([]stateTask, 0, len(resp.Tasks)),
			OpenEscalations: resp.OpenEscalations,
			Runs:            resp.Runs,
		},
		tasks: make(map[string]hydratedTask, len(resp.Tasks)),
	}
	for _, t := range resp.Tasks {
		s.state.Tasks = append(s.state.Tasks, listingRow(t))
		s.tasks[t.Task.ID] = t.hydratedTask
	}
	return s
}

// listingRow lifts one snapshot entry's listing row: the task's
// headline fields beside the daemon's verdicts.
func listingRow(t snapshotTask) stateTask {
	return stateTask{
		ID: t.Task.ID, Title: t.Task.Title, Status: t.Task.Status,
		Priority: t.Task.Priority, Labels: t.Task.Labels, Holder: t.Holder,
		Situation: t.Situation, UnmetDeps: t.UnmetDeps, BlockingEscalations: t.BlockingEscalations,
		CancelledDeps: t.CancelledDeps, Cyclic: t.Cyclic,
		ClosedAt: t.Task.ClosedAt, ClosedBy: t.Task.ClosedBy,
	}
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
	// Most urgent first (P0-highest, 2026-08-21), creation (ULID) order
	// within a rank — the same ordering core.ReadyTasks serves
	// claim_next from.
	sort.SliceStable(b.ready, func(i, j int) bool {
		return core.MoreUrgent(b.ready[i].Priority, b.ready[j].Priority)
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
// lives in `tuhdoo task <id>`). The loud annotations (2026-08-05 edge
// grill) keep the same register: a leading "cyclic" marker for a task
// on a depends_on loop, a ":cancelled" suffix on a dep sitting
// cancelled. "-" when nothing is waited on. The verdicts are the
// daemon's (one classifier, 2026-08-03) — this only serializes them.
func waitingOn(t stateTask) string {
	var parts []string
	if t.Cyclic {
		parts = append(parts, "cyclic")
	}
	for _, dep := range t.UnmetDeps {
		entry := "dep:" + dep
		if slices.Contains(t.CancelledDeps, dep) {
			entry += ":cancelled"
		}
		parts = append(parts, entry)
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
// escalation alone renders no BLOCKED row at all (see buildRows). Loop
// membership leads the line and a cancelled dep reads "waiting on
// cancelled" (2026-08-05 edge grill) — marks a human must act on,
// distinct from ordinary waiting.
func blockedReasonTUI(t stateTask, disp func(string) string) string {
	var parts []string
	if t.Cyclic {
		parts = append(parts, "cyclic — a human must cut an edge")
	}
	for _, dep := range t.UnmetDeps {
		if slices.Contains(t.CancelledDeps, dep) {
			parts = append(parts, "waiting on cancelled "+disp(dep))
		} else {
			parts = append(parts, "depends on "+disp(dep))
		}
	}
	return strings.Join(parts, "; ")
}

// waitingNote is the task view's loud-annotation line (2026-08-05 edge
// grill), shared by the one-shot task command and the TUI detail: only
// the marks a human must act on — loop membership and cancelled deps —
// never the ordinary unmet-dep list, which the depends-on line already
// carries. "" when there is nothing to shout.
func waitingNote(t stateTask, disp func(string) string) string {
	var parts []string
	if t.Cyclic {
		parts = append(parts, "cyclic — a human must cut an edge")
	}
	for _, dep := range t.CancelledDeps {
		parts = append(parts, "waiting on cancelled "+disp(dep))
	}
	return strings.Join(parts, "; ")
}

// stateTaskOf finds one task's state-listing row; the zero row (no
// annotations, no verdicts) when the ID is unknown.
func (s *snapshot) stateTaskOf(id string) stateTask {
	t, _ := s.findTask(id)
	return t
}

// findTask is stateTaskOf with resolution reported: edge rendering
// must distinguish "unknown to the snapshot" from a zero-value row.
func (s *snapshot) findTask(id string) (stateTask, bool) {
	for _, t := range s.state.Tasks {
		if t.ID == id {
			return t, true
		}
	}
	return stateTask{}, false
}

// dependentsOf lists the tasks whose depends_on names id — the reverse
// edges behind the NEEDED BY sections (edge rows, 2026-08-11),
// computed at read time from the snapshot: no stored reverse index.
// The state listing arrives in creation (ULID) order, so the result is
// ULID-ordered; every dependent is included regardless of status —
// accuracy over noise, the status word carries the story.
func (s *snapshot) dependentsOf(id string) []string {
	var out []string
	for _, t := range s.state.Tasks {
		if slices.Contains(s.tasks[t.ID].Task.DependsOn, id) {
			out = append(out, t.ID)
		}
	}
	return out
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
