package daemon

// The JSON HTTP API (002 T4): boring REST over the unix socket. The
// same verbs the MCP surface (B9) will project; both are views over the
// one serialized daemon state. Writers identify themselves with the
// X-Tuhdoo-Actor header (D7 principal: "human" or "human/agent").

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
)

func (d *Daemon) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v0/tasks", d.handleCreateTasks)
	mux.HandleFunc("GET /v0/tasks/{id}", d.handleGetTask)
	mux.HandleFunc("PATCH /v0/tasks/{id}", d.handleUpdateTask)
	mux.HandleFunc("POST /v0/claims", d.handleClaim)
	mux.HandleFunc("POST /v0/claims/renew", d.handleRenewClaim)
	mux.HandleFunc("DELETE /v0/claims", d.handleReleaseClaim)
	mux.HandleFunc("POST /v0/runs", d.handleFinishRun)
	mux.HandleFunc("POST /v0/escalations", d.handleEscalate)
	mux.HandleFunc("POST /v0/escalations/answer", d.handleAnswerEscalation)
	mux.HandleFunc("POST /v0/notes", d.handleAddNote)
	mux.HandleFunc("GET /v0/state", d.handleState)
	return mux
}

// ---- response shapes ----

type taskJSON struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Labels      []string  `json:"labels"`
	Parents     []string  `json:"parents"`
	DependsOn   []string  `json:"depends_on"`
	Status      string    `json:"status"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type claimJSON struct {
	ID      string     `json:"id"`
	Task    string     `json:"task"`
	Actor   string     `json:"actor"`
	Machine string     `json:"machine"`
	MadeAt  time.Time  `json:"made_at"`
	Expires *time.Time `json:"expires"` // lease expiry; null if no lease on record
}

type runJSON struct {
	ID          string   `json:"id"`
	Task        string   `json:"task"`
	Claim       string   `json:"claim"`
	Actor       string   `json:"actor"`
	Machine     string   `json:"machine"`
	Outcome     string   `json:"outcome"`
	Branch      string   `json:"branch"`
	PR          string   `json:"pr"`
	Commits     []string `json:"commits"`
	Summary     string   `json:"summary"`
	Synthesized bool     `json:"synthesized"`
}

type escalationJSON struct {
	ID         string    `json:"id"`
	Task       string    `json:"task"`
	Actor      string    `json:"actor"`
	Question   string    `json:"question"`
	Context    string    `json:"context"`
	Blocking   bool      `json:"blocking"`
	RaisedAt   time.Time `json:"raised_at"`
	Answered   bool      `json:"answered"`
	Answer     string    `json:"answer"`
	AnsweredBy string    `json:"answered_by"`
}

type noteJSON struct {
	ID      string    `json:"id"`
	Task    string    `json:"task"`
	Actor   string    `json:"actor"`
	Text    string    `json:"text"`
	AddedAt time.Time `json:"added_at"`
}

// hydratedTask is one task with everything attached (T5 get_task:
// start work in one call).
type hydratedTask struct {
	Task        taskJSON         `json:"task"`
	Claim       *claimJSON       `json:"claim"`
	Notes       []noteJSON       `json:"notes"`
	Runs        []runJSON        `json:"runs"`
	Escalations []escalationJSON `json:"escalations"`
}

// ---- handlers ----

type createTaskItem struct {
	Tmp         string   `json:"tmp"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	Parents     []string `json:"parents"`
	DependsOn   []string `json:"depends_on"`
}

// handleCreateTasks is batch create with intra-batch tmp refs (T5
// create_task): the whole DAG lands in one store batch or none of it
// does. Validation runs to completion before any event is staged.
func (d *Daemon) handleCreateTasks(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var items []createTaskItem
	if !decodeJSON(w, r, &items) {
		return
	}
	if len(items) == 0 {
		httpError(w, http.StatusBadRequest, "empty batch")
		return
	}
	byTmp := make(map[string]int)
	for i, it := range items {
		if it.Title == "" {
			httpError(w, http.StatusBadRequest, "item %d: title is required", i)
			return
		}
		if it.Tmp != "" {
			if _, dup := byTmp[it.Tmp]; dup {
				httpError(w, http.StatusBadRequest, "duplicate tmp name %q", it.Tmp)
				return
			}
			byTmp[it.Tmp] = i
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}

	// Resolve every edge ref: "tmp:<name>" must name a batch item, and
	// anything else must be a task that already exists.
	adj := make([][]int, len(items)) // intra-batch edges, for the cycle check
	for i, it := range items {
		for _, ref := range append(append([]string(nil), it.Parents...), it.DependsOn...) {
			if name, isTmp := strings.CutPrefix(ref, "tmp:"); isTmp {
				j, ok := byTmp[name]
				if !ok {
					httpError(w, http.StatusBadRequest, "item %d: unknown tmp ref %q", i, ref)
					return
				}
				adj[i] = append(adj[i], j)
			} else if _, ok := d.state.Tasks[ref]; !ok {
				httpError(w, http.StatusBadRequest, "item %d: unknown task %q", i, ref)
				return
			}
		}
	}
	if hasCycle(adj) {
		httpError(w, http.StatusBadRequest, "cyclic tmp refs in batch")
		return
	}

	ids := make([]string, len(items))
	for i := range items {
		u, err := event.NewID(time.Now(), d.entropy)
		if err != nil {
			httpError(w, http.StatusInternalServerError, "%v", err)
			return
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
			httpError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		evs[i] = ev
	}
	if err := d.commitLocked(false, evs...); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	tmp := make(map[string]string, len(byTmp))
	for name, i := range byTmp {
		tmp[name] = ids[i]
	}
	writeJSON(w, http.StatusOK, map[string]any{"ids": ids, "tmp": tmp})
}

type updateTaskReq struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Status      *string   `json:"status"`
	Priority    *int      `json:"priority"`
	Labels      *[]string `json:"labels"`
	Parents     *[]string `json:"parents"`
	DependsOn   *[]string `json:"depends_on"`
}

func (d *Daemon) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	var req updateTaskReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Title == nil && req.Description == nil && req.Status == nil &&
		req.Priority == nil && req.Labels == nil && req.Parents == nil && req.DependsOn == nil {
		httpError(w, http.StatusBadRequest, "no fields to update")
		return
	}
	if req.Status != nil {
		switch *req.Status {
		case core.StatusOpen, core.StatusDone, core.StatusCancelled:
		default:
			httpError(w, http.StatusBadRequest, "invalid status %q: want open, done, or cancelled", *req.Status)
			return
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	if _, ok := d.state.Tasks[id]; !ok {
		httpError(w, http.StatusNotFound, "unknown task %s", id)
		return
	}
	for _, refs := range []*[]string{req.Parents, req.DependsOn} {
		if refs == nil {
			continue
		}
		for _, ref := range *refs {
			if _, ok := d.state.Tasks[ref]; !ok {
				httpError(w, http.StatusBadRequest, "unknown task %q in edges", ref)
				return
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
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err := d.commitLocked(false, ev); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, taskJSONOf(d.state.Tasks[id]))
}

type claimReq struct {
	Task   string   `json:"task"`
	Next   bool     `json:"next"`
	Labels []string `json:"labels"`
}

// handleClaim is claim_task ({task}) or claim_next ({next, labels}) —
// the atomic query-and-claim (T5): pick, claim, lease, and hydrate all
// under the one write mutex.
func (d *Daemon) handleClaim(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req claimReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if (req.Task != "") == req.Next {
		httpError(w, http.StatusBadRequest, `body must set exactly one of "task" or "next"`)
		return
	}

	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	// Claims are the one path where a stale lease verdict changes the
	// outcome, so re-replay at the current instant before deciding.
	if err := d.refreshLocked(now); err != nil {
		d.replyWriteErr(w, err)
		return
	}

	var target *core.Task
	if req.Task != "" {
		t, ok := d.state.Tasks[req.Task]
		if !ok {
			httpError(w, http.StatusNotFound, "unknown task %s", req.Task)
			return
		}
		if !d.state.Ready(req.Task) {
			switch c := d.state.ActiveClaim(req.Task); {
			case c != nil:
				httpError(w, http.StatusConflict, "task %s already claimed by %s", req.Task, c.Actor)
			case t.Status != core.StatusOpen:
				httpError(w, http.StatusConflict, "task %s is not ready: status is %s", req.Task, t.Status)
			default:
				httpError(w, http.StatusConflict, "task %s is not ready: unmet dependencies", req.Task)
			}
			return
		}
		target = t
	} else {
		for _, t := range d.state.ReadyTasks() {
			if hasAllLabels(t, req.Labels) {
				target = t
				break
			}
		}
		if target == nil {
			httpError(w, http.StatusConflict, "no ready task matches")
			return
		}
	}

	ev, err := d.newEventLocked(event.TypeClaimMade, actor, target.ID, event.ClaimMade{})
	if err != nil {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// Eager (T8): claims race across machines, so the commit must not
	// sit in the debounce window. The lease lands in its own commit; if
	// it failed to land, replay would treat the claim as expired and
	// return the task to the pool — self-healing, not corrupting.
	d.stageLocked(ev)
	if err := d.batcher.Flush(); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	if err := d.store.WriteLease(ev.ID, now.Add(d.leaseTTL)); err != nil {
		httpError(w, http.StatusInternalServerError, "write lease: %v", err)
		return
	}
	if err := d.refreshLocked(now); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d.hydrateLocked(target.ID))
}

func (d *Daemon) handleRenewClaim(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Task string `json:"task"`
	}
	if !decodeJSON(w, r, &req) || !requireField(w, "task", req.Task) {
		return
	}

	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	if err := d.refreshLocked(now); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	c, ok := d.holderClaimLocked(w, req.Task, actor)
	if !ok {
		return
	}
	expires := now.Add(d.leaseTTL)
	if err := d.store.WriteLease(c.ID, expires); err != nil {
		httpError(w, http.StatusInternalServerError, "write lease: %v", err)
		return
	}
	if err := d.refreshLocked(now); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"claim":   c.ID,
		"expires": expires.UTC().Truncate(time.Second),
	})
}

func (d *Daemon) handleReleaseClaim(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Task   string `json:"task"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) || !requireField(w, "task", req.Task) || !requireField(w, "reason", req.Reason) {
		return
	}

	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	if err := d.refreshLocked(now); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	c, ok := d.holderClaimLocked(w, req.Task, actor)
	if !ok {
		return
	}
	ev, err := d.newEventLocked(event.TypeClaimReleased, actor, req.Task, event.ClaimReleased{Reason: req.Reason})
	if err != nil {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// Eager: a release returns the task to the pool, and peers racing
	// to claim should see that as fast as a claim itself (T8).
	d.stageLocked(ev)
	if err := d.batcher.Flush(); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	if err := d.store.DeleteLease(c.ID); err != nil {
		httpError(w, http.StatusInternalServerError, "delete lease: %v", err)
		return
	}
	if err := d.refreshLocked(now); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"released": c.ID})
}

func (d *Daemon) handleFinishRun(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Task    string   `json:"task"`
		Outcome string   `json:"outcome"`
		Branch  string   `json:"branch"`
		PR      string   `json:"pr"`
		Commits []string `json:"commits"`
		Summary string   `json:"summary"`
	}
	if !decodeJSON(w, r, &req) || !requireField(w, "task", req.Task) {
		return
	}
	switch req.Outcome {
	case event.OutcomeDone, event.OutcomeFailed, event.OutcomeAbandoned,
		event.OutcomeBlocked, event.OutcomeInterrupted, event.OutcomeSuperseded:
	default:
		httpError(w, http.StatusBadRequest, "invalid outcome %q", req.Outcome)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	if _, ok := d.state.Tasks[req.Task]; !ok {
		httpError(w, http.StatusNotFound, "unknown task %s", req.Task)
		return
	}
	ev, err := d.newEventLocked(event.TypeRunFinished, actor, req.Task, event.RunFinished{
		Outcome: req.Outcome,
		Branch:  req.Branch,
		PR:      req.PR,
		Commits: req.Commits,
		Summary: req.Summary,
	})
	if err != nil {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err := d.commitLocked(false, ev); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": ev.ID})
}

func (d *Daemon) handleEscalate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Task     string `json:"task"`
		Question string `json:"question"`
		Context  string `json:"context"`
		Blocking bool   `json:"blocking"`
	}
	if !decodeJSON(w, r, &req) || !requireField(w, "task", req.Task) || !requireField(w, "question", req.Question) {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	if _, ok := d.state.Tasks[req.Task]; !ok {
		httpError(w, http.StatusNotFound, "unknown task %s", req.Task)
		return
	}
	ev, err := d.newEventLocked(event.TypeEscalationRaised, actor, req.Task, event.EscalationRaised{
		Question: req.Question,
		Context:  req.Context,
		Blocking: req.Blocking,
	})
	if err != nil {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// Eager (T8): a human is waiting on this.
	if err := d.commitLocked(true, ev); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": ev.ID})
}

func (d *Daemon) handleAnswerEscalation(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Escalation string `json:"escalation"`
		Answer     string `json:"answer"`
	}
	if !decodeJSON(w, r, &req) || !requireField(w, "escalation", req.Escalation) || !requireField(w, "answer", req.Answer) {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	esc, ok := d.state.Escalations[req.Escalation]
	if !ok {
		httpError(w, http.StatusNotFound, "unknown escalation %s", req.Escalation)
		return
	}
	ev, err := d.newEventLocked(event.TypeEscalationAnswered, actor, esc.Task, event.EscalationAnswered{
		Answer:     req.Answer,
		Escalation: req.Escalation,
	})
	if err != nil {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// Eager: a blocked agent may be polling for exactly this answer.
	if err := d.commitLocked(true, ev); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": ev.ID})
}

func (d *Daemon) handleAddNote(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Task string `json:"task"`
		Text string `json:"text"`
	}
	if !decodeJSON(w, r, &req) || !requireField(w, "task", req.Task) || !requireField(w, "text", req.Text) {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rejectDegraded(w) {
		return
	}
	if _, ok := d.state.Tasks[req.Task]; !ok {
		httpError(w, http.StatusNotFound, "unknown task %s", req.Task)
		return
	}
	ev, err := d.newEventLocked(event.TypeNoteAdded, actor, req.Task, event.NoteAdded{Text: req.Text})
	if err != nil {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err := d.commitLocked(false, ev); err != nil {
		d.replyWriteErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": ev.ID})
}

func (d *Daemon) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.state.Tasks[id]; !ok {
		httpError(w, http.StatusNotFound, "unknown task %s", id)
		return
	}
	writeJSON(w, http.StatusOK, d.hydrateLocked(id))
}

type stateTask struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Priority int      `json:"priority"`
	Labels   []string `json:"labels"`
	Holder   string   `json:"holder,omitempty"` // actor of the active claim
}

type stateResp struct {
	Degraded        string           `json:"degraded,omitempty"` // fail-safe message when read-only
	Tasks           []stateTask      `json:"tasks"`
	OpenEscalations []escalationJSON `json:"open_escalations"`
	Runs            []runJSON        `json:"runs"`
}

func (d *Daemon) handleState(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	resp := stateResp{
		Tasks:           []stateTask{},
		OpenEscalations: []escalationJSON{},
		Runs:            []runJSON{},
	}
	if d.degraded != nil {
		resp.Degraded = d.degraded.Error()
	}
	for _, id := range d.state.TaskOrder {
		t := d.state.Tasks[id]
		st := stateTask{ID: t.ID, Title: t.Title, Status: t.Status, Priority: t.Priority, Labels: t.Labels}
		if c := d.state.ActiveClaim(id); c != nil {
			st.Holder = c.Actor
		}
		resp.Tasks = append(resp.Tasks, st)
	}
	for _, e := range d.state.OpenEscalations() {
		resp.OpenEscalations = append(resp.OpenEscalations, escalationJSONOf(e))
	}
	for i := range d.state.Runs {
		resp.Runs = append(resp.Runs, runJSONOf(&d.state.Runs[i]))
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---- shared helpers ----

// rejectDegraded rejects the write with 503 when the daemon is in T3
// fail-safe read-only mode. Caller holds d.mu.
func (d *Daemon) rejectDegraded(w http.ResponseWriter) bool {
	if d.degraded != nil {
		httpError(w, http.StatusServiceUnavailable, "writes rejected (fail-safe read-only): %v", d.degraded)
		return true
	}
	return false
}

// replyWriteErr maps a failed write/refresh to a response: 503 when the
// failure put us in fail-safe mode, 500 otherwise.
func (d *Daemon) replyWriteErr(w http.ResponseWriter, err error) {
	if d.degraded != nil {
		httpError(w, http.StatusServiceUnavailable, "writes rejected (fail-safe read-only): %v", err)
		return
	}
	httpError(w, http.StatusInternalServerError, "%v", err)
}

// holderClaimLocked resolves the active claim on taskID and enforces
// that actor holds it. Caller holds d.mu with freshly replayed state.
func (d *Daemon) holderClaimLocked(w http.ResponseWriter, taskID, actor string) (*core.Claim, bool) {
	if _, ok := d.state.Tasks[taskID]; !ok {
		httpError(w, http.StatusNotFound, "unknown task %s", taskID)
		return nil, false
	}
	c := d.state.ActiveClaim(taskID)
	if c == nil {
		httpError(w, http.StatusConflict, "no active claim on task %s", taskID)
		return nil, false
	}
	if c.Actor != actor {
		httpError(w, http.StatusForbidden, "claim on task %s is held by %s, not %s", taskID, c.Actor, actor)
		return nil, false
	}
	return c, true
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

func taskJSONOf(t *core.Task) taskJSON {
	return taskJSON{
		ID: t.ID, Title: t.Title, Description: t.Description,
		Priority: t.Priority, Labels: t.Labels,
		Parents: t.Parents, DependsOn: t.DependsOn,
		Status: t.Status, CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt,
	}
}

func claimJSONOf(c *core.Claim) claimJSON {
	return claimJSON{ID: c.ID, Task: c.Task, Actor: c.Actor, Machine: c.Machine, MadeAt: c.MadeAt}
}

func runJSONOf(r *core.Run) runJSON {
	return runJSON{
		ID: r.ID, Task: r.Task, Claim: r.Claim, Actor: r.Actor, Machine: r.Machine,
		Outcome: r.Outcome, Branch: r.Branch, PR: r.PR, Commits: r.Commits,
		Summary: r.Summary, Synthesized: r.Synthesized,
	}
}

func escalationJSONOf(e *core.Escalation) escalationJSON {
	return escalationJSON{
		ID: e.ID, Task: e.Task, Actor: e.Actor, Question: e.Question,
		Context: e.Context, Blocking: e.Blocking, RaisedAt: e.RaisedAt,
		Answered: e.Answered, Answer: e.Answer, AnsweredBy: e.AnsweredBy,
	}
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

// requireActor extracts and validates the X-Tuhdoo-Actor header, which
// every write must carry (D7 principal: "human" or "human/agent").
func requireActor(w http.ResponseWriter, r *http.Request) (string, bool) {
	a := r.Header.Get("X-Tuhdoo-Actor")
	if err := validateActor(a); err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return "", false
	}
	return a, true
}

func validateActor(a string) error {
	if a == "" {
		return errors.New("X-Tuhdoo-Actor header is required on writes")
	}
	parts := strings.Split(a, "/")
	if len(parts) > 2 {
		return fmt.Errorf("invalid actor %q: want \"human\" or \"human/agent\"", a)
	}
	for _, p := range parts {
		if p == "" || strings.ContainsAny(p, " \t\r\n") {
			return fmt.Errorf("invalid actor %q: want \"human\" or \"human/agent\"", a)
		}
	}
	return nil
}

func requireField(w http.ResponseWriter, name, value string) bool {
	if value == "" {
		httpError(w, http.StatusBadRequest, "%q is required", name)
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httpError(w, http.StatusBadRequest, "bad request body: %v", err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, format string, args ...any) {
	writeJSON(w, code, map[string]string{"error": fmt.Sprintf(format, args...)})
}
