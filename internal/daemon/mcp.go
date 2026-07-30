package daemon

// The MCP surface (002 T5): streamable HTTP on the same unix socket,
// exactly ten tools, projected from the same ops.go operations as the
// HTTP API. One *mcp.Server is minted per session so tool closures can
// capture the session's actor (bound from the X-Tuhdoo-Actor header on
// the initialize POST) and its claim set for lease auto-renewal —
// session liveness, not agent diligence, is what keeps leases alive
// (T5: no heartbeat verb).

import (
	"context"
	"net/http"
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
	"The loop: claim_next (or claim_task) to take work, add_note to checkpoint findings " +
	"as you go — notes are letters to the next agent on this task — escalate when a human " +
	"must decide, and always end with finish_run (or release_claim to stand down). " +
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
	actor string
	stop  chan struct{}
	once  sync.Once // guards the goroutine pair spawn

	mu     sync.Mutex
	claims map[string]string // task ID → claim ID
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

// mcpHandler mounts the streamable HTTP endpoint. The getServer
// callback runs once per session — the first POST arrives without an
// Mcp-Session-Id header — which is exactly where the actor binds.
func (d *Daemon) mcpHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		actor := r.Header.Get(actorHeader)
		if err := ValidateActor(actor); err != nil {
			d.log.Printf("daemon: mcp: rejecting session: %v", err)
			return nil // the SDK serves a 400
		}
		return d.newMCPServer(actor)
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

// newMCPServer builds the per-session server: the ten T5 tools plus
// the lease-renewal machinery tied to session liveness.
func (d *Daemon) newMCPServer(actor string) *mcp.Server {
	s := &mcpSession{
		actor:  actor,
		stop:   make(chan struct{}),
		claims: make(map[string]string),
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "tuhdoo", Version: d.version}, &mcp.ServerOptions{
		Instructions:              mcpInstructions,
		KeepAlive:                 d.mcpKeepAlive,
		KeepAliveFailureThreshold: mcpKeepAliveFailures,
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			ss := req.Session
			s.once.Do(func() {
				go d.renewSessionLeases(s)
				go func() {
					// Wait returns when the session ends: client
					// disconnect (DELETE), keepalive failure, or
					// daemon shutdown. Renewals stop; the leases
					// expire on their own and replay returns the
					// tasks to the pool (T5).
					_ = ss.Wait()
					close(s.stop)
					d.log.Printf("daemon: mcp: session for %s ended", s.actor)
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
		if c == nil || c.Status != core.ClaimActive || c.Actor != s.actor {
			stale = append(stale, task)
			continue
		}
		if err := d.store.WriteLease(claim, now.Add(d.leaseTTL)); err != nil {
			d.log.Printf("daemon: mcp: renew lease %s for %s: %v", claim, s.actor, err)
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

type addNoteInput struct {
	Task string `json:"task" jsonschema:"the task to annotate"`
	Text string `json:"text" jsonschema:"the checkpoint: findings, decisions, exactly where you stopped — a letter to the next agent on this task"`
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
	Status      *string   `json:"status,omitempty" jsonschema:"new status: open, done, or cancelled; omit to leave unchanged"`
	Priority    *int      `json:"priority,omitempty" jsonschema:"new priority; omit to leave unchanged"`
	Labels      *[]string `json:"labels,omitempty" jsonschema:"full replacement label list; omit to leave unchanged"`
	Parents     *[]string `json:"parents,omitempty" jsonschema:"full replacement parent-edge list (task IDs); omit to leave unchanged"`
	DependsOn   *[]string `json:"depends_on,omitempty" jsonschema:"full replacement dependency-edge list (task IDs); omit to leave unchanged"`
}

// ---- tool registration ----

// addMCPTools registers exactly the ten T5 verbs. Op failures return as
// Go errors, which the SDK packs into the result with IsError set — a
// tool error the model can read and correct, never a protocol error.
// Additions to this list require a design-doc revision first.
func (d *Daemon) addMCPTools(srv *mcp.Server, s *mcpSession) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_backlog",
		Description: "The claimable backlog: open tasks with met dependencies and no active claim, " +
			"highest priority first. Orientation only — use claim_next to actually take work.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in getBacklogInput) (*mcp.CallToolResult, backlogResult, error) {
		ready, oe := d.opBacklog()
		if oe != nil {
			return nil, backlogResult{}, oe
		}
		return nil, backlogResult{Ready: ready}, nil
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
		h, oe := d.opClaimNext(s.actor, in.Labels)
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
		h, oe := d.opClaimTask(s.actor, in.Task)
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
			"your reason on record. add_note first if you learned anything worth passing on.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in releaseClaimInput) (*mcp.CallToolResult, releaseClaimResult, error) {
		claim, oe := d.opReleaseClaim(s.actor, in.Task, in.Reason)
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
		id, oe := d.opFinishRun(s.actor, finishRunReq{
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
		id, oe := d.opEscalate(s.actor, escalateReq{
			Task: in.Task, Question: in.Question, Context: in.Context, Blocking: in.Blocking,
		})
		if oe != nil {
			return nil, eventIDResult{}, oe
		}
		return nil, eventIDResult{ID: id}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "add_note",
		Description: "Checkpoint an observation on a task: after significant findings, before risky " +
			"changes, at any stopping point. Notes outlive your session — they are how the next " +
			"agent resumes instead of re-deriving your work.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in addNoteInput) (*mcp.CallToolResult, eventIDResult, error) {
		id, oe := d.opAddNote(s.actor, in.Task, in.Text)
		if oe != nil {
			return nil, eventIDResult{}, oe
		}
		return nil, eventIDResult{ID: id}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "create_task",
		Description: "Create tasks in one atomic batch — a whole plan (epic, children, dependency " +
			"edges) using tmp: refs between items, or a single task as a batch of one. Task " +
			"descriptions are prompts: include acceptance criteria, constraints, and file pointers.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createTasksInput) (*mcp.CallToolResult, createTasksResult, error) {
		ids, tmp, oe := d.opCreateTasks(s.actor, in.Tasks)
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
		t, oe := d.opUpdateTask(s.actor, in.Task, updateTaskReq{
			Title: in.Title, Description: in.Description, Status: in.Status,
			Priority: in.Priority, Labels: in.Labels, Parents: in.Parents, DependsOn: in.DependsOn,
		})
		if oe != nil {
			return nil, taskJSON{}, oe
		}
		return nil, t, nil
	})
}
