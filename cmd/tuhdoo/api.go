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
	Priority    *int      `json:"priority"`
	Labels      []string  `json:"labels"`
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

// updateJSON is one task edit: the actor and the compact per-field
// summaries the history surfaces render verbatim.
type updateJSON struct {
	ID     string   `json:"id"`
	Task   string   `json:"task"`
	Actor  string   `json:"actor"`
	Fields []string `json:"fields"`
}

// hydratedTask is one task with everything attached — the shape each
// snapshot entry embeds (and get_task serves), decoded losslessly.
type hydratedTask struct {
	Task        taskJSON         `json:"task"`
	Claim       *claimJSON       `json:"claim"`
	Notes       []noteJSON       `json:"notes"`
	Runs        []runJSON        `json:"runs"`
	Escalations []escalationJSON `json:"escalations"`
	Updates     []updateJSON     `json:"updates"`
}

// stateTask is one task's listing row — what the render code sorts,
// buckets, and prints — lifted from a snapshot entry by listingRow
// (snapshot.go). Not a wire shape of its own since the snapshot
// (T4, 2026-09-10): the entry carries the task and the verdicts side
// by side, and the row is the verdicts plus the task's headline.
type stateTask struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Priority *int     `json:"priority"`
	Labels   []string `json:"labels"`
	Holder   string   `json:"holder"`
	// One classifier (2026-08-03): the daemon serves core's verdict —
	// situation is ready / in_progress / blocked for open tasks, the
	// status word otherwise; the lists carry blocker IDs. The CLI
	// renders these, it never re-derives them.
	Situation           string   `json:"situation"`
	UnmetDeps           []string `json:"unmet_deps"`
	BlockingEscalations []string `json:"blocking_escalations"`
	// Loud blockage annotations (2026-08-05 edge grill): the unmet deps
	// sitting cancelled, and membership in a depends_on loop among
	// not-done tasks. The daemon's verdict like the lists above — the
	// CLI renders these, it never re-derives them.
	CancelledDeps []string `json:"cancelled_deps"`
	Cyclic        bool     `json:"cyclic"`
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

// stateResp is the snapshot's listing half: the daemon-wide facts plus
// one row per task, in creation (ULID) order. It was GET /v0/state's
// body until the snapshot replaced that endpoint (T4, 2026-09-10);
// now snapshotOf (snapshot.go) assembles it from a snapshotResp so the
// render code keeps one listing shape.
type stateResp struct {
	Degraded        string
	Sync            syncJSON
	Tasks           []stateTask
	OpenEscalations []escalationJSON
	Runs            []runJSON
}

// snapshotTask is one entry of GET /v0/snapshot: the hydrated task
// with its keys flattened alongside the listing's per-task verdicts
// (holder, situation, the blocker lists, the loud annotations) — the
// daemon's verdicts, never re-derived here.
type snapshotTask struct {
	hydratedTask
	Holder              string   `json:"holder"`
	Situation           string   `json:"situation"`
	UnmetDeps           []string `json:"unmet_deps"`
	BlockingEscalations []string `json:"blocking_escalations"`
	CancelledDeps       []string `json:"cancelled_deps"`
	Cyclic              bool     `json:"cyclic"`
}

// snapshotResp is GET /v0/snapshot?since=N&wait=D (T4, 2026-09-10):
// the whole replica at one version, every task fully hydrated. The
// daemon answers at once when its version differs from N — differs,
// not exceeds: a restarted daemon counts from 1 again — and otherwise
// parks until a bump or the wait elapses. Unchanged is true only on
// the wait-elapsed answer, which carries the version alone. Loaded is
// false only on the placeholder served until the first replay lands
// (version 0, sync mode "starting"); the first replay is version 1,
// so a request parked on since=0 wakes the moment the load lands.
type snapshotResp struct {
	Version         uint64           `json:"version"`
	Unchanged       bool             `json:"unchanged"`
	Loaded          bool             `json:"loaded"`
	Degraded        string           `json:"degraded"`
	Sync            syncJSON         `json:"sync"`
	Tasks           []snapshotTask   `json:"tasks"`
	OpenEscalations []escalationJSON `json:"open_escalations"`
	Runs            []runJSON        `json:"runs"`
}
