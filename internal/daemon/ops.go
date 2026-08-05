package daemon

// The operations layer: every verb both surfaces expose, as methods on
// *Daemon. The HTTP API (api.go) and the MCP endpoint (mcp.go) are thin
// wrappers around these — T4's "all surfaces are projections of the same
// daemon state" holds because there is exactly one implementation of
// each operation, serialized by the one write mutex.

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
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
	Tmp         string   `json:"tmp,omitempty" jsonschema:"optional name other items in this batch can reference as 'tmp:<name>' in depends_on; resolved to the real task ID on commit"`
	Title       string   `json:"title" jsonschema:"short imperative summary of the work (required)"`
	Description string   `json:"description,omitempty" jsonschema:"the task body — write it like a prompt: acceptance criteria, constraints, file pointers; output quality of whoever claims this is bounded by what you put here. For status inbox only, a fragment (or nothing) is legitimate: the prompt bar applies at promotion, not capture"`
	Status      string   `json:"status,omitempty" jsonschema:"initial status: open (default — claimable), inbox (untriaged capture; title-only is fine), or held (triaged but deliberately paused). Only open tasks are ever served to claim_next/claim_task"`
	Priority    int      `json:"priority,omitempty" jsonschema:"higher claims first; 0 is the default. Stored but inert while the task is inbox or held"`
	Labels      []string `json:"labels,omitempty" jsonschema:"free-form capability/topic tags, matchable by claim_next"`
	DependsOn   []string `json:"depends_on,omitempty" jsonschema:"task IDs (or 'tmp:<name>' batch refs) that must be done before this task is claimable; a dependency in inbox or held blocks like any other not-done task. Epics are just container tasks that depends_on their children"`
}

type updateTaskReq struct {
	Title       *string   `json:"title,omitempty" jsonschema:"new title; omit to leave unchanged"`
	Description *string   `json:"description,omitempty" jsonschema:"new description; omit to leave unchanged"`
	Status      *string   `json:"status,omitempty" jsonschema:"new status: open, inbox, held, done, or cancelled; omit to leave unchanged. open<->held is pause/resume; inbox->open is promotion — supply a prompt-quality description with it (see the agent protocol)"`
	Priority    *int      `json:"priority,omitempty" jsonschema:"new priority; omit to leave unchanged"`
	Labels      *[]string `json:"labels,omitempty" jsonschema:"full replacement label list; omit to leave unchanged"`
	DependsOn   *[]string `json:"depends_on,omitempty" jsonschema:"full replacement dependency-edge list (task IDs); omit to leave unchanged"`
}

func (r updateTaskReq) empty() bool {
	return r.Title == nil && r.Description == nil && r.Status == nil &&
		r.Priority == nil && r.Labels == nil && r.DependsOn == nil
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

// taskIDPrefix brands newly minted task IDs (T7, 2026-07-31): tuh- is
// uniquely tuhdoo. Tasks minted before the rebrand carry t- and are
// never rewritten (T3) — every surface derives the prefix from the ID
// itself (first hyphen), so both eras coexist until t- ages out.
const taskIDPrefix = "tuh-"

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
		// Tasks are born open (default), inbox, or held. Born-terminal
		// tasks (done/cancelled at create) are always a caller mistake.
		switch it.Status {
		case "", core.StatusOpen, core.StatusInbox, core.StatusHeld:
		default:
			return nil, nil, opErrf(http.StatusBadRequest,
				"item %d: invalid status %q: tasks are created open, inbox, or held", i, it.Status)
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
		for _, ref := range it.DependsOn {
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
		ids[i] = taskIDPrefix + u
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
		status := it.Status
		if status == "" {
			status = core.StatusOpen
		}
		ev, err := d.newEventLocked(event.TypeTaskCreated, actor, ids[i], event.TaskCreated{
			Title:       it.Title,
			Description: it.Description,
			Status:      status,
			Priority:    it.Priority,
			Labels:      it.Labels,
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
		case core.StatusOpen, core.StatusInbox, core.StatusHeld,
			core.StatusDone, core.StatusCancelled:
		default:
			return taskJSON{}, opErrf(http.StatusBadRequest,
				"invalid status %q: want open, inbox, held, done, or cancelled", *req.Status)
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
	if req.DependsOn != nil {
		for _, ref := range *req.DependsOn {
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
			// Name the actual blockers so the caller can act on them:
			// which dependency to finish, which escalation to answer.
			deps, escs := d.state.ClaimBlockers(taskID)
			var parts []string
			if len(deps) > 0 {
				parts = append(parts, "unmet dependencies "+strings.Join(deps, ", "))
			}
			if len(escs) > 0 {
				parts = append(parts, "blocked by open escalation "+strings.Join(escs, ", "))
			}
			return hydratedTask{}, opErrf(http.StatusConflict, "task %s is not ready: %s",
				taskID, strings.Join(parts, "; "))
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

// claimWarning rides every claim response (agent protocol step 5,
// 2026-08-04): a claim is provisional until confirmed, and verb results
// are the only channel the daemon has to a working agent (D6 — losers
// learn at verb-time), so the confirm-before-merge rule is stated at
// the moment the claim is handed over.
const claimWarning = "Before merging anything from this work, call confirm_claim and merge only on a " +
	"confirmed verdict — a claim is provisional, and merging on an unconfirmed claim is a protocol " +
	"violation (agent protocol, step 5). If the verdict is lost: stand down, close any PR you opened, " +
	"and record your attempt with finish_run."

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
	h := d.hydrateLocked(target.ID)
	h.Warning = claimWarning
	return h, nil
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

// opReleaseClaim ends the actor's hold voluntarily. Two shapes succeed:
// the live holder releasing (the task returns to the pool), and — since
// 2026-08-04 (D6 clause 3) — a provisionally-voided claimant standing
// down, which is exactly what the protocol asks of a race loser and so
// is acknowledged, never errored. The stand-down writes the release
// event for the reason's sake (replay no-ops a release from a
// non-holder) and deletes the loser's lease, which lets replay close
// the attempt as a synthesized superseded run immediately. The non-""
// message is the acknowledgment text for the stand-down arm.
func (d *Daemon) opReleaseClaim(actor, taskID, reason string) (claimID, message string, oe *opError) {
	if taskID == "" {
		return "", "", opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	if reason == "" {
		return "", "", opErrf(http.StatusBadRequest, "%q is required", "reason")
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", "", oe
	}
	if err := d.refreshLocked(now); err != nil {
		return "", "", d.writeErrLocked(err)
	}
	if _, ok := d.state.Tasks[taskID]; !ok {
		return "", "", opErrf(http.StatusNotFound, "unknown task %s", taskID)
	}
	c := d.state.ActiveClaim(taskID)
	if c == nil || c.Actor != actor {
		// Not the live holder: a voided claimant standing down is the
		// loser doing what every verb told it to do — acknowledge it.
		if mine := latestClaim(d.state, taskID, actor); mine != nil && mine.Status == core.ClaimVoided {
			return d.releaseVoidedLocked(actor, taskID, reason, mine, now)
		}
		if c == nil {
			return "", "", opErrf(http.StatusConflict, "no active claim on task %s", taskID)
		}
		return "", "", opErrf(http.StatusForbidden, "claim on task %s is held by %s, not %s", taskID, c.Actor, actor)
	}
	ev, err := d.newEventLocked(event.TypeClaimReleased, actor, taskID, event.ClaimReleased{Reason: reason})
	if err != nil {
		return "", "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	// Eager: a release returns the task to the pool, and peers racing
	// to claim should see that as fast as a claim itself (T8).
	d.stageLocked(ev)
	if err := d.batcher.Flush(); err != nil {
		return "", "", d.writeErrLocked(err)
	}
	if err := d.store.ReleaseLease(c.ID, now); err != nil {
		return "", "", opErrf(http.StatusInternalServerError, "release lease: %v", err)
	}
	if err := d.refreshLocked(now); err != nil {
		return "", "", d.writeErrLocked(err)
	}
	return c.ID, "", nil
}

// releaseVoidedLocked is the stand-down arm of opReleaseClaim: the
// actor's latest claim on the task lost its race (voided), and they are
// standing down as asked. The release event carries the reason onto the
// ledger; tombstoning the loser's lease at this instant closes the
// attempt — replay's next pass synthesizes the branch-less superseded
// run — so the ack points any remaining salvage at add_note. The lease
// is overwritten, never deleted: a missing lease reads as lapsed at
// every instant and would re-adjudicate the contest's past. Caller
// holds d.mu with freshly replayed state.
func (d *Daemon) releaseVoidedLocked(actor, taskID, reason string, mine *core.Claim, now time.Time) (claimID, message string, oe *opError) {
	if closed, synth := d.attemptCloseLocked(mine); closed || synth {
		return "", "", opErrf(http.StatusConflict,
			"%s's attempt on task %s is already closed — nothing to release; "+
				"if the branch is worth salvaging, record it with add_note", actor, taskID)
	}
	ev, err := d.newEventLocked(event.TypeClaimReleased, actor, taskID, event.ClaimReleased{Reason: reason})
	if err != nil {
		return "", "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	d.stageLocked(ev)
	if err := d.batcher.Flush(); err != nil {
		return "", "", d.writeErrLocked(err)
	}
	if err := d.store.ReleaseLease(mine.ID, now); err != nil {
		return "", "", opErrf(http.StatusInternalServerError, "release lease: %v", err)
	}
	if err := d.refreshLocked(now); err != nil {
		return "", "", d.writeErrLocked(err)
	}
	return mine.ID, fmt.Sprintf(
		"Stand-down acknowledged: your claim on task %s lost its race — a peer won this task. "+
			"Your attempt is recorded as superseded; close any open PR from it, and if the branch "+
			"is worth salvaging, record it with add_note.", taskID), nil
}

// finishRunResult is what a finish actually recorded — with the
// referee's statement when the record differs from the report (D6,
// 2026-08-04: the daemon is the referee of how attempts ended; agents
// report what they did).
type finishRunResult struct {
	ID      string `json:"id" jsonschema:"the recorded run.finished event's ID"`
	Outcome string `json:"outcome" jsonschema:"the outcome as recorded — done is refereed (D6): an attempt that lost its claim race is recorded superseded, never done"`
	Message string `json:"message,omitempty" jsonschema:"the referee's statement when the recorded outcome is not the reported one; empty when the run recorded exactly as reported"`
}

// opFinishRun records a run. It accepts every catalog outcome — the
// MCP layer narrows to the agent-reported set before calling (T5:
// interrupted and superseded are daemon-only verdicts). The finish
// guard enforces that the actor has an attempt of their own to close;
// the two checks compose — the MCP filter runs first, then this op
// guards both surfaces.
//
// The op is also the referee of how the attempt ended (D6, 2026-08-04):
// a finish on an attempt that lost its claim race is coerced to outcome
// superseded, keeping the reported branch/PR/commits/summary as the
// salvage record; and outcome done runs the confirmation gate — a done
// record is certified by winning the contest (a claim.confirmed at the
// remote's ref CAS), never by holding a provisional claim.
func (d *Daemon) opFinishRun(actor string, req finishRunReq) (finishRunResult, *opError) {
	if req.Task == "" {
		return finishRunResult{}, opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	switch req.Outcome {
	case event.OutcomeDone, event.OutcomeFailed, event.OutcomeAbandoned,
		event.OutcomeBlocked, event.OutcomeInterrupted, event.OutcomeSuperseded:
	default:
		return finishRunResult{}, opErrf(http.StatusBadRequest, "invalid outcome %q", req.Outcome)
	}

	now := time.Now()

	// Local pass. Holdership moves with the clock (a lease may have
	// lapsed since the last refresh), so re-replay at the current instant
	// before judging — the same posture as release_claim.
	d.mu.Lock()
	oe := d.degradedLocked()
	if oe == nil {
		if err := d.refreshLocked(now); err != nil {
			oe = d.writeErrLocked(err)
		}
	}
	if oe == nil {
		oe = d.finishGuardLocked(req.Task, actor)
	}
	if oe != nil {
		d.mu.Unlock()
		return finishRunResult{}, oe
	}

	if req.Outcome != event.OutcomeDone {
		// Non-done outcomes never certify work, so no gate — but a lost
		// attempt still records as superseded, whatever the report said
		// (D6 clause 3: the daemon is the referee of how attempts ended).
		defer d.mu.Unlock()
		if mine := latestClaim(d.state, req.Task, actor); mine != nil && mine.Status == core.ClaimVoided {
			return d.writeRunLocked(actor, req, event.OutcomeSuperseded)
		}
		return d.writeRunLocked(actor, req, req.Outcome)
	}

	// done: judge locally first — the confirmed winner, and contests
	// already lost in local state, both answer without a round-trip (the
	// same local pass as opConfirmClaim; a voided verdict only ever
	// hardens, so coercing on it is final-safe).
	res, _, joe := judgeConfirm(d.state, req.Task, actor)
	if joe != nil {
		d.mu.Unlock()
		return finishRunResult{}, joe
	}
	if res != nil {
		defer d.mu.Unlock()
		if res.Confirmed {
			return d.writeRunLocked(actor, req, event.OutcomeDone)
		}
		return d.writeRunLocked(actor, req, event.OutcomeSuperseded)
	}
	d.mu.Unlock()

	// done on an unconfirmed provisional winner: the verdict must be won
	// at the remote before the record is written — the same gate, same
	// loop, as confirm_claim. Flush first so the judged head carries
	// everything staged locally.
	if err := d.batcher.Flush(); err != nil {
		return finishRunResult{}, opErrf(http.StatusInternalServerError, "flush: %v", err)
	}
	verdict, oe := d.gateVerdict(actor, req.Task, now)
	if oe != nil {
		return finishRunResult{}, oe
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return finishRunResult{}, oe
	}
	if err := d.refreshLocked(now); err != nil {
		return finishRunResult{}, d.writeErrLocked(err)
	}
	// Re-guard: the world may have moved while the gate was out.
	if oe := d.finishGuardLocked(req.Task, actor); oe != nil {
		return finishRunResult{}, oe
	}
	if verdict.Confirmed {
		return d.writeRunLocked(actor, req, event.OutcomeDone)
	}
	return d.writeRunLocked(actor, req, event.OutcomeSuperseded)
}

// writeRunLocked stamps one run.finished carrying the refereed outcome
// and the caller's reported links, and answers with the record — plus
// the referee's stand-down statement when the recorded outcome is not
// the reported one. Caller holds d.mu with the finish guard passed.
func (d *Daemon) writeRunLocked(actor string, req finishRunReq, outcome string) (finishRunResult, *opError) {
	ev, err := d.newEventLocked(event.TypeRunFinished, actor, req.Task, event.RunFinished{
		Outcome: outcome,
		Branch:  req.Branch,
		PR:      req.PR,
		Commits: req.Commits,
		Summary: req.Summary,
	})
	if err != nil {
		return finishRunResult{}, opErrf(http.StatusInternalServerError, "%v", err)
	}
	if err := d.commitLocked(false, ev); err != nil {
		return finishRunResult{}, d.writeErrLocked(err)
	}
	message := ""
	if outcome != req.Outcome {
		message = d.supersededMessageLocked(req.Task, req.Outcome)
	}
	return finishRunResult{ID: ev.ID, Outcome: outcome, Message: message}, nil
}

// supersededMessageLocked words a coerced record for the loser (agent
// protocol, steps 5–6): recorded as superseded, not what was reported;
// a peer won; close any open PR. Caller holds d.mu.
func (d *Daemon) supersededMessageLocked(taskID, reported string) string {
	won := "a peer won this task"
	if active := d.state.ActiveClaim(taskID); active != nil {
		won = fmt.Sprintf("a peer won this task (%s holds it)", active.Actor)
	}
	return fmt.Sprintf("Recorded as superseded, not %s: %s. Stand down — close any open PR "+
		"from this attempt; your reported branch, PR, commits, and summary are kept on the "+
		"run for salvage.", reported, won)
}

// confirmGateRetries bounds the gate's fetch → judge → push loop when
// the remote keeps moving underneath it, mirroring the sync loop's
// maxCycleRetries.
const confirmGateRetries = 4

// confirmClaimResult is the referee's answer, shaped for an agent to
// act on without reading docs: confirmed means merge freely, lost means
// stand down.
type confirmClaimResult struct {
	Confirmed bool   `json:"confirmed" jsonschema:"true: the verdict is final and irrevocable — merge freely; false: this attempt lost the race — stand down, do not merge"`
	Claim     string `json:"claim,omitempty" jsonschema:"the claim the verdict is about"`
	Message   string `json:"message" jsonschema:"the verdict in words, with what to do next"`
}

func confirmedResult(claimID, taskID string) confirmClaimResult {
	return confirmClaimResult{Confirmed: true, Claim: claimID, Message: fmt.Sprintf(
		"Confirmed: claim %s on task %s is yours, irrevocably. Merge freely; "+
			"record the outcome with finish_run when the work lands.", claimID, taskID)}
}

func lostResult(claimID, why string) confirmClaimResult {
	return confirmClaimResult{Confirmed: false, Claim: claimID, Message: fmt.Sprintf(
		"Lost: %s. Stand down — do not merge, close any PR you opened, and call "+
			"finish_run to record your attempt; your branch and summary are kept for salvage.", why)}
}

// judgeConfirm is the referee's rule over one replayed state: given the
// head a confirmation would land on, is actor's claim the provisional
// winner with no competing confirmation? A nil result with a nil error
// means proceed — push the confirmation for claimID. The same rule
// judges the local state (remoteless: the daemon is the sole writer, so
// local is final) and the reconciled remote head (D6).
func judgeConfirm(st *core.State, taskID, actor string) (res *confirmClaimResult, claimID string, oe *opError) {
	if _, ok := st.Tasks[taskID]; !ok {
		return nil, "", opErrf(http.StatusNotFound, "unknown task %s", taskID)
	}
	// The actor's own attempt is their most recent claim on the task.
	mine := latestClaim(st, taskID, actor)
	if mine == nil {
		return nil, "", opErrf(http.StatusConflict,
			"no claim by %s on task %s: confirm_claim certifies your own claim — claim before confirming", actor, taskID)
	}
	if mine.Confirmation != "" {
		// Idempotent: an already-won verdict answers instantly (D6).
		r := confirmedResult(mine.ID, taskID)
		return &r, mine.ID, nil
	}
	switch active := st.ActiveClaim(taskID); {
	case active != nil && active.ID == mine.ID:
		return nil, mine.ID, nil // provisional winner, no competing confirmation: proceed
	case active != nil && active.Confirmation != "":
		r := lostResult(mine.ID, fmt.Sprintf(
			"task %s was confirmed to %s (claim %s), irrevocably", taskID, active.Actor, active.ID))
		return &r, mine.ID, nil
	case active != nil:
		r := lostResult(mine.ID, fmt.Sprintf(
			"an earlier claim by %s holds task %s and yours is provisionally voided", active.Actor, taskID))
		return &r, mine.ID, nil
	case mine.Status == core.ClaimVoided:
		r := lostResult(mine.ID, fmt.Sprintf(
			"your claim on task %s lost its race; that contest is over", taskID))
		return &r, mine.ID, nil
	default:
		// Released, finished, or expired without a confirmation: the
		// attempt ended by the actor's own hand or the lease clock —
		// there is nothing left to certify.
		return nil, "", opErrf(http.StatusConflict,
			"%s's claim %s on task %s ended %s — nothing to confirm; claim again to start a new attempt",
			actor, mine.ID, taskID, mine.Status)
	}
}

// opConfirmClaim is the D6 confirmation gate (T5 confirm_claim, added
// 2026-08-04): make the claim's verdict final before the agent merges.
// The final verdict is won, not computed — the daemon syncs against the
// remote, judges the head it would push onto, and lands claim.confirmed
// onto exactly that head through the remote's atomic ref CAS; a
// non-fast-forward means the remote moved and the loop re-judges,
// bounded. Remoteless (T2): the daemon is the sole writer, so the local
// verdict is final and instant. Remote configured but unreachable: a
// retryable refusal with nothing written — the referee never guesses.
func (d *Daemon) opConfirmClaim(actor, taskID string) (confirmClaimResult, *opError) {
	if taskID == "" {
		return confirmClaimResult{}, opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	now := time.Now()

	// Local pass first: caller mistakes, idempotent re-confirms, and
	// already-final losses all answer without a network round-trip.
	// Everything staged locally is flushed so the judged head carries it.
	d.mu.Lock()
	oe := d.degradedLocked()
	if oe == nil {
		if err := d.refreshLocked(now); err != nil {
			oe = d.writeErrLocked(err)
		}
	}
	var res *confirmClaimResult
	if oe == nil {
		res, _, oe = judgeConfirm(d.state, taskID, actor)
	}
	d.mu.Unlock()
	if oe != nil {
		return confirmClaimResult{}, oe
	}
	if res != nil {
		return *res, nil
	}
	if err := d.batcher.Flush(); err != nil {
		return confirmClaimResult{}, opErrf(http.StatusInternalServerError, "flush: %v", err)
	}
	return d.gateVerdict(actor, taskID, now)
}

// gateVerdict is the D6 gate's fetch → judge → push loop, shared by
// confirm_claim and every done-recording path (T5: the gate rides the
// verbs). Each pass judges the head a confirmation would land on and
// pushes claim.confirmed onto exactly that head through the remote's
// ref CAS; a non-fast-forward means the remote moved and the loop
// re-judges, bounded. Remoteless falls to the T2 local arm. The caller
// must have flushed everything staged, so the judged head carries it.
func (d *Daemon) gateVerdict(actor, taskID string, now time.Time) (confirmClaimResult, *opError) {
	for attempt := 0; attempt < confirmGateRetries; attempt++ {
		head, hstate, err := d.sync.GateHead()
		if errors.Is(err, gitx.ErrNoRemote) {
			return d.confirmLocally(actor, taskID, now)
		}
		if err != nil {
			return confirmClaimResult{}, opErrf(http.StatusServiceUnavailable,
				"cannot consult the remote (%v): confirmation refused, nothing written — "+
					"the referee never guesses; retry when the remote is reachable", err)
		}
		res, claimID, oe := judgeConfirm(hstate, taskID, actor)
		if oe != nil {
			return confirmClaimResult{}, oe
		}
		if res != nil {
			// The remote settled it while we weren't looking; make the
			// local cache agree before answering.
			d.refreshAfterGate(now)
			return *res, nil
		}
		d.mu.Lock()
		ev, evErr := d.newEventLocked(event.TypeClaimConfirmed, actor, taskID, event.ClaimConfirmed{Claim: claimID})
		d.mu.Unlock()
		if evErr != nil {
			return confirmClaimResult{}, opErrf(http.StatusInternalServerError, "%v", evErr)
		}
		err = d.sync.GatePush(head, ev)
		if errors.Is(err, gitx.ErrNonFastForward) {
			continue // the remote moved: refetch and re-judge (D6 in action)
		}
		if err != nil {
			return confirmClaimResult{}, opErrf(http.StatusServiceUnavailable,
				"confirmation push failed (%v): nothing certified — retry when the remote is reachable", err)
		}
		d.refreshAfterGate(now)
		return confirmedResult(claimID, taskID), nil
	}
	return confirmClaimResult{}, opErrf(http.StatusServiceUnavailable,
		"remote kept moving for %d attempts: confirmation not settled — retry", confirmGateRetries)
}

// confirmLocally is the T2 remoteless arm of the gate: no remote, one
// writer, so the local provisional verdict is the final verdict.
func (d *Daemon) confirmLocally(actor, taskID string, now time.Time) (confirmClaimResult, *opError) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return confirmClaimResult{}, oe
	}
	if err := d.refreshLocked(now); err != nil {
		return confirmClaimResult{}, d.writeErrLocked(err)
	}
	res, claimID, oe := judgeConfirm(d.state, taskID, actor)
	if oe != nil {
		return confirmClaimResult{}, oe
	}
	if res != nil {
		return *res, nil
	}
	ev, err := d.newEventLocked(event.TypeClaimConfirmed, actor, taskID, event.ClaimConfirmed{Claim: claimID})
	if err != nil {
		return confirmClaimResult{}, opErrf(http.StatusInternalServerError, "%v", err)
	}
	// Eager (T8): a confirmation is exactly the class of event peers
	// race against — it must not sit in the debounce window if a remote
	// is added later, and the agent is waiting on it now.
	if err := d.commitLocked(true, ev); err != nil {
		return confirmClaimResult{}, d.writeErrLocked(err)
	}
	return confirmedResult(claimID, taskID), nil
}

// refreshAfterGate folds a gate outcome into the cached state. Failures
// are logged, never returned: the verdict is already settled on the
// branch, and the next refresh converges.
func (d *Daemon) refreshAfterGate(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.refreshLocked(now); err != nil {
		d.log.Printf("daemon: refresh after confirmation gate: %v", err)
	}
}

// opEscalate raises a question for a human. The warning return is the
// verb-time stand-down notice when the caller's claim on the task has
// lost its race (D6), "" otherwise.
func (d *Daemon) opEscalate(actor string, req escalateReq) (id, warning string, oe *opError) {
	if req.Task == "" {
		return "", "", opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	if req.Question == "" {
		return "", "", opErrf(http.StatusBadRequest, "%q is required", "question")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", "", oe
	}
	if _, ok := d.state.Tasks[req.Task]; !ok {
		return "", "", opErrf(http.StatusNotFound, "unknown task %s", req.Task)
	}
	ev, err := d.newEventLocked(event.TypeEscalationRaised, actor, req.Task, event.EscalationRaised{
		Question: req.Question,
		Context:  req.Context,
		Blocking: req.Blocking,
	})
	if err != nil {
		return "", "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	// Eager (T8): a human is waiting on this.
	if err := d.commitLocked(true, ev); err != nil {
		return "", "", d.writeErrLocked(err)
	}
	return ev.ID, d.lostNoticeLocked(req.Task, actor), nil
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

// opAddNote records a checkpoint note. The warning return is the
// verb-time stand-down notice when the caller's claim on the task has
// lost its race (D6), "" otherwise.
func (d *Daemon) opAddNote(actor, task, text string) (id, warning string, oe *opError) {
	if task == "" {
		return "", "", opErrf(http.StatusBadRequest, "%q is required", "task")
	}
	if text == "" {
		return "", "", opErrf(http.StatusBadRequest, "%q is required", "text")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if oe := d.degradedLocked(); oe != nil {
		return "", "", oe
	}
	if _, ok := d.state.Tasks[task]; !ok {
		return "", "", opErrf(http.StatusNotFound, "unknown task %s", task)
	}
	ev, err := d.newEventLocked(event.TypeNoteAdded, actor, task, event.NoteAdded{Text: text})
	if err != nil {
		return "", "", opErrf(http.StatusInternalServerError, "%v", err)
	}
	if err := d.commitLocked(false, ev); err != nil {
		return "", "", d.writeErrLocked(err)
	}
	return ev.ID, d.lostNoticeLocked(task, actor), nil
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
// get_backlog: ready-filtered and dependency-aware, not a raw dump),
// plus the inbox and held shelves (2026-07-31) in creation order so
// agents can orient on parked and captured work without ever being
// served it — claim_next/claim_task take from ready alone. Lease
// verdicts move with the clock, so replay at the current instant
// first — a stale expiry must not hide a ready task.
//
// scope (T5 read parity, 2026-08-02) requests the remaining sections
// by name: in_progress, blocked, done, cancelled, escalations. Empty
// or omitted, the result carries exactly the three arrays above.
func (d *Daemon) opBacklog(scope []string) (backlogResult, *opError) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.degraded == nil {
		if err := d.refreshLocked(now); err != nil {
			return backlogResult{}, d.writeErrLocked(err)
		}
	}
	res := backlogResult{Ready: []taskJSON{}, Inbox: []taskJSON{}, Held: []taskJSON{}}
	for _, t := range d.state.ReadyTasks() {
		res.Ready = append(res.Ready, taskJSONOf(t))
	}
	for _, id := range d.state.TaskOrder {
		switch t := d.state.Tasks[id]; t.Status {
		case core.StatusInbox:
			res.Inbox = append(res.Inbox, taskJSONOf(t))
		case core.StatusHeld:
			res.Held = append(res.Held, taskJSONOf(t))
		}
	}
	for _, name := range scope {
		switch name {
		case "in_progress":
			rows := d.inProgressRowsLocked()
			res.InProgress = &rows
		case "blocked":
			rows := d.blockedRowsLocked()
			res.Blocked = &rows
		case "done":
			rows := d.closedRowsLocked(core.StatusDone)
			res.Done = &rows
		case "cancelled":
			rows := d.closedRowsLocked(core.StatusCancelled)
			res.Cancelled = &rows
		case "escalations":
			rows := d.openEscalationRowsLocked()
			res.Escalations = &rows
		default:
			return backlogResult{}, opErrf(http.StatusBadRequest,
				"unknown scope %q: valid values are in_progress, blocked, done, cancelled, escalations", name)
		}
	}
	return res, nil
}

// scopeRowOf is the slim base row every scope section shares: no
// description by design (orientation lists, hydration digs).
func scopeRowOf(t *core.Task) scopeTaskJSON {
	return scopeTaskJSON{ID: t.ID, Title: t.Title, Status: t.Status,
		Priority: t.Priority, Labels: t.Labels}
}

// inProgressRowsLocked lists actively claimed open tasks in creation
// order, each with its holder and lease expiry.
func (d *Daemon) inProgressRowsLocked() []scopeTaskJSON {
	rows := []scopeTaskJSON{}
	for _, id := range d.state.TaskOrder {
		t := d.state.Tasks[id]
		if t.Status != core.StatusOpen {
			continue
		}
		c := d.state.ActiveClaim(id)
		if c == nil {
			continue
		}
		row := scopeRowOf(t)
		row.Holder = c.Actor
		if exp, ok := d.leases[c.ID]; ok {
			e := exp
			row.LeaseExpires = &e
		}
		rows = append(rows, row)
	}
	return rows
}

// blockedRowsLocked lists open, unclaimed tasks that cannot be claimed,
// in creation order. Membership and reasons both come from
// core.ClaimBlockers — the daemon holds no blocking predicate of its
// own, so this list can never disagree with what Ready withholds.
func (d *Daemon) blockedRowsLocked() []scopeTaskJSON {
	rows := []scopeTaskJSON{}
	for _, id := range d.state.TaskOrder {
		t := d.state.Tasks[id]
		if t.Status != core.StatusOpen || d.state.ActiveClaim(id) != nil {
			continue
		}
		deps, escs := d.state.ClaimBlockers(id)
		if len(deps) == 0 && len(escs) == 0 {
			continue // claimable — that is ready's row, not blocked's
		}
		row := scopeRowOf(t)
		for _, dep := range deps {
			row.WaitingOn = append(row.WaitingOn, "dep:"+dep)
		}
		for _, esc := range escs {
			row.WaitingOn = append(row.WaitingOn, "esc:"+esc)
		}
		rows = append(rows, row)
	}
	return rows
}

// closedRowsLocked lists the tasks in one terminal status, newest close
// first (recency is the browse axis), each with its close metadata.
// Stable over creation order, so equal stamps keep a deterministic
// order.
func (d *Daemon) closedRowsLocked(status string) []scopeTaskJSON {
	rows := []scopeTaskJSON{}
	for _, id := range d.state.TaskOrder {
		t := d.state.Tasks[id]
		if t.Status != status {
			continue
		}
		row := scopeRowOf(t)
		closed := t.ClosedAt
		row.ClosedAt = &closed
		row.ClosedBy = t.ClosedBy
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ClosedAt.After(*rows[j].ClosedAt)
	})
	return rows
}

// openEscalationRowsLocked lists open escalations in raise order — the
// discovery path for the escalation IDs relay_answer needs.
func (d *Daemon) openEscalationRowsLocked() []openEscalationJSON {
	rows := []openEscalationJSON{}
	for _, e := range d.state.OpenEscalations() {
		rows = append(rows, openEscalationJSON{ID: e.ID, Task: e.Task,
			Question: e.Question, Context: e.Context,
			Blocking: e.Blocking, RaisedAt: e.RaisedAt})
	}
	return rows
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

// latestClaim returns actor's most recent claim on taskID, or nil —
// their attempt, the one every write-side judgment is about.
// ClaimsByTask is replay (ULID) order, so the last match is it.
func latestClaim(st *core.State, taskID, actor string) *core.Claim {
	var mine *core.Claim
	for _, cid := range st.ClaimsByTask[taskID] {
		if c := st.Claims[cid]; c.Actor == actor {
			mine = c
		}
	}
	return mine
}

// attemptCloseLocked reports how the attempt under claim c stands
// closed: by a real run of the actor's minted after the claim, or by
// replay's synthesized run for the claim itself (interrupted for an
// expired holder, superseded for an expired loser). Caller holds d.mu.
func (d *Daemon) attemptCloseLocked(c *core.Claim) (closed, synthesized bool) {
	for i := range d.state.Runs {
		r := &d.state.Runs[i]
		if r.Task != c.Task || r.Actor != c.Actor {
			continue
		}
		switch {
		case r.Synthesized && r.Claim == c.ID:
			synthesized = true
		case r.ID > c.ID:
			closed = true
		}
	}
	return closed, synthesized
}

// lostNoticeLocked words the verb-time stand-down (D6 clause 3,
// 2026-08-04: losers learn at verb-time — there is no push channel to a
// working agent, so every verb a provisionally-voided claimant touches
// on its task states the loss plainly). Empty unless actor's latest
// claim on taskID is a voided, still-unclosed attempt: once the
// superseded run exists, the loss is on the record and further verbs
// are salvage, not stand-down. Caller holds d.mu.
func (d *Daemon) lostNoticeLocked(taskID, actor string) string {
	c := latestClaim(d.state, taskID, actor)
	if c == nil || c.Status != core.ClaimVoided {
		return ""
	}
	if closed, synth := d.attemptCloseLocked(c); closed || synth {
		return ""
	}
	return fmt.Sprintf("Your claim on task %s lost its race — a peer holds this task, and your "+
		"attempt will be recorded as superseded. Stand down: do not merge, close any open PR from "+
		"this attempt, and record it with finish_run (your branch and summary are kept for salvage).",
		taskID)
}

// finishGuardLocked enforces that actor has an attempt of their own to
// close on taskID before a run.finished event is written. Three shapes
// are admitted: the actor holds the live claim (the normal finish); the
// actor's latest claim ended released and no later run of theirs has
// closed it (the blocking protocol: escalate → release_claim →
// finish_run blocked); or it ended voided and is likewise unclosed — a
// race loser reporting back, admitted only until the voided claim's
// lease expires, because at expiry replay synthesizes the branch-less
// superseded close itself (D6 clause 3, 2026-08-04) and one close per
// attempt makes that final. Everything else is rejected: no claim
// history, a live claim held by someone else, an expired claim (replay
// already synthesized its interrupted run), or a second close of the
// same attempt. This is write-side validation only — replay never
// re-judges stored run.finished events (T3), so the claimless runs
// already on ledgers stand untouched. Caller holds d.mu with freshly
// replayed state.
func (d *Daemon) finishGuardLocked(taskID, actor string) *opError {
	if _, ok := d.state.Tasks[taskID]; !ok {
		return opErrf(http.StatusNotFound, "unknown task %s", taskID)
	}
	active := d.state.ActiveClaim(taskID)
	if active != nil && active.Actor == actor {
		return nil
	}
	// Not the live holder: the only closable attempt is the actor's own
	// most recent claim on the task.
	latest := latestClaim(d.state, taskID, actor)
	if latest != nil && (latest.Status == core.ClaimReleased || latest.Status == core.ClaimVoided) {
		// One close per attempt: ULIDs order events in time, so any run
		// by this actor minted after the claim already closed it — and
		// replay's synthesized superseded run closes a voided attempt
		// whose lease ran out before the loser reported.
		closed, synth := d.attemptCloseLocked(latest)
		if !closed && !synth {
			return nil
		}
		if latest.Status == core.ClaimVoided && synth && !closed {
			return opErrf(http.StatusConflict,
				"%s's attempt on task %s was already closed as superseded (claim %s lost its race "+
					"and its lease expired before a report); if the branch is worth salvaging, "+
					"record it with add_note", actor, taskID, latest.ID)
		}
	}
	switch {
	case active != nil:
		return opErrf(http.StatusForbidden,
			"claim on task %s is held by %s, not %s", taskID, active.Actor, actor)
	case latest == nil:
		return opErrf(http.StatusConflict,
			"no claim by %s on task %s: finish_run closes your own attempt — claim before finishing", actor, taskID)
	default:
		// Finished or expired claim, or a released/voided one a later
		// run of the actor's already closed.
		return opErrf(http.StatusConflict,
			"%s's attempt on task %s (claim %s, %s) is already closed", actor, taskID, latest.ID, latest.Status)
	}
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
