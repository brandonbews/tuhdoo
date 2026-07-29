# Open questions

Unresolved threads, grouped into candidate grilling cycles. Move answers into `001-core-design.md` (or a new numbered doc) as they get settled.

## Cycle 2 — Technology choices (next up)

Deliberately deferred during the design grilling so the stack serves the design, not vice versa.

- **Daemon runtime/language.** Constraints from the design: long-lived localhost process, embeds an MCP server, shells out to (or embeds) git, serves a local UI later, easy single-binary install for `tuhdoo init`. Candidates worth grilling: Go, Rust, Bun/Node, and why.
- **Git plumbing approach.** Subprocess `git` vs. an embedded library (libgit2 bindings, gitoxide, go-git). Key workloads: committing to a branch that is never checked out (plumbing: `hash-object` / `mktree` / `commit-tree` / `update-ref` — no worktree needed), merges of the data branch, and fetch/push loops. Reliability vs. speed vs. dependency weight.
- **Event format details.** JSON vs. JSONL vs. something else; envelope schema (id, actor, sig-reserved, schema version); event schema versioning and migration strategy for an append-only log.
- **MCP server surface.** Which verbs, exactly, for v0 (claim, release, record-run, escalate, query-backlog, …); how a harness session binds to an agent principal.
- **View generation.** Templating approach; what views v0 actually needs; whether views live on the same branch or a derived one.
- **Local API transport.** MCP-only vs. MCP + plain HTTP/JSON for the CLI and future UI; daemon discovery (port file? socket?); one daemon per repo vs. per machine.
- **Sync cadence numbers.** Fetch interval, debounce window for commit batching, lease heartbeat interval and expiry — pick starting values and how they get measured/tuned.

## Cycle 3 — The agent loop in practice

- What does the claim → work → record → escalate loop look like from inside a real harness (Claude Code, others)? Write the actual system-prompt/skill text an agent would use.
- How does an agent discover "what should I work on next" — priority order, dependency-readiness, affinity hints?
- Salvage flow for `superseded` runs: who looks at them, how are they surfaced, when are they garbage-collected?
- How do escalations reach a human *promptly* when no UI is open — notification channel (OS notification from daemon? nothing in v0?) without violating no-server.
- Run ↔ code linkage details: how runs reference branches/commits/PRs robustly (branches get deleted, PRs live host-side).

## Cycle 4 — Team onboarding & operations

- `tuhdoo init` UX: creating the orphan branch, CI path-filter guidance, branch-protection guidance, teammate joining flow.
- Repo-hosting edge cases: shallow clones, `--single-branch` clones, forks, mirrors.
- Monorepo story: one tuhdoo branch per repo — is that the right grain when a repo hosts many projects?
- Multi-repo story: does a plan ever span repos, or is that explicitly out of scope?
- What does "uninstall" look like — how cleanly can a team walk away (delete branch, done)?

## Cycle 5 — Later / parked

- Public intake bridge (GitHub Issues → events) for OSS projects — parked per D4.
- Event signing (`sig` field) — parked per D7.
- Kanban/board UI — explicitly gated on the steering loop being proven (D8).
- Read-only sharing with non-committers (stakeholder view) — conflicts with D4's trust boundary; revisit only with real demand.
- Name check: is "tuhdoo" the shipping name?
