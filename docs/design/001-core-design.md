# tuhdoo — Core Design Record

**Date:** 2026-07-28
**Status:** Founding decisions from the first design grilling. Held firmly, but revisable — edit in place and note revisions.

---

## What tuhdoo is

tuhdoo is a **coordination fabric for agent fleets, steered by humans** — a shared backlog, work queue, and activity ledger that lives in a git orphan branch inside the same repository as the application it plans. A team of people, each running a team of agents, works against one shared plan that is synced through an ordinary git remote with no other infrastructure.

The pain it targets: in 2026, a single developer with multiple worktrees and multiple agents is already a distributed system with no shared brain. A team of such developers is several uncoordinated brains. Nothing today connects *intent* (the backlog) to *fleet activity* (who claimed what, what happened, what needs a human) in a way that agents write to natively and humans can steer.

The soul of the product: **the project and its plan are the same asset.** Clone the repo, and the roadmap comes with it. No vendor, no server, no account. The plan never depends on which branch is checked out or what happened on `main` while you were planning.

## Founding principles

1. **Ownership.** The plan lives in the repo, travels with the repo, and belongs to whoever holds the repo. Anything requiring a hosted service beyond the git remote violates this.
2. **Convergence, not consensus.** Across the network, git gives us compare-and-swap on a ref and nothing else. tuhdoo never promises locks or exclusive assignment — it promises *deterministic reconciliation*: every machine, replaying the same events, reaches the same verdict.
3. **Online is the posture; offline is graceful degradation.** Users are assumed connected. Solo use works fully offline (everything is local); team use with the network off is out of scope as a good experience.
4. **Git's honor system is our honor system.** The trust boundary is repo write access, exactly as it is for code. Attribution is honest-actor bookkeeping, not cryptographic security.
5. **Append-only tree semantics, forever.** History rewriting and force-pushes on the data branch are forbidden as a hard product invariant — divergent histories with no common ancestor cannot merge, and every peer breaks at once.

---

## Decisions

### D1. Product identity: agent substrate, human-steered

tuhdoo is **not** a human-first PM tool that agents can read (that fight is against Linear, GitHub Projects, and Jira, all of which already ship MCP — unwinnable on polish, and the exact shape of the burned POCs). It is an **agent work-substrate**: the queue agents claim work from, the ledger where they record what they did, and the escalation channel through which they ask humans for decisions. Humans set intent and priority, review outcomes, and answer escalations — they steer; they don't data-enter.

*Why this fixes the POC failure:* the POCs were human-first dashboards whose value only materializes at the agent-fleet use case. The design center is the agent loop — **claim → work → record → escalate** — and the human UI is a view over that loop, not the product itself.

### D2. Topology: remote-only across the network, serialized within a machine

- **Network layer:** the git remote (GitHub/GitLab/any) is the *only* shared infrastructure. No tuhdoo server, no hosted coordinator, ever. Cross-machine writes are truly concurrent and reconcile by merge.
- **Machine layer:** a local daemon (localhost) is the **sole writer** to the orphan branch on its machine. Agents and UIs never run `git commit` against the data branch themselves — they submit intents to the daemon over MCP/HTTP, and it applies them one at a time. Multiplayer begins on one machine (N agents, one repo); the daemon makes that layer transactional. Everything else about git usage — feature branches, `main`, normal agent commits to code — is untouched.

*Accepted consequences:* cross-machine sync is eventually consistent on git-push timescales; the data model must merge automatically by construction; no locks or transactions exist across machines.

### D3. Data model: event log as truth, derived readable views

- **Source of truth:** an append-only event log — one immutable file per event (`task-created`, `status-changed`, `claim-made`, …). Files are only ever *added*, so cross-machine git merges are set-union and structurally cannot conflict. Current state is computed by deterministic replay.
- **Derived views:** after every write/merge, the daemon regenerates human-readable markdown views (backlog, per-task pages) and commits them alongside the log. View regeneration is deterministic over the merged event set, so all machines converge to byte-identical views; any merge conflict in a view file is resolved by discarding both sides and rebuilding. This preserves the ownership soul — browsing the orphan branch on GitHub shows the actual roadmap.

*Accepted consequences:* the daemon is real software (replay engine, view builder, schema versioning); every write lands twice (log + views), which raises volume — addressed by D10.

### D4. Storage: orphan branch, committer-team audience

The orphan branch beats the alternatives: an in-tree directory ties plan state to the checked-out branch (disqualified by the founding pain), and a sibling repo breaks the one-asset soul. Accepted consequences, explicitly:

1. **Plan-write = code-write.** Git hosts have no per-branch permissions worth relying on. Anyone who can edit the backlog can push code. There is no "PM without repo access" persona — and this is honest, because agent teams already hold repo-write.
2. **No drive-by intake.** Contributors without write access cannot file into tuhdoo. It is the maintainer/committer team's planning brain, not a public issue tracker. Public intake (e.g., a bridge from GitHub Issues) is a possible later add-on, never a core capability.
3. **Host friction is onboarding, not architecture.** CI triggers firing on data-branch pushes, branch-protection rules blocking the daemon — all mitigated by a good `tuhdoo init` and docs.

### D5. Entities: five, DAG-shaped, nothing more

| Entity | Role |
|---|---|
| **Task** | Unit of intent: title, description, priority, status, labels, parent edges + dependency edges (a DAG — agents decompose recursively, not in tidy epic/story/subtask tiers). Milestones are just tasks other tasks point into. |
| **Claim** | "Agent X on machine Y is working on task Z," carrying a **lease** that expires unless renewed — a crashed agent's task returns to the pool automatically. |
| **Run** | The attempt record: claim → worktree/branch → commits/PR produced → outcome (`done` / `failed` / `abandoned` / `superseded`). The ledger that makes fleet activity legible after the fact. |
| **Escalation** | An agent-raised question or blocker awaiting a human answer, attached to a task. The human's inbox; the steering half of the product. |
| **Note** | Comments/context on a task, from humans or agents. |

**Deliberately excluded from v1:** sprints/cycles, custom fields and configurable workflows (Jira disease), docs/wiki, time tracking, OKRs. Each is a possible bolt-on *if* real usage demands it; each included now permanently doubles the schema burden of an append-only log.

### D6. Claim semantics: optimistic, deterministic, no prevention machinery

Nothing prevents two machines claiming the same task inside the sync window — only resolution is possible:

1. **Winner rule:** claims carry logical timestamps (ULID + machine-id tiebreak; wall clocks are never trusted across machines). On merge, **earliest claim wins**; the later claim is voided by replay logic — automatically and identically on every machine.
2. **Loser handling:** the losing daemon tells its agent to stand down; half-done work is recorded as a Run with outcome `superseded` (branch name included) so it can be salvaged, not mystery-wasted.
3. **No prevention machinery in v1:** agent-hours are cheap; rare duplicate attempts cost less than any coordination layer. Mitigations are: eager push on claim events, short fetch intervals, and advisory `assignee` as a soft filter. Measure real collision rates before building anything cleverer.
4. **Leases:** renewable heartbeats (order of ~30 min); expiry is evaluated at read time by replay logic — no reaper process.

### D7. Identity: hierarchical principals, git-derived, unsigned

- Humans are root actors (`brandon`); agents are sub-principals operating under a human's authority (`brandon/impl-2`). Every event carries its actor; every agent action traces to a responsible human.
- Human identity derives from git identity (`user.email`, as the host already trusts). Agent names are assigned in daemon config when a harness connects over MCP.
- No cryptographic signing in v1 (trust boundary is repo-write; forgery is exactly as possible as forging git commit authors today). The event envelope reserves a `sig` field so signing can arrive later without a schema break.

### D8. Surfaces and build order

1. **v0 — daemon + MCP + CLI.** The daemon is the kernel (sole writer, sync loop, replay, view builder); the MCP server lives inside it (one process) and is the front door for every harness; the CLI is a thin veneer on the same API (`init`, `status`, `escalations`). v0 is fully usable by a solo power user who steers via the generated markdown views.
2. **Free surface — the git host.** The D3 markdown views make GitHub/GitLab a zero-effort read-only UI: browsable backlog, linkable tasks, per-task history via `git log`.
3. **v1 — the steering UI.** Scoped strictly to steering: **escalation inbox, fleet ledger (what are/were my agents doing), queue reordering.** One dense page is acceptable.
4. **No kanban board until the steering loop is proven.** The pretty board is the siren song that sank the POCs; it is product-(a) UI and competes with Linear on Linear's terms.

Dogfooding order matters: living with the MCP surface first is what reveals what the UI must actually show.

### D9. Lifecycle: keeping the branch from becoming a landfill

1. **Commit granularity:** the daemon batches events into a commit every few seconds of activity — except claim events, which push eagerly (shrinks the D6 race window).
2. **Tree layout:** event files are date-sharded (`events/2026/07/28/<ulid>.json`); no directory ever exceeds a day's traffic.
3. **Lease hygiene:** heartbeats are junk mail — they live as *mutable* per-claim files in a `leases/` area (safe: only the owning machine's daemon touches its own agents' leases), keeping the permanent log free of ~80% of projected volume.
4. **Epoch compaction:** periodically (milestone close, or by age) the daemon writes a **snapshot event** ("full state as of X") and *deletes* superseded event files **in an ordinary commit**. The tree shrinks; git history retains everything for forensics; replay starts from the latest snapshot; the commit merges like any other. Append-only *tree semantics*, not append-only *tree contents*.
5. **Force-push on the data branch is forbidden, permanently** (founding principle 5).

### D10. Online-mostly posture

All users are assumed to be running frontier models and to be online; sync cadence is tuned for connected operation, making claim collisions genuinely rare rather than merely tolerable. Solo use degrades gracefully to fully-offline (everything is local; there is nobody to conflict with). Team use offline is explicitly not a supported good experience. Note: being always-online never buys locks — two daemons can still push in the same second, so all D6 machinery remains mandatory; connectivity changes collision *frequency*, not the need for resolution.

### D11. First user and competitive position

- **First user: solo developers running multi-worktree, multi-agent setups** (user #1 is the author). Adoption is a personal decision, not a group negotiation; v0 serves them completely; "single-player multiplayer" (one machine, many agents) delivers value with zero team adoption. Teams are the v1 story — a specific 5-person team at work is the eventual landing zone and the definition of "v1 done," but not a day-1 requirement.
- **Competitive landscape (as of mid-2026):** the nearest neighbor is **beads** (Steve Yegge) — a git-backed, agent-first issue tracker with real traction. Also adjacent: GitHub's Copilot agent task orchestration, harness-native task lists, and MCP servers from every PM incumbent.
- **Differentiation:** (1) the **multiplayer consensus story** — daemon-serialized machine-local writes plus deterministic cross-machine claim resolution and leases, i.e., a real answer for fleets across machines; (2) **Runs and Escalations as first-class** — a steering ledger, not just an issue list; (3) the orphan-branch model — data travels with the repo but never touches working branches.

---

## Glossary

- **Daemon** — the per-machine localhost process that is the sole writer to the data branch, serves MCP/HTTP/CLI, runs the sync loop, replays events, and builds views.
- **Serialization** — forcing concurrent writers into a single-file line: all local writes route through the daemon and apply one at a time, eliminating local races.
- **Convergence** (vs. consensus) — no coordination happens up front across machines; divergent histories merge and deterministic replay guarantees every machine reaches the same state.
- **Logical timestamp** — ULID-based ordering with machine-id tiebreak, used for winner rules; wall clocks across machines are never trusted.
- **Epoch compaction** — snapshot + in-commit deletion of superseded event files: shrinks the live tree without rewriting history.
- **Superseded run** — the recorded remains of a claim that lost a cross-machine race, preserved (with branch pointer) for salvage.
