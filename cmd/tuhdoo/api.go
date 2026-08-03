package main

// Mirrors of the daemon's JSON response shapes (internal/daemon/api.go).
// Those types are unexported in the daemon package, so the CLI declares
// its own; unknown fields added later are ignored by decoding, which is
// the additive-first contract working in our favor.

import "time"

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
	// Close metadata (history view, 2026-08-02): set on done and
	// cancelled tasks only.
	ClosedAt *time.Time `json:"closed_at"`
	ClosedBy string     `json:"closed_by"`
}

type claimJSON struct {
	ID      string     `json:"id"`
	Task    string     `json:"task"`
	Actor   string     `json:"actor"`
	Machine string     `json:"machine"`
	MadeAt  time.Time  `json:"made_at"`
	Expires *time.Time `json:"expires"`
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
	RelayedBy  string    `json:"relayed_by"`
}

type noteJSON struct {
	ID      string    `json:"id"`
	Task    string    `json:"task"`
	Actor   string    `json:"actor"`
	Text    string    `json:"text"`
	AddedAt time.Time `json:"added_at"`
}

// hydratedTask is GET /v0/tasks/{id}: one task with everything attached.
type hydratedTask struct {
	Task        taskJSON         `json:"task"`
	Claim       *claimJSON       `json:"claim"`
	Notes       []noteJSON       `json:"notes"`
	Runs        []runJSON        `json:"runs"`
	Escalations []escalationJSON `json:"escalations"`
}

type stateTask struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Priority int      `json:"priority"`
	Labels   []string `json:"labels"`
	Holder   string   `json:"holder"`
	// Close metadata (history view, 2026-08-02): what the history rows
	// sort and stamp by; nil on open tasks and pre-upgrade daemons.
	ClosedAt *time.Time `json:"closed_at"`
	ClosedBy string     `json:"closed_by"`
}

// syncJSON is the sync loop's health (B7). Mode is one of local-only,
// syncing, error, or starting (first cycle not finished yet).
type syncJSON struct {
	Mode       string `json:"mode"`
	Remote     string `json:"remote"`
	LastFetch  string `json:"last_fetch"` // RFC3339
	LastPush   string `json:"last_push"`
	LastError  string `json:"last_error"`
	Collisions int    `json:"collisions"`
	Merges     int    `json:"merges"`
}

// stateResp is GET /v0/state. Tasks arrive in creation (ULID) order.
type stateResp struct {
	Degraded        string           `json:"degraded"`
	Sync            syncJSON         `json:"sync"`
	Tasks           []stateTask      `json:"tasks"`
	OpenEscalations []escalationJSON `json:"open_escalations"`
	Runs            []runJSON        `json:"runs"`
}
