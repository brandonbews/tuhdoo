package daemon

// The operations layer: every verb both surfaces expose, as methods on
// *Daemon. The HTTP API (api.go) and the MCP endpoint (mcp.go) are thin
// wrappers around these — T4's "all surfaces are projections of the same
// daemon state" holds because there is exactly one implementation of
// each operation, serialized by the one write mutex.

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
)

// opError is an operation failure with an HTTP-status-shaped code, so
// both surfaces map the same failure the same way: api.go writes the
// code directly, mcp.go turns any opError into a tool error.
type opError struct {
	code int // http.Status* constant
	msg  string
}

func (e *opError) Error() string { return e.msg }

func opErrf(code int, format string, args ...any) *opError {
	return &opError{code: code, msg: fmt.Sprintf(format, args...)}
}

// degradedLocked returns the T3 fail-safe rejection when the daemon is
// in read-only mode, nil otherwise. Caller holds d.mu.
func (d *Daemon) degradedLocked() *opError {
	if d.degraded != nil {
		return opErrf(http.StatusServiceUnavailable, "writes rejected (fail-safe read-only): %v", d.degraded)
	}
	return nil
}

// writeErrLocked maps a failed write/refresh: 503 when the failure put
// us in fail-safe mode, 500 otherwise. Caller holds d.mu.
func (d *Daemon) writeErrLocked(err error) *opError {
	if d.degraded != nil {
		return opErrf(http.StatusServiceUnavailable, "writes rejected (fail-safe read-only): %v", err)
	}
	return opErrf(http.StatusInternalServerError, "%v", err)
}

// ---- request shapes shared by both surfaces ----
//
// The jsonschema descriptions double as the MCP tools' agent-facing
// documentation (T5: the agent protocol is a first-class deliverable).

type createTaskItem struct {
	Tmp         string   `json:"tmp,omitempty" jsonschema:"optional name other items in this batch can reference as 'tmp:<name>' in parents/depends_on; resolved to the real task ID on commit"`
	Title       string   `json:"title" jsonschema:"short imperative summary of the work (required)"`
	Description string   `json:"description,omitempty" jsonschema:"the task body — write it like a prompt: acceptance criteria, constraints, file pointers; output quality of whoever claims this is bounded by what you put here"`
	Priority    int      `json:"priority,omitempty" jsonschema:"higher claims first; 0 is the default"`
	Labels      []string `json:"labels,omitempty" jsonschema:"free-form capability/topic tags, matchable by claim_next"`
	Parents     []string `json:"parents,omitempty" jsonschema:"task IDs (or 'tmp:<name>' batch refs) this task is a child of; epics are just tasks"`
	DependsOn   []string `json:"depends_on,omitempty" jsonschema:"task IDs (or 'tmp:<name>' batch refs) that must be done before this task is claimable"`
}

type updateTaskReq struct {
	Title       *string   `json:"title,omitempty" jsonschema:"new title; omit to leave unchanged"`
	Description *string   `json:"description,omitempty" jsonschema:"new description; omit to leave unchanged"`
	Status      *string   `json:"status,omitempty" jsonschema:"new status: open, done, or cancelled; omit to leave unchanged"`
	Priority    *int      `json:"priority,omitempty" jsonschema:"new priority; omit to leave unchanged"`
	Labels      *[]string `json:"labels,omitempty" jsonschema:"full replacement label list; omit to leave unchanged"`
	Parents     *[]string `json:"parents,omitempty" jsonschema:"full replacement parent-edge list (task IDs); omit to leave unchanged"`
	DependsOn   *[]string `json:"depends_on,omitempty" jsonschema:"full replacement dependency-edge list (task IDs); omit to leave unchanged"`
}

func (r updateTaskReq) empty() bool {
	return r.Title == nil && r.Description == nil && r.Status == nil &&
		r.Priority == nil && r.Labels == nil && r.Parents == nil && r.DependsOn == nil
}

type finishRunReq struct {
	Task    string   `json:"task"`
	Outcome string   `json:"outcome"`
	Branch  string   `json:"branch,omitempty"`
	PR      string   `json:"pr,omitempty"`
	Commits []string `json:"commits,omitempty"`
	Summary string   `json:"summary,omitempty"`
}

type escalateReq struct {
	Task     string `json:"task"`
	Question string `json:"question"`
	Context  string `json:"context,omitempty"`
	Blocking bool   `json:"blocking,omitempty"`
}

// ---- operations ----

// opCreateTasks is batch create with intra-batch tmp refs (T5
// create_task): the whole DAG lands in one store batch or none of it
// does. Validation runs to completion before any event is staged.
func (d *Daemon) opCreateTasks(actor string, items []createTaskItem) (ids []string, tmp map[string]string, oe *opError) {
	if len(items) == 0 {
		return nil, nil, opErrf(http.StatusBadRequest, "empty batch")
	}
	byTmp := make(map[string]int)
	for i, it := range items {
		if it.Title == "" {
			return nil, nil, opErrf(http.StatusBadRequest, "item %d: title is required", i)
		}
		if it.Tmp != "" {
			if _, dup := byTmp[it.Tmp]; dup {
				return nil, nil, opErrf(http.StatusBadRequest, "duplicate tmp name %q", it.Tmp)
			}
			byTmp[it.Tmp] = i
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return nil, nil, oe
	}

	// Resolve every edge ref: "tmp:<name>" must name a batch item, and
	// anything else must be a task that already exists.
	adj := make([][]int, len(items)) // intra-batch edges, for the cycle check
	for i, it := range items {
		for _, ref := range append(append([]string(nil), it.Parents...), it.DependsOn...) {
			if name, isTmp := strings.CutPrefix(ref, "tmp:"); isTmp {
				j, ok := byTmp[name]
				if !ok {
					return nil, nil, opErrf(http.StatusBadRequest, "item %d: unknown tmp ref %q", i, ref)
				}
				adj[i] = append(adj[i], j)
			} else if _, ok := d.state.Tasks[ref]; !ok {
				return nil, nil, opErrf(http.StatusBadRequest, "item %d: unknown task %q", i, ref)
			}
		}
	}
	if hasCycle(adj) {
		return nil, nil, opErrf(http.StatusBadRequest, "cyclic tmp refs in batch")
	}

	ids = make([]string, len(items))
	for i := range items {
		u, err := event.NewID(time.Now(), d.entropy)
		if err != nil {
			return nil, nil, opErrf(http.StatusInternalServerError, "%v", err)
		}
		ids[i] = "t-" + u
	}
	resolve := func(refs []string) []string {
		if len(refs) == 0 {
			return nil
		}
		out := make([]string, len(refs))
		for k, ref := range refs {
			if name, isTmp := strings.CutPrefix(ref, "tmp:"); isTmp {
				out[k] = ids[byTmp[name]]
			} else {
				out[k] = ref
			}
		}
		return out
	}
	evs := make([]event.Event, len(items))
	for i, it := range items {
		ev, err := d.newEventLocked(event.TypeTaskCreated, actor, ids[i], event.TaskCreated{
			Title:       it.Title,
			Description: it.Description,
			Priority:    it.Priority,
			Labels:      it.Labels,
			Parents:     resolve(it.Parents),
			DependsOn:   resolve(it.DependsOn),
		})
		if err != nil {
			return nil, nil, opErrf(http.StatusInternalServerError, "%v", err)
		}
		evs[i] = ev
	}
	if err := d.commitLocked(false, evs...); err != nil {
		return nil, nil, d.writeErrLocked(err)
	}
	tmp = make(map[string]string, len(byTmp))
	for name, i := range byTmp {
		tmp[name] = ids[i]
	}
	return ids, tmp, nil
}

func (d *Daemon) opUpdateTask(actor, id string, req updateTaskReq) (taskJSON, *opError) {
	if req.empty() {
		return taskJSON{}, opErrf(http.StatusBadRequest, "no fields to update")
	}
	if req.Status != nil {
		switch *req.Status {
		case core.StatusOpen, core.StatusDone, core.StatusCancelled:
		default:
			return taskJSON{}, opErrf(http.StatusBadRequest, "invalid status %q: want open, done, or cancelled", *req.Status)
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return taskJSON{}, oe
	}
	if _, ok := d.state.Tasks[id]; !ok {
		return taskJSON{}, opErrf(http.StatusNotFound, "unknown task %s", id)
	}
	for _, refs := range []*[]string{req.Parents, req.DependsOn} {
		if refs == nil {
			continue
		}
		for _, ref := range *refs {
			if _, ok := d.state.Tasks[ref]; !ok {
				return taskJSON{}, opErrf(http.StatusBadRequest, "unknown task %q in edges", ref)
			}
		}
	}
	ev, err := d.newEventLocked(event.TypeTaskUpdated, actor, id, event.TaskUpdated{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		Labels:      req.Labels,
		Parents:     req.Parents,
		DependsOn:   req.DependsOn,
	})
	if err != nil {
		return taskJSON{}, opErrf(http.StatusInternalServerError, "%v", err)
	}
	if err := d.commitLocked(false, ev); err != nil {
		return taskJSON{}, d.writeErrLocked(err)
	}
	return taskJSONOf(d.state.Tasks[id]), nil
}

// opClaimTask claims one specific task (T5 claim_task, the
// human-directed path).
func (d *Daemon) opClaimTask(actor, taskID string) (hydratedTask, *opError) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return hydratedTask{}, oe
	}
	// Claims are the one path where a stale lease verdict changes the
	// outcome, so re-replay at the current instant before deciding.
	if err := d.refreshLocked(now); err != nil {
		return hydratedTask{}, d.writeErrLocked(err)
	}

	t, ok := d.state.Tasks[taskID]
	if !ok {
		return hydratedTask{}, opErrf(http.StatusNotFound, "unknown task %s", taskID)
	}
	if !d.state.Ready(taskID) {
		switch c := d.state.ActiveClaim(taskID); {
		case c != nil:
			return hydratedTask{}, opErrf(http.StatusConflict, "task %s already claimed by %s", taskID, c.Actor)
		case t.Status != core.StatusOpen:
			return hydratedTask{}, opErrf(http.StatusConflict, "task %s is not ready: status is %s", taskID, t.Status)
		default:
			return hydratedTask{}, opErrf(http.StatusConflict, "task %s is not ready: unmet dependencies", taskID)
		}
	}
	return d.claimTargetLocked(actor, t, now)
}

// opClaimNext is the atomic query-and-claim (T5 claim_next): pick the
// best ready task and claim it under the one write mutex. A nil task
// with a nil error means no ready task matched — a normal outcome, not
// a failure; each surface renders it in its own idiom.
func (d *Daemon) opClaimNext(actor string, labels []string) (*hydratedTask, *opError) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return nil, oe
	}
	if err := d.refreshLocked(now); err != nil {
		return nil, d.writeErrLocked(err)
	}

	var target *core.Task
	for _, t := range d.state.ReadyTasks() {
		if hasAllLabels(t, labels) {
			target = t
			break
		}
	}
	if target == nil {
		return nil, nil
	}
	h, oe := d.claimTargetLocked(actor, target, now)
	if oe != nil {
		return nil, oe
	}
	return &h, nil
}

// claimTargetLocked stamps the claim event, flushes it eagerly, writes
// the lease, and hydrates. Caller holds d.mu with freshly replayed
// state and a Ready target.
func (d *Daemon) claimTargetLocked(actor string, target *core.Task, now time.Time) (hydratedTask, *opError) {
	ev, err := d.newEventLocked(event.TypeClaimMade, actor, target.ID, event.ClaimMade{})
	if err != nil {
		return hydratedTask{}, opErrf(http.StatusInternalServerError, "%v", err)
	}
	// Eager (T8): claims race across machines, so the commit must not
	// sit in the debounce window. The lease lands in its own commit; if
	// it failed to land, replay would treat the claim as expired and
	// return the task to the pool — self-healing, not corrupting.
	d.stageLocked(ev)
	if err := d.batcher.Flush(); err != nil {
		return hydratedTask{}, d.writeErrLocked(err)
	}
	if err := d.store.WriteLease(ev.ID, now.Add(d.leaseTTL)); err != nil {
		return hydratedTask{}, opErrf(http.StatusInternalServerError, "write lease: %v", err)
	}
	if err := d.refreshLocked(now); err != nil {
		return hydratedTask{}, d.writeErrLocked(err)
	}
	return d.hydrateLocked(target.ID), nil
}

func (d *Daemon) opRenewClaim(actor, taskID string) (claimID string, expires time.Time, oe *opError) {
	if taskID == "" {
		return "", time.Time{}, opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", time.Time{}, oe
	}
	if err := d.refreshLocked(now); err != nil {
		return "", time.Time{}, d.writeErrLocked(err)
	}
	c, oe := d.holderClaimLocked(taskID, actor)
	if oe != nil {
		return "", time.Time{}, oe
	}
	expires = now.Add(d.leaseTTL)
	if err := d.store.WriteLease(c.ID, expires); err != nil {
		return "", time.Time{}, opErrf(http.StatusInternalServerError, "write lease: %v", err)
	}
	if err := d.refreshLocked(now); err != nil {
		return "", time.Time{}, d.writeErrLocked(err)
	}
	return c.ID, expires, nil
}

func (d *Daemon) opReleaseClaim(actor, taskID, reason string) (claimID string, oe *opError) {
	if taskID == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	if reason == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "reason")
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", oe
	}
	if err := d.refreshLocked(now); err != nil {
		return "", d.writeErrLocked(err)
	}
	c, oe := d.holderClaimLocked(taskID, actor)
	if oe != nil {
		return "", oe
	}
	ev, err := d.newEventLocked(event.TypeClaimReleased, actor, taskID, event.ClaimReleased{Reason: reason})
	if err != nil {
		return "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	// Eager: a release returns the task to the pool, and peers racing
	// to claim should see that as fast as a claim itself (T8).
	d.stageLocked(ev)
	if err := d.batcher.Flush(); err != nil {
		return "", d.writeErrLocked(err)
	}
	if err := d.store.DeleteLease(c.ID); err != nil {
		return "", opErrf(http.StatusInternalServerError, "delete lease: %v", err)
	}
	if err := d.refreshLocked(now); err != nil {
		return "", d.writeErrLocked(err)
	}
	return c.ID, nil
}

// opFinishRun records a run. It accepts every catalog outcome — the
// MCP layer narrows to the agent-reported set before calling (T5:
// interrupted and superseded are daemon-only verdicts).
func (d *Daemon) opFinishRun(actor string, req finishRunReq) (string, *opError) {
	if req.Task == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	switch req.Outcome {
	case event.OutcomeDone, event.OutcomeFailed, event.OutcomeAbandoned,
		event.OutcomeBlocked, event.OutcomeInterrupted, event.OutcomeSuperseded:
	default:
		return "", opErrf(http.StatusBadRequest, "invalid outcome %q", req.Outcome)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", oe
	}
	if _, ok := d.state.Tasks[req.Task]; !ok {
		return "", opErrf(http.StatusNotFound, "unknown task %s", req.Task)
	}
	ev, err := d.newEventLocked(event.TypeRunFinished, actor, req.Task, event.RunFinished{
		Outcome: req.Outcome,
		Branch:  req.Branch,
		PR:      req.PR,
		Commits: req.Commits,
		Summary: req.Summary,
	})
	if err != nil {
		return "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	if err := d.commitLocked(false, ev); err != nil {
		return "", d.writeErrLocked(err)
	}
	return ev.ID, nil
}

func (d *Daemon) opEscalate(actor string, req escalateReq) (string, *opError) {
	if req.Task == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	if req.Question == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "question")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", oe
	}
	if _, ok := d.state.Tasks[req.Task]; !ok {
		return "", opErrf(http.StatusNotFound, "unknown task %s", req.Task)
	}
	ev, err := d.newEventLocked(event.TypeEscalationRaised, actor, req.Task, event.EscalationRaised{
		Question: req.Question,
		Context:  req.Context,
		Blocking: req.Blocking,
	})
	if err != nil {
		return "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	// Eager (T8): a human is waiting on this.
	if err := d.commitLocked(true, ev); err != nil {
		return "", d.writeErrLocked(err)
	}
	return ev.ID, nil
}

// opAnswerEscalation is the steering-surface path (TUI/CLI over HTTP):
// the actor is the answerer, and answering again amends — last answer
// wins in replay. It lives here so api.go stays a uniform thin layer.
func (d *Daemon) opAnswerEscalation(actor, escalation, answer string) (string, *opError) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.answerLocked(actor, actor, escalation, answer, false)
}

// opRelayAnswer is the agent path (T5 relay_answer, 2026-07-30
// revision): the agent records an answer given out of band, attributed
// to the session principal's root — derived here, never agent-supplied,
// so a session can only attribute an answer to its own root. Open
// escalations only: amending a settled answer stays steering work.
func (d *Daemon) opRelayAnswer(actor, escalation, answer string) (string, *opError) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.answerLocked(actor, rootPrincipal(actor), escalation, answer, true)
}

// answerLocked writes one escalation.answered event: actor on the
// envelope, answeredBy in the payload. Callers hold d.mu.
func (d *Daemon) answerLocked(actor, answeredBy, escalation, answer string, openOnly bool) (string, *opError) {
	if escalation == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "escalation")
	}
	if answer == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "answer")
	}
	if oe := d.degradedLocked(); oe != nil {
		return "", oe
	}
	esc, ok := d.state.Escalations[escalation]
	if !ok {
		return "", opErrf(http.StatusNotFound, "unknown escalation %s", escalation)
	}
	if openOnly && esc.Answered {
		return "", opErrf(http.StatusConflict,
			"escalation %s is already answered by %s; amending an answer is steering work (TUI/CLI)",
			escalation, esc.AnsweredBy)
	}
	ev, err := d.newEventLocked(event.TypeEscalationAnswered, actor, esc.Task, event.EscalationAnswered{
		Answer:     answer,
		AnsweredBy: answeredBy,
		Escalation: escalation,
	})
	if err != nil {
		return "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	// Eager: a blocked agent may be polling for exactly this answer.
	if err := d.commitLocked(true, ev); err != nil {
		return "", d.writeErrLocked(err)
	}
	return ev.ID, nil
}

// rootPrincipal is the human half of a D7 principal: "brandon/impl-2"
// roots to "brandon"; a root principal roots to itself.
func rootPrincipal(actor string) string {
	root, _, _ := strings.Cut(actor, "/")
	return root
}

func (d *Daemon) opAddNote(actor, task, text string) (string, *opError) {
	if task == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	if text == "" {
		return "", opErrf(http.StatusBadRequest, "%q is required", "text")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", oe
	}
	if _, ok := d.state.Tasks[task]; !ok {
		return "", opErrf(http.StatusNotFound, "unknown task %s", task)
	}
	ev, err := d.newEventLocked(event.TypeNoteAdded, actor, task, event.NoteAdded{Text: text})
	if err != nil {
		return "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	if err := d.commitLocked(false, ev); err != nil {
		return "", d.writeErrLocked(err)
	}
	return ev.ID, nil
}

// opGetTask hydrates one task (T5 get_task: start work in one call).
func (d *Daemon) opGetTask(id string) (hydratedTask, *opError) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.state.Tasks[id]; !ok {
		return hydratedTask{}, opErrf(http.StatusNotFound, "unknown task %s", id)
	}
	return d.hydrateLocked(id), nil
}

// opBacklog lists claimable tasks, highest priority first (T5
// get_backlog: ready-filtered and dependency-aware, not a raw dump).
// Lease verdicts move with the clock, so replay at the current instant
// first — a stale expiry must not hide a ready task.
func (d *Daemon) opBacklog() ([]taskJSON, *opError) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.degraded == nil {
		if err := d.refreshLocked(now); err != nil {
			return nil, d.writeErrLocked(err)
		}
	}
	ready := []taskJSON{}
	for _, t := range d.state.ReadyTasks() {
		ready = append(ready, taskJSONOf(t))
	}
	return ready, nil
}

// holderClaimLocked resolves the active claim on taskID and enforces
// that actor holds it. Caller holds d.mu with freshly replayed state.
func (d *Daemon) holderClaimLocked(taskID, actor string) (*core.Claim, *opError) {
	if _, ok := d.state.Tasks[taskID]; !ok {
		return nil, opErrf(http.StatusNotFound, "unknown task %s", taskID)
	}
	c := d.state.ActiveClaim(taskID)
	if c == nil {
		return nil, opErrf(http.StatusConflict, "no active claim on task %s", taskID)
	}
	if c.Actor != actor {
		return nil, opErrf(http.StatusForbidden, "claim on task %s is held by %s, not %s", taskID, c.Actor, actor)
	}
	return c, nil
}

// hydrateLocked builds the full picture of one task. Caller holds d.mu
// and has checked the task exists.
func (d *Daemon) hydrateLocked(id string) hydratedTask {
	h := hydratedTask{
		Task:        taskJSONOf(d.state.Tasks[id]),
		Notes:       []noteJSON{},
		Runs:        []runJSON{},
		Escalations: []escalationJSON{},
	}
	if c := d.state.ActiveClaim(id); c != nil {
		cj := claimJSONOf(c)
		if exp, ok := d.leases[c.ID]; ok {
			e := exp
			cj.Expires = &e
		}
		h.Claim = &cj
	}
	for _, n := range d.state.Notes {
		if n.Task == id {
			h.Notes = append(h.Notes, noteJSON{ID: n.ID, Task: n.Task, Actor: n.Actor, Text: n.Text, AddedAt: n.AddedAt})
		}
	}
	for i := range d.state.Runs {
		if d.state.Runs[i].Task == id {
			h.Runs = append(h.Runs, runJSONOf(&d.state.Runs[i]))
		}
	}
	for _, eid := range d.state.EscOrder {
		if e := d.state.Escalations[eid]; e.Task == id {
			h.Escalations = append(h.Escalations, escalationJSONOf(e))
		}
	}
	return h
}

// hasCycle reports whether the intra-batch edge graph has a cycle
// (three-color depth-first search).
func hasCycle(adj [][]int) bool {
	const white, grey, black = 0, 1, 2
	color := make([]int, len(adj))
	var visit func(int) bool
	visit = func(i int) bool {
		color[i] = grey
		for _, j := range adj[i] {
			if color[j] == grey {
				return true
			}
			if color[j] == white && visit(j) {
				return true
			}
		}
		color[i] = black
		return false
	}
	for i := range adj {
		if color[i] == white && visit(i) {
			return true
		}
	}
	return false
}

// hasAllLabels reports whether t carries every requested label.
func hasAllLabels(t *core.Task, want []string) bool {
	for _, w := range want {
		found := false
		for _, l := range t.Labels {
			if l == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
