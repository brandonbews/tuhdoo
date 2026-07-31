# tuhdoo — Technology Decisions

**Date:** 2026-07-29
**Status:** Cycle 2 grilling outcomes. Builds on `001-core-design.md`; where the two touch, this doc is the more specific authority and `001` carries a revision note.

---

## T1. Daemon language: Go

The daemon is written in Go. Criteria, ranked for the agentic era (agents write most code; the human steers by reviewing): **distribution** (single static binary per platform — `brew install tuhdoo` with zero runtime dependency is the strongest v0 install story), **reviewability by the steering human** (Go is designed to be read by people who didn't write it; the author is a TS dev deliberately learning Go, and review-reading ramps far faster than writing), **agent competence** (large corpus, agents write it well), **ecosystem** (official MCP Go SDK; best-in-class TUI ecosystem — see T7).

- TypeScript was the real rival (MCP's first-class citizen, author's home language, one language with a future web UI) and lost narrowly on install story and daemon-grade auditability.
- Rust dismissed for this phase: hardest for the author to review, slowest to iterate, and v0's problem is product discovery, not performance.

**Accepted risk:** the author's steering power is reduced for the first weeks of Go reading. Mitigations, adopted as project law:

1. **Boring Go only.** Written convention: no clever concurrency (no channels-of-channels, no sync gymnastics); plain mutexes, plain loops — code a learner can audit.
2. **The deterministic core is pure functions.** Replay, winner rules, lease expiry, compaction, and view generation take data in and return data out — no I/O — and carry table-driven tests ("this event set produces exactly this state/bytes"). Correctness rests on reviewable tests, not line-by-line vigilance.

## T2. Git access: subprocess plumbing, no checkout, app-level merges

- **Subprocess `git`** (not go-git, not libgit2 bindings, not Dolt-style): fetch/push inherit the user's entire auth reality (SSH agents, credential helpers, `gh` tokens, proxies) for free, and the daemon takes no third-party correctness risk on the one job that must never corrupt data. go-git's auth story and long-tail correctness gaps disqualify it; libgit2 Go bindings are unmaintained; Dolt-style (beads' `refs/dolt/data`) pays a heavy dependency and loses host-browsability to buy cell-level merges our event log doesn't need.
- **The data branch is never checked out.** Writes use plumbing: `hash-object` → `mktree` → `commit-tree` → atomic `update-ref`. Reads use `cat-file` / `ls-tree`. No worktree, ever.
- **Merges are application logic, not git machinery.** Divergent data-branch histories merge by set-union of event files + deterministic view regeneration; the daemon computes the merged tree itself and writes a two-parent merge commit via `commit-tree`. `git merge`, conflict markers, and merge drivers are never invoked.
- The daemon operates **directly on the user's `.git`** (shared object store, atomic ref updates, tuhdoo-owned refs) — no shadow clone.
- The data lives at a **normal visible branch** (`refs/heads/tuhdoo`), not a hidden custom ref — host browsability (D4) requires it.
- All git access goes through a small internal `Git` interface so a library could slot in later without a rewrite. Minimum supported git version documented (target: ≥ 2.40).

**Host-agnostic by construction (named constraint).** tuhdoo speaks the git protocol and nothing else: any remote works (GitHub, GitLab, Bitbucket, Gitea, a bare repo on a NAS or USB drive), and **no remote** works too — absent a remote, the sync loop simply doesn't run and everything else is fully live; adding a remote later is detected automatically and the first push publishes the branch, no migration. The daemon never calls a host API; Run→PR links are stored strings, never dereferenced. Host-specific features (issue-intake bridges, webhook-driven fetch) are forever optional add-ons, never core. `tuhdoo init` must not assume a remote; `status` shows `local-only` as a normal state, not an error.

## T3. Event format: canonical JSON, versioned envelope, fail-safe degradation

**Serialization.** One event = one file (mutable shared files like JSONL conflict; settled by D3). Format: **JSON in a canonical encoding** — sorted keys, fixed number/string forms, UTF-8, no insignificant whitespace. Canonical bytes are what make the reserved `sig` field possible later and make identical events produce identical bytes everywhere. Filename = event ID: `events/2026/07/29/<ULID>.json`.

**Envelope** (every event, regardless of type):

```json
{
  "id": "01J9GQ3V7R8Z4M2K...",        // ULID — sole ordering key
  "type": "task.created",             // noun.verb, namespaced
  "v": 1,                             // schema version OF THIS TYPE
  "actor": "brandon/impl-2",          // D7 principal
  "machine": "m-3f9a",                // stable per-machine id
  "task": "t-01J9GQ...",              // subject task, if any
  "sig": null,                        // reserved (D7)
  "data": { ... }                     // type-specific payload
}
```

**Ordering.** Replay order is ULID lexical order — a deterministic total order whether or not wall clocks are truthful. A skewed clock can make a claim unfairly win "earliest"; it can never make two machines disagree about who won. Fairness-under-skew is traded for mandatory determinism. No Lamport machinery in v1.

**The three version contracts** (distinct, independently moving — conflating them is how tools nag users into constant upgrades):

| Contract | Bumped when | Consequence of mismatch |
|---|---|---|
| **Event schema** (per-type `v`) | Breaking payload change only — additive fields never bump it | **Fail-safe mode** (below). Rare by design; ideally major versions only. |
| **View format** (single integer) | Rendered markdown output changes | Cosmetic: older machines see slightly stale markdown until upgrade or peer sync (T6). No lockout, no nag. |
| **Daemon binary** | Every release | None. Peers at different binary versions collaborate in total peace if the above two match. |

**Versioning rules:** additive-first (readers ignore unknown fields; rewriters preserve them); breaking changes bump per-type `v` and the daemon carries **upcasters** — pure functions lifting old shapes to current at replay time, in memory only; stored bytes are never rewritten.

**Fail-safe, not best-effort.** When a daemon meets events it cannot honestly replay (unknown type from a newer peer, `v` above its comprehension), it enters **read-only degraded mode**: everything visible as of the last comprehensible state, writes and view regeneration paused, clear "upgrade tuhdoo" message. Best-effort skipping is rejected because it lets two teammates compute different truths from the same log and then write events based on divergent views — the "implicitly brittle" failure mode readmitted.

## T4. Daemon topology and transport

- **One daemon per repo** (not per machine). Repo-local state, config, locks, and sync loop; crash blast radius of one project; per-repo version pinning. A future per-machine supervisor / cross-project dashboard can *manage* per-repo daemons without reversing this. 
- **Endpoint:** Unix domain socket at `.git/tuhdoo/daemon.sock`, with `.git/tuhdoo/daemon.json` (pid, endpoint) for discovery — inside `.git`, never committed.
- **Protocols:** (1) **MCP via streamable HTTP** for harnesses; (2) a **plain JSON HTTP API** — same verbs, boring REST — for the CLI, the TUI, future UIs, and curl. All surfaces are projections of the same daemon state and cannot disagree.
- **stdio shim:** `tuhdoo mcp` proxies stdio ↔ daemon socket, so any harness config is just `command: tuhdoo, args: [mcp]`. The D7 principal is auto-derived at session bind *(revised 2026-07-30; originally `--as` was required)*: the shim takes the human half from git identity (the local part of `user.email`) and the daemon mints the agent half from the harness's MCP initialize `clientInfo.name` plus a per-daemon counter (`brandonbews/claude-code-3`), unique among the daemon's live sessions. `--as <principal>` remains as a full override for shared machines and scripted actors.
- **Principal override** *(added 2026-07-30)*: `git config tuhdoo.principal <name>` sets the preferred human principal once, replacing the `user.email` local-part rule everywhere auto-derivation applies — the mcp shim and the TUI's steer mode (motivation: GitHub noreply addresses derive to `4099114+brandonbews`). `--as` still beats it; the derived result passes the same validation (root principal only — the agent half stays daemon-minted), and a set-but-invalid value fails derivation loudly rather than silently falling back to email. **Storage is git config, not a file under `.git/tuhdoo/` and not the data branch**: a preferred name is per-human, per-clone state, so it does not belong in shared fleet state; git config gives one familiar command, per-machine reach via `--global` with ordinary precedence (local beats global), survival across `.git/tuhdoo/` wipes, and zero new file formats. Old events keep their recorded actors — identity derivation changes forward behavior only (T3).
- **Lifecycle:** lazy — first CLI/MCP invocation auto-spawns the daemon if absent (single instance via lockfile). No launchd/systemd ceremony. The daemon does not self-shutdown when idle (it is a few MB; cold starts cost more than it saves) and never exits silently — reason always logged.

## T5. MCP verb surface (v0) *(revised 2026-07-30 — out-of-band answers: `relay_answer` is the eleventh verb)*

Eleven tools *(originally ten; `relay_answer` added 2026-07-30 — rationale below)*. Agents perform worse as tool count grows; every verb earns its slot. The D1 loop — claim → work → record → escalate — in minimum calls.

| Group | Verb | Notes |
|---|---|---|
| Orient | `get_backlog` | Ready-filtered, priority-ordered, dependency-aware — not a raw dump. |
| Orient | `get_task` | One task fully hydrated: description, edges, notes, runs, escalations — start work in one call. |
| Loop | `claim_next` | **Atomic query-and-claim**: best ready task, claimed in one motion, optional capability/label filter. Kills the local browse-then-claim race; fewer tokens. |
| Loop | `claim_task` | Claim a specific task (the human-directed path). |
| Loop | `release_claim` | Voluntary stand-down, reason recorded. |
| Loop | `finish_run` | Outcome + links (branch/PR/commits) + summary. Outcomes: `done` / `failed` / `abandoned` / **`blocked`** (waiting on an escalation answer — distinct from gave-up). |
| Communicate | `escalate` | Question + context + blocking-or-not. |
| Communicate | `add_note` | Checkpoint observations (see agent protocol below). |
| Communicate | `relay_answer` | *(added 2026-07-30)* Record an out-of-band answer to an escalation — one given in the asking agent's live session rather than on a steering surface. Emits the same `escalation.answered` event as a TUI answer, so readiness unblocks identically. Semantics below. |
| Decompose | `create_task` | **Batch with intra-batch temp refs**: submit a whole DAG (epic + children + dependency edges) in one call using placeholder refs (`"tmp:setup-daemon"`); daemon assigns real IDs, resolves refs, commits atomically — whole plan lands or nothing does. Single task = batch of one. |
| Decompose | `update_task` | Status, priority, edges, labels. |

**Session-bound leases — no heartbeat verb.** While the MCP session holding a claim stays connected, the daemon auto-renews its leases. Session dies → renewals stop → lease expires → task returns to pool. The least reliable party (the agent) is never responsible for the most safety-critical bookkeeping. The daemon **auto-closes runs orphaned by lease expiry** as `outcome: interrupted` — never assume an agent's last act was tidy.

**Out-of-band answers: the relaying agent is the scribe, never the decider** *(2026-07-30)*. Escalations are often answered in the asking agent's own live session rather than on a steering surface; without a relay path the settled question lingers in the inbox, polluting exactly the signal escalations exist to provide. `relay_answer` records such an answer with honest attribution: the event's envelope actor is the relaying session's principal (the agent wrote the event), and an additive `answered_by` payload field on `escalation.answered` names the principal the answer is attributed to. The daemon derives `answered_by` as the session principal's root (D7) — it is never agent-supplied, so a session can only ever attribute an answer to its own root. Replay credits the answer to `answered_by` and, where that differs from the envelope actor, marks the answer as *relayed by* the actor; pre-existing events carry no `answered_by` and replay falls back to the envelope actor, so stored bytes stand untouched (T3, additive-first). Guard rails: the agent surface answers **open escalations only** — amending a settled answer stays human work on the steering surfaces, where last-answer-wins applies; **no claim on the task is required** — the blocking protocol has the asker release before any answer can land; **any session may relay** — restricting callers would be theater, since the trust boundary is repo write access (D4) and the attribution derivation is the real guard. The "human" in this section is today's answerer, not a type constraint: attribution binds to the session's root principal, whatever kind of actor that turns out to be.

**No delete, no admin verbs.** Curation (cancel, reprioritize, archive) is human work via CLI/TUI on the HTTP API. The agent surface is a worker's toolbox, not a janitor's keyring.

**The agent protocol doc is a first-class deliverable.** Findings from role-playing the primary user (a frontier-model agent):

- **The ledger is agent memory before it is human audit trail.** Sessions end and contexts compact; a fresh agent claiming a previously-attempted task resumes from prior runs and notes instead of re-deriving hours of investigation. *(Revised 2026-07-30, steering: continuity is carried by the typed transition events — claim, finish_run, release_claim, escalate, the synthesized interrupted run — whose payloads are already mandatory; `add_note` is optional mid-flight garnish, not the primary mechanism. The original "checkpoint noting … letters to the next incarnation" framing overstated it; the agent protocol doc carries the current doctrine.)*
- **Escalations are answered after the asking agent is gone — design for succession, not conversation.** The protocol: escalate → note exactly where work stopped → release the claim → `finish_run(blocked)`. The answer lands on the task; the next claimant picks up question-and-answer in hydration.
- **Task descriptions are prompts.** Output quality off `claim_next` is bounded by what's in the task: acceptance criteria, constraints, file pointers. In product (b), writing a good task *is* programming the fleet — a documentation/culture convention with more leverage than any schema feature.

## T6. Views: four, code-generated, version-stamped

Rendered to the data branch on every write/merge; audience is the **remote/browsing surface** (host web UI, teammates, posterity) — local humans use the CLI (T7), agents use MCP.

1. **`README.md`** (branch root) — orientation + live counts; the host renders it when anyone browses the branch. The "clone it and you can read it" soul made visible.
2. **`backlog.md`** — ready (priority order), claimed (by whom), blocked (on what); parent structure as nesting.
3. **`escalations.md`** — the steering inbox: open questions, who, which task, waiting how long.
4. **`tasks/<id>.md`** — per-task biography: description, edges, status history, notes, runs with links. `git log` on the file is the task's history for free.

Nothing else in v0. **No template engine** — plain Go functions building markdown, embedded in the binary: nobody edits templates in v0, determinism is the prime directive, and string-building Go is the most testable and author-reviewable form (byte-exact table-driven tests). User-customizable templates are a parked future feature.

**View-churn defense:** views carry a **view-format version** stamp (`views/.meta`); the rule is **highest version wins** — a daemon never overwrites views stamped by a newer generator; it updates the event log only and lets the newer peer regenerate. Prevents mixed-version regeneration ping-pong wars. Staleness consequence is cosmetic only (T3 table); replayed state (CLI/MCP/TUI) is always current.

## T7. Human surfaces: CLI portal in v0, TUI in v1, browser demoted *(revised 2026-07-30 by Cycle 4 — one verb-less TUI)*

The v0 CLI is not a thin setup veneer — it is the **local human's primary portal** (the author runs terminal-over-SSH; checking out a branch to read markdown is absurd, and a browser tab is friction where a tmux pane is native):

- `tuhdoo status` — one-screen fleet overview: active claims, open escalations, queue depth, sync state (`local-only` is a normal state).
- `tuhdoo backlog` / `tuhdoo task <id>` / `tuhdoo escalations` — terminal renderings served from the daemon's replayed state; always current. Scriptable, pipeable plumbing — deliberately untouched by the Cycle 4 revision.
- `tuhdoo create <title>` / `tuhdoo update <id>` / `tuhdoo answer <id> <text>` *(added 2026-07-31)* — the write path when no MCP session exists: shell humans who prefer commands to the TUI, scripts, cron jobs, and agents wrapping up after a deploy restart killed their session. They speak the same daemon HTTP API as TUI steer mode with the same actor rules (a root human principal: `--as` wins, else the `tuhdoo.principal` git config, else the `user.email` local part). Descriptions read from stdin via `--desc -`; list flags are full replacements; `answer` addresses an open escalation by its own ID or its task's ID, with the short-ID input contract throughout. **The claim lifecycle (claim / finish_run / release) is deliberately excluded**: leases are session-bound (T8) — a claim taken by a one-shot command would have nothing renewing it and would lapse interrupted. The MCP shim stays the only work-loop surface; the CLI help text owns this exclusion explicitly.

**The interactive surface is a single verb-less TUI** *(Cycle 4, 2026-07-30 — supersedes the v0 `watch` command and the v1 `tuhdoo top` name; both shipped, briefly coexisted as duplicate Bubble Tea models, and are now tombstoned)*. **Bare `tuhdoo`** launches the steering TUI (Bubble Tea, same binary), guarded: it launches only when stdout is a TTY — otherwise usage prints, exactly as bare invocation always behaved, so pipes/scripts/CI never meet a TUI — and an uninitialized repo gets a `run tuhdoo init` hint. `-w`/`--watch` launches the same screen **disarmed**: steering keys dead, navigation and the task detail screen alive, a visible `watch mode` badge in the header. The mode is fixed at launch — no keypress can re-arm a disarmed pane. Watch mode needs no principal; steer mode derives one from git identity as before (`--as` overrides; combining `--as` with `--watch` is rejected).

One screen carries the whole picture: a header (mode badge, sync line, counts), four status sections (open escalations, ready, in progress, blocked — with `waiting:` reasons) *(2026-07-30: the escalations section renders under the title "Needs Input" — the entity keeps its name everywhere; the header alone softens the severity the word overstates, and deliberately avoids naming who answers, since a future answerer may not be a human)*, flat rows with edge-marker suffixes (`· N deps`, `· in <parent>` — tree rendering is gated on the edge-semantics open question), and an in-place task detail screen (enter: description plus full ULID-merged history, live off the polled snapshot) — read-only this round; arming it is a filed follow-up. Key semantics are absolute: `esc` steps out, `q` quits, `ctrl+c` quits — one meaning per key, on every screen.

**Short IDs are the human contract** *(2026-07-30)*: the git model, applied to task IDs — the full ULID is plumbing, the short form (`t-` + lowercase last four of the ULID tail, e.g. `t-d83w`) is what humans see and type. Canonical IDs remain ULIDs (sequential IDs would need the central counter the no-server design forbids, and ULID sort order is load-bearing); this is display and input sugar only — stored bytes, events, the HTTP API, and the MCP verbs are untouched (agents pass full IDs from hydration JSON and never retype them). Display: every task reference in the TUI uses the short form; the detail screen keeps the full ULID exactly once, dimmed on its `id` line, as the copyable canonical form. Dep/parent references that resolve in the snapshot are annotated with status and title (`depends on t-d83w (done — old chore)`) — done/cancelled tasks render no rows, so the annotation is what proves such an edge isn't dangling; unresolvable IDs render bare, never an invented status. Input: `tuhdoo task <id>` accepts a full ID, the short form, or any unambiguous ID fragment; ambiguity is an error listing the candidates. One-shot command output stays byte-identical — scriptable plumbing keeps full IDs.

Accepted consequences (Cycle 4): the always-open pane is armed unless launched with `--watch` (mitigations: the flag, and cancel stays confirm-gated); no runtime arm/disarm toggle — fluidity traded for an ironclad read-only guarantee; `watch` and `top` die as verbs with tombstone errors, not aliases — vestigial names outlive their excuse otherwise.

Rationale unchanged from v1's original TUI bet: works over SSH and in tmux where the author actually lives; Go's TUI ecosystem is best-in-class (retroactively reinforcing T1); and the steering scope (inbox, ledger, queue) is naturally a dense terminal dashboard. **Browser UI is demoted to v2+** — it's the kanban board that wants a browser, and the board stays banned until the steering loop is proven (D8). CLI, TUI, and any future web UI all speak the same daemon HTTP API; nothing is throwaway.

## T8. Cadence defaults

Config-file knobs; these are starting values, and the daemon logs collision counts and sync latencies so tuning is evidence-based.

| Knob | Default | Rationale |
|---|---|---|
| Commit debounce | 2s | Batch a fleet's burst into one commit; the TUI still feels live. |
| Push | Immediate for commits containing claims or escalations; else piggyback ≤ 30s | Claims race (D6); escalations have a human waiting. Notes can amble. |
| Fetch interval | 60s | With eager claim-push, worst-case collision window ≈ 1 min; D6 accepts rare duplicates. Host webhooks could make this push-driven later (optional add-on per T2). |
| Lease TTL / renewal | 15 min / every 5 min | Renewal is the daemon's job (session-bound), so short TTLs cost nothing; crashed fleets return tasks in ≤ 15 min. Three missed renewals = expiry. *(Supersedes the ~30 min sketch in D6.)* |
| MCP session keepalive | Ping every 30s; 3 failures tolerated | Session liveness is what lease renewal hangs off (T5), so the daemon probes it directly; three tolerated failures mirrors the lease posture above — a transiently slow client is not a vanished one. *(Added with B9; pings ride streamable HTTP's standalone GET stream, which is why agent access is shim-only — see `docs/agent-protocol.md`.)* |
| Idle shutdown | None | Cold-start latency isn't worth a few MB. Exits are always logged with reason. |
