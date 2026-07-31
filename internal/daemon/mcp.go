package daemon

// The MCP surface (002 T5): streamable HTTP on the same unix socket,
// exactly eleven tools, projected from the same ops.go operations as
// the HTTP API. One *mcp.Server is minted per session so tool closures can
// capture the session's actor (bound from the X-Tuhdoo-Actor header on
// the initialize POST) and its claim set for lease auto-renewal —
// session liveness, not agent diligence, is what keeps leases alive
// (T5: no heartbeat verb).

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
)

// mcpInstructions orients a fresh agent (T5: the D1 loop in minimum
// calls). The full protocol lives in docs/agent-protocol.md; this is
// the elevator version every session carries.
const mcpInstructions = "tuhdoo is the shared work ledger for this repository. " +
	"The loop: claim_next (or claim_task) to take work, escalate when a human must decide " +
	"(relay_answer records their answer if it arrives out of band, in your own session), " +
	"and always end with finish_run (or release_claim to stand down) — the typed " +
	"transitions are the record your successors resume from. add_note is optional: " +
	"checkpoint mid-flight context only when it would save a successor real work. " +
	"Your claim's lease renews automatically while this session is connected; " +
	"if the session drops, the task returns to the pool."

// mcpKeepAliveFailures tolerated before the daemon declares the session
// dead — mirroring T8's "three missed renewals = expiry" posture, so a
// transiently slow client is not confused with a vanished one.
const mcpKeepAliveFailures = 3

// mcpSession is one connected agent: its principal and the claims made
// through this session, which the daemon keeps leased until the session
// ends.
type mcpSession struct {
	actor string // full principal; empty until minted when mint is set
	// mint completes an auto-derived principal (agentNameHeader). It
	// runs at most once, at session bind — getServer is documented to
	// run any number of times per session, so the counter bump cannot
	// live there.
	mint     func() string
	mintOnce sync.Once
	stop     chan struct{}
	once     sync.Once // guards the goroutine pair spawn

	mu     sync.Mutex
	claims map[string]string // task ID → claim ID
}

// principal returns the session's actor, minting it on first use for
// auto-derived sessions. sync.Once makes the mint exactly-once and
// publishes actor safely to every later caller.
func (s *mcpSession) principal() string {
	if s.mint != nil {
		s.mintOnce.Do(func() { s.actor = s.mint() })
	}
	return s.actor
}

func (s *mcpSession) track(task, claim string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims[task] = claim
}

func (s *mcpSession) untrack(task string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.claims, task)
}

// snapshot copies the tracked claims so renewal never holds both locks.
func (s *mcpSession) snapshot() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.claims))
	for task, claim := range s.claims {
		out[task] = claim
	}
	return out
}

// agentNameHeader carries the harness's MCP initialize clientInfo.name,
// forwarded by the shim when no --as override was given. Its presence
// asks the daemon to mint the agent half of the principal at session
// bind (D7: agent names are assigned when a harness connects over MCP).
const agentNameHeader = "X-Tuhdoo-Agent-Name"

// mcpHandler mounts the streamable HTTP endpoint. The getServer
// callback runs once per session — the first POST arrives without an
// Mcp-Session-Id header — which is exactly where the actor binds:
// either the full principal from X-Tuhdoo-Actor, or, when
// X-Tuhdoo-Agent-Name is present, a daemon-minted
// <human>/<client-name>-<n> unique among this daemon's sessions.
func (d *Daemon) mcpHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		actor := r.Header.Get(actorHeader)
		client := r.Header.Get(agentNameHeader)
		if err := validateSessionIdentity(actor, client); err != nil {
			d.log.Printf("daemon: mcp: rejecting session: %v", err)
			return nil // the SDK serves a 400
		}
		return d.newMCPServer(actor, client)
	}, &mcp.StreamableHTTPOptions{
		// Requests arrive over the repo-local unix socket, which is
		// gated by filesystem permissions; Host headers over a unix
		// socket are arbitrary, and the SDK's DNS-rebinding check could
		// 403 them. There is no network listener to rebind against.
		DisableLocalhostProtection: true,
		// No SessionTimeout: an idle-but-alive agent thinking for
		// twenty minutes keeps its session; keepalive pings are the
		// liveness signal, not request traffic.
	})
}

// validateSessionIdentity checks the identity headers at the door.
// Without an agent name, the actor must be a full valid principal.
// With one, the actor must be a root human (single segment) — the shim
// derives it from git identity (D7) and the daemon mints the agent
// half at bind.
func validateSessionIdentity(actor, client string) error {
	if err := ValidateActor(actor); err != nil {
		return err
	}
	if client != "" && strings.Contains(actor, "/") {
		return fmt.Errorf("auto-derive: %q must be a root human, not an agent principal", actor)
	}
	return nil
}

// mintPrincipal builds <human>/<client>-<n> for a session that asked
// for auto-derivation. The per-name counter is monotonic for the
// daemon's lifetime: distinct sessions never share a name, which is
// stronger than uniqueness among live sessions and keeps ledger
// attribution honest. It resets on daemon restart.
func (d *Daemon) mintPrincipal(human, client string) string {
	name := sanitizeAgentName(client)
	d.agentMu.Lock()
	d.agentSeq[name]++
	n := d.agentSeq[name]
	d.agentMu.Unlock()
	minted := fmt.Sprintf("%s/%s-%d", human, name, n)
	d.log.Printf("daemon: mcp: session bound as %s (minted from client %q)", minted, client)
	return minted
}

// sanitizeAgentName folds a free-form MCP clientInfo.name into a
// principal-safe segment: lowercased, runs of anything outside
// [a-z0-9._-] collapsed to one dash. An empty result falls back to
// "agent" — a nameless client still gets a unique principal.
func sanitizeAgentName(client string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(client) {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "agent"
	}
	return out
}

// newMCPServer builds the per-session server: the eleven T5 tools plus
// the lease-renewal machinery tied to session liveness. A non-empty
// client name means the principal is auto-derived: actor is the root
// human and the agent half is minted at session bind.
func (d *Daemon) newMCPServer(actor, client string) *mcp.Server {
	s := &mcpSession{
		actor:  actor,
		stop:   make(chan struct{}),
		claims: make(map[string]string),
	}
	if client != "" {
		s.actor = ""
		s.mint = func() string { return d.mintPrincipal(actor, client) }
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "tuhdoo", Version: d.version}, &mcp.ServerOptions{
		Instructions:              mcpInstructions,
		KeepAlive:                 d.mcpKeepAlive,
		KeepAliveFailureThreshold: mcpKeepAliveFailures,
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			ss := req.Session
			s.once.Do(func() {
				// Session bind: this runs once per real session (getServer
				// may build throwaway servers), so the minted principal is
				// fixed here. Tool handlers also call principal() as a
				// safety net for clients that skip initialized.
				s.principal()
				go d.renewSessionLeases(s)
				go func() {
					// Wait returns when the session ends: client
					// disconnect (DELETE), keepalive failure, or
					// daemon shutdown. Renewals stop; the leases
					// expire on their own and replay returns the
					// tasks to the pool (T5).
					_ = ss.Wait()
					close(s.stop)
					d.log.Printf("daemon: mcp: session for %s ended", s.principal())
				}()
			})
		},
	})
	d.addMCPTools(srv, s)
	return srv
}

// renewSessionLeases renews every claim this session holds, every
// LeaseTTL/3 (T8: 15 min TTL, renewed every 5), until the session ends.
func (d *Daemon) renewSessionLeases(s *mcpSession) {
	ticker := time.NewTicker(d.leaseTTL / 3)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			d.renewOnce(s)
		}
	}
}

// renewOnce re-checks each tracked claim under d.mu — still active,
// still held by this session's actor — renewing the live ones and
// dropping the rest from tracking (released elsewhere, expired, or
// lost to a cross-machine race).
func (d *Daemon) renewOnce(s *mcpSession) {
	claims := s.snapshot()
	if len(claims) == 0 {
		return
	}
	now := time.Now()
	var stale []string

	d.mu.Lock()
	if d.degraded != nil {
		// Fail-safe mode rejects writes; leases will lapse, which is
		// the honest outcome when the daemon cannot replay truthfully.
		d.mu.Unlock()
		return
	}
	renewed := false
	for task, claim := range claims {
		c := d.state.Claims[claim]
		if c == nil || c.Status != core.ClaimActive || c.Actor != s.principal() {
			stale = append(stale, task)
			continue
		}
		if err := d.store.WriteLease(claim, now.Add(d.leaseTTL)); err != nil {
			d.log.Printf("daemon: mcp: renew lease %s for %s: %v", claim, s.principal(), err)
			continue
		}
		renewed = true
	}
	if renewed {
		if err := d.refreshLocked(now); err != nil {
			d.log.Printf("daemon: mcp: refresh after renewal: %v", err)
		}
	}
	d.mu.Unlock()

	for _, task := range stale {
		s.untrack(task)
	}
}

// ---- tool input/output shapes ----
//
// Field descriptions are the agent's documentation: they surface in
// every tools/list response.

type getBacklogInput struct{}

type backlogResult struct {
	Ready []taskJSON `json:"ready" jsonschema:"claimable tasks: open, unclaimed, dependencies done, no blocking escalation; highest priority first"`
	Inbox []taskJSON `json:"inbox" jsonschema:"untriaged captures (status inbox), creation order — never served by claim_next/claim_task; promoting one to open means supplying a prompt-quality description first"`
	Held  []taskJSON `json:"held" jsonschema:"triaged but deliberately paused tasks (status held), creation order — never served by claim_next/claim_task; resume by setting status open"`
}

type getTaskInput struct {
	Task string `json:"task" jsonschema:"the task ID"`
}

type claimNextInput struct {
	Labels []string `json:"labels,omitempty" jsonschema:"only claim a task carrying every one of these labels; omit to take the best ready task"`
}

type claimNextResult struct {
	Claimed bool          `json:"claimed" jsonschema:"whether a task was claimed; false with a reason is a normal outcome, not an error"`
	Reason  string        `json:"reason,omitempty" jsonschema:"why nothing was claimed, when claimed is false"`
	Task    *hydratedTask `json:"task,omitempty" jsonschema:"the claimed task, fully hydrated, when claimed is true"`
}

type claimTaskInput struct {
	Task string `json:"task" jsonschema:"the task ID to claim"`
}

type releaseClaimInput struct {
	Task   string `json:"task" jsonschema:"the task whose claim to release"`
	Reason string `json:"reason" jsonschema:"why you are standing down; recorded on the ledger for the next claimant"`
}

type releaseClaimResult struct {
	Released string `json:"released" jsonschema:"the released claim ID"`
}

type finishRunInput struct {
	Task    string   `json:"task" jsonschema:"the task this run was for"`
	Outcome string   `json:"outcome" jsonschema:"done (acceptance criteria met), failed (attempted and did not work), abandoned (gave up), or blocked (waiting on an escalation answer — escalate and release_claim first)"`
	Branch  string   `json:"branch,omitempty" jsonschema:"branch the work lives on, if any"`
	PR      string   `json:"pr,omitempty" jsonschema:"pull/merge request link, if any; stored as a string, never dereferenced"`
	Commits []string `json:"commits,omitempty" jsonschema:"relevant commit hashes, if any"`
	Summary string   `json:"summary,omitempty" jsonschema:"what happened and where things stand — written for whoever picks this up next"`
}

type escalateInput struct {
	Task     string `json:"task" jsonschema:"the task the question is about"`
	Question string `json:"question" jsonschema:"the decision you need a human to make"`
	Context  string `json:"context,omitempty" jsonschema:"what you tried, what you found, and why you cannot decide alone — the human answers long after you are gone"`
	Blocking bool   `json:"blocking,omitempty" jsonschema:"true if the task cannot proceed until answered; a blocking escalation keeps the task out of the ready pool until a human answers"`
}

type relayAnswerInput struct {
	Escalation string `json:"escalation" jsonschema:"the escalation being answered — its ID, from the task's hydration"`
	Answer     string `json:"answer" jsonschema:"the answer as it was given — you are the scribe, not the decider"`
}

type addNoteInput struct {
	Task string `json:"task" jsonschema:"the task to annotate"`
	Text string `json:"text" jsonschema:"the checkpoint: a significant finding, the state before a risky step, or exactly where things stand — only what would save a successor real work"`
}

type eventIDResult struct {
	ID string `json:"id" jsonschema:"the recorded event's ID"`
}

type createTasksInput struct {
	Tasks []createTaskItem `json:"tasks" jsonschema:"the batch: a whole DAG (epic, children, dependency edges) using 'tmp:<name>' refs between items; it lands atomically or not at all; a single task is a batch of one"`
}

type createTasksResult struct {
	IDs []string          `json:"ids" jsonschema:"assigned task IDs, in batch order"`
	Tmp map[string]string `json:"tmp,omitempty" jsonschema:"tmp name to assigned task ID"`
}

type updateTaskInput struct {
	Task        string    `json:"task" jsonschema:"the task ID to update"`
	Title       *string   `json:"title,omitempty" jsonschema:"new title; omit to leave unchanged"`
	Description *string   `json:"description,omitempty" jsonschema:"new description; omit to leave unchanged"`
	Status      *string   `json:"status,omitempty" jsonschema:"new status: open, inbox, held, done, or cancelled; omit to leave unchanged. open<->held is pause/resume; inbox->open is promotion — supply a prompt-quality description with it (see the agent protocol)"`
	Priority    *int      `json:"priority,omitempty" jsonschema:"new priority; omit to leave unchanged"`
	Labels      *[]string `json:"labels,omitempty" jsonschema:"full replacement label list; omit to leave unchanged"`
	Parents     *[]string `json:"parents,omitempty" jsonschema:"full replacement parent-edge list (task IDs); omit to leave unchanged"`
	DependsOn   *[]string `json:"depends_on,omitempty" jsonschema:"full replacement dependency-edge list (task IDs); omit to leave unchanged"`
}

// ---- tool registration ----

// addMCPTools registers exactly the eleven T5 verbs. Op failures return
// as Go errors, which the SDK packs into the result with IsError set —
// a tool error the model can read and correct, never a protocol error.
// Additions to this list require a design-doc revision first.
func (d *Daemon) addMCPTools(srv *mcp.Server, s *mcpSession) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_backlog",
		Description: "The claimable backlog: open tasks with met dependencies and no active claim, " +
			"highest priority first — plus the inbox (untriaged captures) and held (deliberately " +
			"paused) shelves, which claim verbs never serve. Orientation only — use claim_next " +
			"to actually take work.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in getBacklogInput) (*mcp.CallToolResult, backlogResult, error) {
		ready, inbox, held, oe := d.opBacklog()
		if oe != nil {
			return nil, backlogResult{}, oe
		}
		return nil, backlogResult{Ready: ready, Inbox: inbox, Held: held}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_task",
		Description: "One task fully hydrated: description, edges, notes, runs, and escalations. " +
			"Read this before working — prior runs and notes are your memory of earlier attempts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in getTaskInput) (*mcp.CallToolResult, hydratedTask, error) {
		h, oe := d.opGetTask(in.Task)
		if oe != nil {
			return nil, hydratedTask{}, oe
		}
		return nil, h, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "claim_next",
		Description: "Atomically claim the best ready task and return it hydrated. " +
			"An empty pool returns claimed:false — a normal outcome, not an error. " +
			"The claim's lease auto-renews while this session stays connected.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in claimNextInput) (*mcp.CallToolResult, claimNextResult, error) {
		h, oe := d.opClaimNext(s.principal(), in.Labels)
		if oe != nil {
			return nil, claimNextResult{}, oe
		}
		if h == nil {
			return nil, claimNextResult{Claimed: false, Reason: "no ready task matches"}, nil
		}
		if h.Claim != nil {
			s.track(h.Task.ID, h.Claim.ID)
		}
		return nil, claimNextResult{Claimed: true, Task: h}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "claim_task",
		Description: "Claim one specific task (the human-directed path) and return it hydrated. " +
			"Fails if the task is already claimed, not open, or has unmet dependencies.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in claimTaskInput) (*mcp.CallToolResult, hydratedTask, error) {
		h, oe := d.opClaimTask(s.principal(), in.Task)
		if oe != nil {
			return nil, hydratedTask{}, oe
		}
		if h.Claim != nil {
			s.track(h.Task.ID, h.Claim.ID)
		}
		return nil, h, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "release_claim",
		Description: "Voluntarily stand down from a task you hold, returning it to the pool with " +
			"your reason on record. The reason is the handoff; add_note first only for " +
			"resume-state it cannot carry.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in releaseClaimInput) (*mcp.CallToolResult, releaseClaimResult, error) {
		claim, oe := d.opReleaseClaim(s.principal(), in.Task, in.Reason)
		if oe != nil {
			return nil, releaseClaimResult{}, oe
		}
		s.untrack(in.Task)
		return nil, releaseClaimResult{Released: claim}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "finish_run",
		Description: "Record how your attempt ended: outcome, links (branch/PR/commits), and a " +
			"summary for whoever comes next. Every claim should end in a finish_run or a release_claim.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in finishRunInput) (*mcp.CallToolResult, eventIDResult, error) {
		// Agents report their own verdicts only; "interrupted" and
		// "superseded" are daemon-synthesized (T5) and rejected here
		// even though the shared op accepts them for the HTTP surface.
		switch in.Outcome {
		case event.OutcomeDone, event.OutcomeFailed, event.OutcomeAbandoned, event.OutcomeBlocked:
		default:
			return nil, eventIDResult{}, opErrf(http.StatusBadRequest,
				"invalid outcome %q: agents report done, failed, abandoned, or blocked", in.Outcome)
		}
		id, oe := d.opFinishRun(s.principal(), finishRunReq{
			Task: in.Task, Outcome: in.Outcome, Branch: in.Branch,
			PR: in.PR, Commits: in.Commits, Summary: in.Summary,
		})
		if oe != nil {
			return nil, eventIDResult{}, oe
		}
		s.untrack(in.Task)
		return nil, eventIDResult{ID: id}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "escalate",
		Description: "Raise a question a human must answer. Answers usually land after your session " +
			"ends: for a blocking question, escalate, note where you stopped, release_claim, then " +
			"finish_run with outcome blocked — the next claimant inherits question and answer.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in escalateInput) (*mcp.CallToolResult, eventIDResult, error) {
		id, oe := d.opEscalate(s.principal(), escalateReq{
			Task: in.Task, Question: in.Question, Context: in.Context, Blocking: in.Blocking,
		})
		if oe != nil {
			return nil, eventIDResult{}, oe
		}
		return nil, eventIDResult{ID: id}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "relay_answer",
		Description: "Record an answer to an escalation that a human gave out of band — in your " +
			"own session rather than a steering surface. The answer is attributed to your root " +
			"principal; the ledger marks you as the relay. Open escalations only: a settled " +
			"answer can be amended only from a steering surface.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in relayAnswerInput) (*mcp.CallToolResult, eventIDResult, error) {
		id, oe := d.opRelayAnswer(s.principal(), in.Escalation, in.Answer)
		if oe != nil {
			return nil, eventIDResult{}, oe
		}
		return nil, eventIDResult{ID: id}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "add_note",
		Description: "Optionally checkpoint mid-flight context on a task: a significant finding, " +
			"the state before a risky step, where things stand at a stopping point. The typed " +
			"transitions (claim, finish_run, release_claim, escalate) carry the record; write " +
			"a note only when it would save a successor real work.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in addNoteInput) (*mcp.CallToolResult, eventIDResult, error) {
		id, oe := d.opAddNote(s.principal(), in.Task, in.Text)
		if oe != nil {
			return nil, eventIDResult{}, oe
		}
		return nil, eventIDResult{ID: id}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "create_task",
		Description: "Create tasks in one atomic batch — a whole plan (epic, children, dependency " +
			"edges) using tmp: refs between items, or a single task as a batch of one. Task " +
			"descriptions are prompts: include acceptance criteria, constraints, and file pointers " +
			"— except for status inbox, the chuck-it-in capture tier, where title-only is legitimate " +
			"and the prompt bar applies at promotion instead.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createTasksInput) (*mcp.CallToolResult, createTasksResult, error) {
		ids, tmp, oe := d.opCreateTasks(s.principal(), in.Tasks)
		if oe != nil {
			return nil, createTasksResult{}, oe
		}
		return nil, createTasksResult{IDs: ids, Tmp: tmp}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "update_task",
		Description: "Update a task's fields: status, priority, labels, edges, title, description. " +
			"Only the fields you send change.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateTaskInput) (*mcp.CallToolResult, taskJSON, error) {
		t, oe := d.opUpdateTask(s.principal(), in.Task, updateTaskReq{
			Title: in.Title, Description: in.Description, Status: in.Status,
			Priority: in.Priority, Labels: in.Labels, Parents: in.Parents, DependsOn: in.DependsOn,
		})
		if oe != nil {
			return nil, taskJSON{}, oe
		}
		return nil, t, nil
	})
}
