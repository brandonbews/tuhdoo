# Open questions

Unresolved threads, grouped into candidate grilling cycles. Move answers into the numbered design docs as they get settled.

*Cycle 2 (technology choices) was resolved 2026-07-29 → `002-technology.md`.*

## Cycle 3 — The agent loop in practice (next up)

- **Write the agent protocol doc** (promoted to first-class deliverable in `002` T5): the actual instruction text a harness loads — claim before working, checkpoint notes as letters to the next incarnation, escalate → note → release → `finish_run(blocked)` for blocking questions, always finish or release. Then test it against a real harness and see where agents actually deviate.
- **Task-descriptions-are-prompts conventions:** what does a well-formed task look like (acceptance criteria, constraints, file pointers)? Template or convention? This is "programming the fleet" and deserves its own doc.
- How does an agent discover "what should I work on next" beyond `claim_next` defaults — capability/label filters, affinity hints, priority semantics?
- Salvage flow for `superseded` and `interrupted` runs: who reviews them, how surfaced (CLI? escalation?), when garbage-collected?
- How do escalations reach a human *promptly* when neither `watch` nor the TUI is open — OS notification from the daemon? Nothing in v0? (Must not violate host-agnosticism or no-server.)
- Run ↔ code linkage robustness: branches get deleted, PRs live host-side; what exactly does a Run store so links stay meaningful years later?
- The plan-materialization flow end-to-end: a planning conversation ends with "turn this into tuhdoo tasks" — batch `create_task` exists (`002` T5), but what makes the output *good*? Prompting conventions for decomposition quality.

## Cycle 4 — Team onboarding & operations

- `tuhdoo init` UX: creating the orphan branch, CI path-filter guidance, branch-protection guidance, teammate joining flow, remoteless-start flow (`002` T2 requires it be a normal state).
- Repo-hosting edge cases: shallow clones, `--single-branch` clones, forks, mirrors.
- Monorepo story: one tuhdoo branch per repo — is that the right grain when a repo hosts many projects?
- Multi-repo story: does a plan ever span repos, or is that explicitly out of scope?
- What does "uninstall" look like — how cleanly can a team walk away (delete branch, done)?
- Epoch compaction triggers and mechanics in practice (D9): when exactly, who initiates, how is the snapshot verified?

## Cycle 5 — Later / parked

- Public intake bridge (GitHub Issues → events) for OSS projects — parked per D4; must remain an optional add-on per `002` T2 host-agnosticism.
- Event signing (`sig` field) — parked per D7.
- Kanban/board browser UI — explicitly gated on the steering loop being proven (D8); demoted to v2+ (`002` T7).
- User-customizable view templates — parked (`002` T6); would need view-format-version treatment.
- Webhook-driven fetch (replacing polling) — optional host add-on, parked (`002` T8).
- Per-machine supervisor / cross-project dashboard over per-repo daemons — parked (`002` T4).
- Read-only sharing with non-committers (stakeholder view) — conflicts with D4's trust boundary; revisit only with real demand.
- Name check: is "tuhdoo" the shipping name?
