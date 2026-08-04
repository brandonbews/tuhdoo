# tuhdoo — Core Design Record

**Date:** 2026-07-28 (revised 2026-07-29 by Cycle 2 — see `002-technology.md`; revision notes inline)
**Status:** Founding decisions from the first design grilling. Held firmly, but revisable — edit in place and note revisions.

---

## What tuhdoo is

tuhdoo is a **coordination fabric for agent fleets, steered by humans** — a shared backlog, work queue, and activity ledger that lives in a git orphan branch inside the same repository as the application it plans. A team of people, each running a team of agents, works against one shared plan that is synced through an ordinary git remote with no other infrastructure.

The pain it targets: in 2026, a single developer with multiple worktrees and multiple agents is already a distributed system with no shared brain. A team of such developers is several uncoordinated brains. Nothing today connects *intent* (the backlog) to *fleet activity* (who claimed what, what happened, what needs a human) in a way that agents write to natively and humans can steer.

The soul of the product: **the project and its plan are the same asset.** Clone the repo, and the roadmap comes with it. No vendor, no server, no account. The plan never depends on which branch is checked out or what happened on `main` while you were planning.

## Founding principles

1. **Ownership.** The plan lives in the repo, travels with the repo, and belongs to whoever holds the repo. Anything requiring a hosted service beyond the git remote violates this.
2. **Convergence, not consensus.** Across the network, git gives us compare-and-swap on a ref and nothing else. tuhdoo never promises locks or exclusive assignment — it promises *deterministic reconciliation*: every machine, replaying the same events, reaches the same verdict. *(2026-08-04: the D6 confirmation gate is built on exactly that compare-and-swap — an irrevocable merge-time verdict won at the ref is the one exclusivity git can honestly promise. Claim-time stays lock-free; the principle stands.)*
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

### D5. Entities: five, DAG-shaped, nothing more *(revised 2026-07-31 — the status model grew inbox and held; note below)*

| Entity | Role |
|---|---|
| **Task** | Unit of intent: title, description, priority, status, labels, parent edges + dependency edges (a DAG — agents decompose recursively, not in tidy epic/story/subtask tiers). Milestones are just tasks other tasks point into. |
| **Claim** | "Agent X on machine Y is working on task Z," carrying a **lease** that expires unless renewed — a crashed agent's task returns to the pool automatically. |
| **Run** | The attempt record: claim → worktree/branch → commits/PR produced → outcome (`done` / `failed` / `abandoned` / `superseded` / `blocked` / `interrupted` — the last two added in Cycle 2: `blocked` = waiting on an escalation answer; `interrupted` = auto-closed by the daemon after lease expiry). The ledger that makes fleet activity legible after the fact — and, per Cycle 2, agent memory first: it's what lets a fresh agent resume a predecessor's work. |
| **Escalation** | An agent-raised question or blocker awaiting a human answer, attached to a task. The human's inbox; the steering half of the product. |
| **Note** | Comments/context on a task, from humans or agents. |

**Milestones are a label, not a mechanism** *(2026-08-03, milestone grill)*: the table's "milestones are just tasks other tasks point into" is literally true — the word appears in no Go file, and a milestone is simply a task carrying a `milestone` label, with no special casing anywhere. Its done-ness is **declared**, exactly like any other task's; computed done-ness was considered and rejected (`open-questions.md`, Cycle 3 resolution). A milestone that should not be worked is parked with `held` — never fenced with a blocking escalation, per the fence rule in `docs/agent-protocol.md`.

**Deliberately excluded from v1:** sprints/cycles, custom fields and configurable workflows (Jira disease), docs/wiki, time tracking, OKRs. Each is a possible bolt-on *if* real usage demands it; each included now permanently doubles the schema burden of an append-only log.

**The status model** *(revised 2026-07-31, grill cycle — inbox and held join open/done/cancelled)*: five statuses — `open`, `inbox`, `held`, `done`, `cancelled`. The pressure this relieves: a queue that prices every entry at commission rates (prompt-quality description, priority, scope) forces idea capture into premature quantification, so ideas either don't get captured or get captured badly. The two additions split "not being worked" into its two honest kinds:

- **`inbox`** — never triaged; the chuck-it-in tier. Capture minimum is title-only; fragment descriptions are legitimate **for inbox items only**; carries inherent review debt by definition.
- **`held`** — passed triage, workable, deliberately paused. Absorbs and kills the parked-`p0`-label convention.

`open` remains the only status the claim verbs ever serve. Transitions are **mechanically permissive** — the deterministic core validates status vocabulary but never transition paths (no rejected-event edge cases; see `002` T3) — with the semantics carried by `docs/agent-protocol.md`: promote inbox→open by supplying a prompt-quality description; pause/resume is open↔held; anyone (human or agent) may promote — the quality bar is documentation, not schema. Inbox/held items are ordinary shared state on the data branch: labels and edges are allowed at capture (dependencies on inbox/held tasks block naturally — they are not `done`), and priority is stored but inert until `open`. `cancelled` is displayed as cancelled — the archive porcelain word was retired *(see `002`, status-vocabulary revision 2026-08-01)* — and cancel works on every non-terminal status, which is what makes cheap capture reversible.

### D6. Claim semantics: optimistic claims, refereed verdicts *(revised 2026-08-04, confirmation-gate grill — the final verdict moves from claim mint-time to a CAS-anchored confirmation; clauses amended in place, notes inline)*

Nothing prevents two machines claiming the same task inside the sync window — claiming stays optimistic, instant, and offline-capable. The 2026-08-04 revision separates the **provisional** verdict (who should stand down, computed from claim ULIDs) from the **final** verdict (who may merge and record `done`, decided by winning a compare-and-swap push at the remote). The reasoning: any rule keyed to when a claim was *minted* is revocable — an earlier-minted claim can always still be in flight, so no amount of waiting makes "you win" final — while the remote serializes pushes, so a verdict anchored to a successful push can never be unseated. Priority order for this mechanism, each level exhausted before the next: (1) **accuracy** — exactly one winner at any scale; a duplicate certified outcome is a bug at any probability, never an accepted tail; (2) **token protection** — waste is bounded best-effort and always recorded, never hidden; (3) **speed** — a 5–30 s refereeing round-trip at the merge/finish moment is accepted; everything else stays fast.

1. **Provisional winner rule** *(demoted from final, 2026-08-04)*: claims carry ULIDs; on merge, the earliest claim provisionally holds the task and later claims are voided by replay — automatically and identically on every machine. Provisional state drives leases, stand-down warnings, and display; it is advisory and revocable until a confirmation exists. *(The original "machine-id tiebreak" is deleted as vacuous, not demoted: ULIDs are minted from monotonic crypto randomness and structurally never tie, so the tiebreak branch never existed in code — collision-harness finding, 2026-08-03.)*
2. **The confirmation gate: the final verdict is won, not computed** *(added 2026-08-04)*. Before work may be merged or recorded `done`, its claim must be **confirmed**: the daemon examines the remote head it is about to push onto, and only if that state shows no competing confirmation for the task and this claim as the provisional winner does it push a `claim.confirmed` event onto exactly that head. The push is git's atomic compare-and-swap on the branch ref — the remote serializes all comers — and the app-level merge/push path (T2) refuses to carry a second confirmation for a task that already has one, so **at most one confirmation per task can ever land**, by construction, not by probability. Replay's rule is one line: a confirmed claim wins unconditionally. The gate runs in two places (T5): the explicit `confirm_claim` verb, for the moment before an agent merges or arms auto-merge, and **inside every `done`-recording path** (`finish_run` and the HTTP run surface), so even an agent that skips the verb gets refereed — a certified `done` means the referee confirmed the winner, not that an agent believed it finished. With no remote configured, the daemon is the sole writer and confirmation is locally sound and instant (T2: remoteless is a normal state). With a configured remote unreachable, the gate **refuses honestly** with a retryable error — it never guesses (accuracy outranks liveness).
3. **Loser handling** *(rewritten 2026-08-04 — the original clause promised a stand-down push and a daemon-known branch name; neither was implementable, and nothing implemented it: the collision harness found the `superseded` shape designed for, guarded for, and written by no one)*. Losers learn at verb-time — there is no push channel to a working agent: every claim response carries the warning to confirm before merging, and any verb touched by a provisionally-voided claimant says plainly that it lost. **The daemon is the referee of how attempts ended**: a `finish_run` on a lost attempt is coerced to outcome `superseded`, keeping the agent's reported branch/PR/commits/summary as the salvage record — the branch name is knowable only to the losing agent, so the honest design routes the record through its own report. A loser that never reports leaves a trace anyway: when a voided claim's lease expires unclosed, replay synthesizes a branch-less `superseded` run (the same mechanism as `interrupted`), and the attempt is thereafter closable no longer — one close per attempt, deterministic at every instant. A late-returning loser is told its attempt is closed and pointed at `add_note` for salvage.
4. **No claim-time prevention machinery** *(scope narrowed 2026-08-04 — this clause previously read as "no prevention machinery" absolutely; the confirmation gate is a merge-time verdict, not claim-time prevention, and the distinction is deliberate)*: claims never wait, lock, or coordinate; agent-hours are cheap and rare duplicate *attempts* cost less than any claim-time coordination layer. Mitigations against wasted attempts stay: eager push on claim and confirmation events, short fetch intervals, verb-time stand-down. What is *not* accepted at any rate is a duplicate **merge** or duplicate certified `done` by protocol-following agents — that is what the gate exists to make impossible rather than improbable.
5. **Leases:** renewable, session-bound (the daemon auto-renews while the claiming MCP session stays connected — agents never heartbeat manually); expiry is evaluated at read time by replay logic — no reaper process. Cycle 2 set TTL 15 min / renewal 5 min (see `002` T8, superseding the ~30 min sketch here).

**Accepted consequences** *(2026-08-04)*:

- **The guarantee is bounded by the protocol, not the repo.** tuhdoo has no authority over the host repo: an actor that merges without ever calling the gate is outside any coordination system's reach — the same trust-boundary shape as D7, and the same one a lock server has (nothing stops a client that bypasses the lock). The promise, exactly: *any agent following the protocol — one verb call — can never duplicate-merge or double-certify, at any probability.*
- **Residual token waste exists and is disclosed, never prevented at claim time.** A loser may burn a full attempt before its next verb call. The superseded run *is* the disclosure mechanism.
- **The remote is the serialization point.** Accuracy holds at any scale; confirmation *throughput* degrades under extreme push contention (retries) — the correct trade under the priority order.
- **`finish_run(done)` gains a sync round-trip** (5–30 s with a remote). Already-confirmed claims skip it; confirmation is idempotent.
- **A new event type (`claim.confirmed`) rides T3's fail-safe**: peers running older binaries go read-only ("upgrade tuhdoo") once the first confirmation lands. Upgrade every machine's binary before the gate ships events in a mixed fleet.

### D7. Identity: hierarchical principals, git-derived, unsigned

- Humans are root actors (`brandon`); agents are sub-principals operating under a human's authority (`brandon/impl-2`). Every event carries its actor; every agent action traces to a responsible human.
- Human identity derives from git identity (`user.email`, as the host already trusts). Agent names are assigned in daemon config when a harness connects over MCP.
- No cryptographic signing in v1 (trust boundary is repo-write; forgery is exactly as possible as forging git commit authors today). The event envelope reserves a `sig` field so signing can arrive later without a schema break.

### D8. Surfaces and build order *(revised by Cycle 2 — details in `002` T7; revised 2026-07-30 by Cycle 4 — one verb-less TUI)*

1. **v0 — daemon + MCP + CLI.** The daemon is the kernel (sole writer, sync loop, replay, view builder); the MCP server lives inside it (one process) and is the front door for every harness. The CLI is the **local human's primary portal**, not a thin veneer: `status`, `backlog`, `task <id>`, `escalations`, and a live dashboard pane that sits beside a working agent *(v0 shipped that pane as read-only `watch`; Cycle 4 folded it into the TUI as `tuhdoo --watch` — see `002` T7)*. Markdown views serve the *remote/browsing* audience, not local steering.
2. **Free surface — the git host.** The D3 markdown views make GitHub/GitLab a zero-effort read-only UI: browsable backlog, linkable tasks, per-task history via `git log`.
3. **v1 — the steering surface is a TUI** (in the same binary, works over SSH/tmux where the author lives), grown from `watch` by adding interactivity: escalation inbox, fleet ledger, queue reordering. *(Cycle 4: the grown TUI is verb-less — bare `tuhdoo`, with `--watch` as the disarmed mode; `top` was a transient name.)* Browser UI demoted to v2+.
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
- **Logical timestamp** — ULID-based ordering, used for replay order and the *provisional* winner rule; wall clocks across machines are never trusted. Final claim verdicts are CAS-anchored, not timestamp-based (D6, 2026-08-04).
- **Confirmation** — the `claim.confirmed` event that makes a claim's verdict final and irrevocable; won by a compare-and-swap push at the remote, at most one per task by construction (D6, 2026-08-04).
- **Epoch compaction** — snapshot + in-commit deletion of superseded event files: shrinks the live tree without rewriting history.
- **Superseded run** — the recorded remains of a claim that lost a cross-machine race, preserved for salvage: with branch pointer when the loser reported back, branch-less when synthesized at lease expiry (D6, 2026-08-04).
