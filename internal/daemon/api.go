package daemon

// The JSON HTTP API (002 T4): boring REST over the unix socket. Every
// handler is decode → ops.go call → respond; the MCP surface (mcp.go)
// wraps the same operations, so the two cannot disagree. Writers
// identify themselves with the X-Tuhdoo-Actor header (D7 principal:
// "human" or "human/agent").

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/syncer"
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
	// The MCP endpoint (T4): GET, POST, and DELETE all route to the one
	// streamable handler.
	mux.Handle("/mcp", d.mcpHandler())
	return mux
}

// ---- response shapes ----

type taskJSON struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    *int      `json:"priority" jsonschema:"P0-highest: 0 is most urgent, larger numbers are less urgent; null means unprioritized (sorts after every prioritized task)"`
	Labels      []string  `json:"labels"`
	DependsOn   []string  `json:"depends_on"`
	Status      string    `json:"status"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	// Close metadata (history view, 2026-08-02): present on done and
	// cancelled tasks only — replay-derived, never stored (T3).
	ClosedAt *time.Time `json:"closed_at,omitempty"`
	ClosedBy string     `json:"closed_by,omitempty"`
}

// scopeTaskJSON is the slim orientation row served by get_backlog's
// scope sections (T5 read parity, 2026-08-02). No description by
// design — orientation lists, hydration digs (get_task). The trailing
// fields are per-scope payoffs: holder/lease_expires on in_progress
// rows, waiting_on on blocked rows, closed_at/closed_by on done and
// cancelled rows.
type scopeTaskJSON struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Priority     *int       `json:"priority" jsonschema:"P0-highest: 0 is most urgent, larger numbers are less urgent; null means unprioritized (sorts after every prioritized task)"`
	Labels       []string   `json:"labels"`
	Holder       string     `json:"holder,omitempty" jsonschema:"in_progress rows: the principal holding the active claim"`
	LeaseExpires *time.Time `json:"lease_expires,omitempty" jsonschema:"in_progress rows: when the holder's lease expires unless renewed"`
	WaitingOn    []string   `json:"waiting_on,omitempty" jsonschema:"blocked rows: condensed reasons — dep:<task-id> per unmet dependency, esc:<escalation-id> per open blocking escalation; hydrate with get_task for the story"`
	// Loud blockage annotations (2026-08-05 edge grill). Annotations
	// only: blocked stays the status, these say more about why.
	CancelledDeps []string   `json:"cancelled_deps,omitempty" jsonschema:"blocked rows: the unmet dependencies sitting cancelled — cancelled never counts as done; re-pointing the edge is a human decision"`
	Cyclic        bool       `json:"cyclic,omitempty" jsonschema:"blocked rows: this task sits on a depends_on loop among not-done tasks and can never become ready until a human cuts an edge"`
	ClosedAt      *time.Time `json:"closed_at,omitempty" jsonschema:"done/cancelled rows: when the task entered its terminal status"`
	ClosedBy      string     `json:"closed_by,omitempty" jsonschema:"done/cancelled rows: the actor that closed it"`
}

// openEscalationJSON is get_backlog's escalations-scope row: the open
// question in full (the question text is the payload — not slim by
// design), no answer fields — open only; answered escalations hydrate
// via get_task.
type openEscalationJSON struct {
	ID       string    `json:"id"`
	Task     string    `json:"task"`
	Question string    `json:"question"`
	Context  string    `json:"context"`
	Blocking bool      `json:"blocking"`
	RaisedAt time.Time `json:"raised_at"`
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
	MergedAs    []string `json:"merged_as"`
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
	RelayedBy  string    `json:"relayed_by"`
}

type noteJSON struct {
	ID      string    `json:"id"`
	Task    string    `json:"task"`
	Actor   string    `json:"actor"`
	Text    string    `json:"text"`
	AddedAt time.Time `json:"added_at"`
}

// updateJSON is one task edit (core.Update): the actor and the compact
// per-field summaries the history surfaces render verbatim.
type updateJSON struct {
	ID     string   `json:"id"`
	Task   string   `json:"task"`
	Actor  string   `json:"actor"`
	Fields []string `json:"fields"`
}

// hydratedTask is one task with everything attached (T5 get_task:
// start work in one call).
type hydratedTask struct {
	Task        taskJSON         `json:"task"`
	Claim       *claimJSON       `json:"claim"`
	Notes       []noteJSON       `json:"notes"`
	Runs        []runJSON        `json:"runs"`
	Escalations []escalationJSON `json:"escalations"`
	Updates     []updateJSON     `json:"updates"`

	// Warning carries the standing confirm-before-merge rule on claim
	// responses (agent protocol step 5; D6 call-time stand-down) and is
	// empty on plain hydration (get_task).
	Warning string `json:"warning,omitempty" jsonschema:"on claim responses: the standing rule — call confirm_claim and merge only on a confirmed verdict; merging on an unconfirmed claim is a protocol violation"`
}

// ---- handlers ----

func (d *Daemon) handleCreateTasks(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var items []createTaskItem
	if !decodeJSON(w, r, &items) {
		return
	}
	ids, tmp, oe := d.opCreateTasks(actor, items)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ids": ids, "tmp": tmp})
}

func (d *Daemon) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req updateTaskReq
	if !decodeJSON(w, r, &req) {
		return
	}
	t, oe := d.opUpdateTask(actor, r.PathValue("id"), req)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

type claimReq struct {
	Task   string   `json:"task"`
	Next   bool     `json:"next"`
	Labels []string `json:"labels"`
}

// handleClaim is claim_task ({task}) or claim_next ({next, labels}).
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

	if req.Task != "" {
		h, oe := d.opClaimTask(actor, req.Task)
		if oe != nil {
			writeOpError(w, oe)
			return
		}
		writeJSON(w, http.StatusOK, h)
		return
	}
	h, oe := d.opClaimNext(actor, req.Labels)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	if h == nil {
		httpError(w, http.StatusConflict, "no ready task matches")
		return
	}
	writeJSON(w, http.StatusOK, *h)
}

func (d *Daemon) handleRenewClaim(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Task string `json:"task"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	claim, expires, oe := d.opRenewClaim(actor, req.Task)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"claim":   claim,
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
	if !decodeJSON(w, r, &req) {
		return
	}
	claim, message, oe := d.opReleaseClaim(actor, req.Task, req.Reason)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	body := map[string]any{"released": claim}
	if message != "" {
		body["message"] = message
	}
	writeJSON(w, http.StatusOK, body)
}

func (d *Daemon) handleFinishRun(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req finishRunReq
	if !decodeJSON(w, r, &req) {
		return
	}
	res, oe := d.opFinishRun(actor, req)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (d *Daemon) handleEscalate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var req escalateReq
	if !decodeJSON(w, r, &req) {
		return
	}
	id, warning, oe := d.opEscalate(actor, req)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	body := map[string]any{"id": id}
	if warning != "" {
		body["warning"] = warning
	}
	writeJSON(w, http.StatusOK, body)
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
	if !decodeJSON(w, r, &req) {
		return
	}
	id, oe := d.opAnswerEscalation(actor, req.Escalation, req.Answer)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
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
	if !decodeJSON(w, r, &req) {
		return
	}
	id, warning, oe := d.opAddNote(actor, req.Task, req.Text)
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	body := map[string]any{"id": id}
	if warning != "" {
		body["warning"] = warning
	}
	writeJSON(w, http.StatusOK, body)
}

func (d *Daemon) handleGetTask(w http.ResponseWriter, r *http.Request) {
	h, oe := d.opGetTask(r.PathValue("id"))
	if oe != nil {
		writeOpError(w, oe)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

type stateTask struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Priority *int     `json:"priority"`
	Labels   []string `json:"labels"`
	Holder   string   `json:"holder,omitempty"` // actor of the active claim
	// One classifier (2026-08-03): core's verdict rides the wire so no
	// client re-derives it. Situation is always present — ready /
	// in_progress / blocked for open tasks, the status word for the
	// rest. The blocker lists carry IDs for every task regardless of
	// status (core.ClaimBlockers' contract); rendering the why is the
	// client's job, deciding it is not.
	Situation           string   `json:"situation"`
	UnmetDeps           []string `json:"unmet_deps,omitempty"`
	BlockingEscalations []string `json:"blocking_escalations,omitempty"`
	// Loud blockage annotations (2026-08-05 edge grill): the unmet deps
	// sitting cancelled, and membership in a depends_on loop among
	// not-done tasks. Annotations, not statuses — a marked task is
	// still just blocked; clients render the why, never re-derive it.
	CancelledDeps []string `json:"cancelled_deps,omitempty"`
	Cyclic        bool     `json:"cyclic,omitempty"`
	// Close metadata (history view, 2026-08-02): the TUI's history rows
	// sort and stamp off the state listing, so it rides here too.
	ClosedAt *time.Time `json:"closed_at,omitempty"`
	ClosedBy string     `json:"closed_by,omitempty"`
}

type stateResp struct {
	Degraded        string           `json:"degraded,omitempty"` // fail-safe message when read-only
	Sync            syncJSON         `json:"sync"`
	Tasks           []stateTask      `json:"tasks"`
	OpenEscalations []escalationJSON `json:"open_escalations"`
	Runs            []runJSON        `json:"runs"`
}

// syncJSON is the sync loop's health for status surfaces.
type syncJSON struct {
	Mode       string `json:"mode"` // local-only | syncing | error
	Remote     string `json:"remote,omitempty"`
	LastFetch  string `json:"last_fetch,omitempty"` // RFC3339
	LastPush   string `json:"last_push,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	Collisions int    `json:"collisions"`
	Merges     int    `json:"merges"`
}

func syncJSONOf(st syncer.Status) syncJSON {
	out := syncJSON{
		Mode:       st.Mode,
		Remote:     st.Remote,
		LastError:  st.LastError,
		Collisions: st.Collisions,
		Merges:     st.Merges,
	}
	if st.Mode == "" {
		out.Mode = "starting"
	}
	if !st.LastFetch.IsZero() {
		out.LastFetch = st.LastFetch.UTC().Format(time.RFC3339)
	}
	if !st.LastPush.IsZero() {
		out.LastPush = st.LastPush.UTC().Format(time.RFC3339)
	}
	return out
}

func (d *Daemon) handleState(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	// Lease verdicts move with the clock, so replay at the current
	// instant first (D6: expiry is evaluated at read time) — the status
	// poll must not render a lapsed lease as a live holder. Degraded
	// skips the refresh: the last good state keeps serving reads.
	if d.degraded == nil {
		if err := d.refreshLocked(now); err != nil {
			writeOpError(w, d.writeErrLocked(err))
			return
		}
	}
	resp := stateResp{
		Sync:            syncJSONOf(d.sync.Status()),
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
		st.Situation = d.state.Situation(id)
		b := d.state.Blockage(id)
		st.UnmetDeps, st.BlockingEscalations = b.UnmetDeps, b.BlockingEscalations
		st.CancelledDeps, st.Cyclic = b.CancelledDeps, b.Cyclic
		if c := d.state.ActiveClaim(id); c != nil {
			st.Holder = c.Actor
		}
		if !t.ClosedAt.IsZero() {
			closed := t.ClosedAt
			st.ClosedAt = &closed
			st.ClosedBy = t.ClosedBy
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

func taskJSONOf(t *core.Task) taskJSON {
	out := taskJSON{
		ID: t.ID, Title: t.Title, Description: t.Description,
		Priority: t.Priority, Labels: t.Labels, DependsOn: t.DependsOn,
		Status: t.Status, CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt,
	}
	if !t.ClosedAt.IsZero() {
		closed := t.ClosedAt
		out.ClosedAt = &closed
		out.ClosedBy = t.ClosedBy
	}
	return out
}

func claimJSONOf(c *core.Claim) claimJSON {
	return claimJSON{ID: c.ID, Task: c.Task, Actor: c.Actor, Machine: c.Machine, MadeAt: c.MadeAt}
}

func runJSONOf(r *core.Run) runJSON {
	return runJSON{
		ID: r.ID, Task: r.Task, Claim: r.Claim, Actor: r.Actor, Machine: r.Machine,
		Outcome: r.Outcome, Branch: r.Branch, PR: r.PR, Commits: r.Commits,
		MergedAs: r.MergedAs, Summary: r.Summary, Synthesized: r.Synthesized,
	}
}

func escalationJSONOf(e *core.Escalation) escalationJSON {
	return escalationJSON{
		ID: e.ID, Task: e.Task, Actor: e.Actor, Question: e.Question,
		Context: e.Context, Blocking: e.Blocking, RaisedAt: e.RaisedAt,
		Answered: e.Answered, Answer: e.Answer, AnsweredBy: e.AnsweredBy,
		RelayedBy: e.RelayedBy,
	}
}

// requireActor extracts and validates the X-Tuhdoo-Actor header, which
// every write must carry (D7 principal: "human" or "human/agent").
func requireActor(w http.ResponseWriter, r *http.Request) (string, bool) {
	a := r.Header.Get(actorHeader)
	if err := ValidateActor(a); err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return "", false
	}
	return a, true
}

// actorHeader carries the D7 principal on every write, on both the
// HTTP and MCP surfaces.
const actorHeader = "X-Tuhdoo-Actor"

// ValidateActor checks a D7 principal ("human" or "human/agent").
// Exported for the CLI's mcp shim, which binds the principal before the
// daemon ever sees it.
func ValidateActor(a string) error {
	if a == "" {
		return errors.New(actorHeader + " header is required on writes")
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

func writeOpError(w http.ResponseWriter, oe *opError) {
	writeJSON(w, oe.code, map[string]string{"error": oe.msg})
}

func httpError(w http.ResponseWriter, code int, format string, args ...any) {
	writeJSON(w, code, map[string]string{"error": fmt.Sprintf(format, args...)})
}
