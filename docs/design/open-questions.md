# Open questions

Unresolved threads, grouped into candidate grilling cycles. Move answers into the numbered design docs as they get settled.

*Cycle 2 (technology choices) was resolved 2026-07-29 → `002-technology.md`.*

## Cycle 3 — The agent loop in practice (next up)

- **Write the agent protocol doc** (promoted to first-class deliverable in `002` T5): the actual instruction text a harness loads — claim before working, checkpoint notes as letters to the next incarnation, escalate → note → release → `finish_run(blocked)` for blocking questions, always finish or release. Then test it against a real harness and see where agents actually deviate.
- **Task-descriptions-are-prompts conventions:** what does a well-formed task look like (acceptance criteria, constraints, file pointers)? Template or convention? This is "programming the fleet" and deserves its own doc.
- How does an agent discover "what should I work on next" beyond `claim_next` defaults — capability/label filters, affinity hints, priority semantics?
- Salvage flow for `superseded` and `interrupted` runs: who reviews them, how surfaced (CLI? escalation?), when garbage-collected?
- How do escalations reach a human *promptly* when the TUI is not open — OS notification from the daemon? Nothing in v0? (Must not violate host-agnosticism or no-server.)
- Run ↔ code linkage robustness: branches get deleted, PRs live host-side; what exactly does a Run store so links stay meaningful years later?
- The plan-materialization flow end-to-end: a planning conversation ends with "turn this into tuhdoo tasks" — batch `create_task` exists (`002` T5), but what makes the output *good*? Prompting conventions for decomposition quality.
- **Edge semantics, cycle enforcement, and what a milestone is** *(hit during the B12 cutover, 2026-07-30)*. D5 says "parent edges + dependency edges (a DAG)" without saying whether each relation is a DAG or their union is one; the code chose the union (`opCreateTasks` cycle check), which makes the natural milestone shape — children carry parent edges into it, it depends_on the children — unrepresentable. Three sub-questions: (1) *Semantics:* parents are containment, depends_on is scheduling; should cross-relation "cycles" be legal, with each relation acyclic on its own? (2) *Enforcement honesty:* the check guards only one door — `update_task` does no cycle check, intra-batch edges to pre-existing tasks aren't tracked, and set-union merge (D3) means two acyclic writes can union into a cycle no daemon ever saw. Cycle-freedom structurally cannot be a global invariant here; the core already tolerates dep-cycles by convergent starvation (`core.Ready` returns false forever, identically on every machine). Should write-time checks be per-relation + added to `update_task`, with dep-cycles *surfaced at read time* (status/views warning) instead of pretended away? (3) *Milestones:* D5's "just tasks other tasks point into" is underspecified for readiness — a milestone whose deps complete becomes *claimable*, which is meaningless for human-verification work (B12 fenced the v0 milestone with a blocking escalation as a workaround). Do milestones want computed done-ness rather than claimability? The edge-check question is downstream of this one.

## Cycle 4 — Team onboarding & operations

- `tuhdoo init` UX: creating the orphan branch, CI path-filter guidance, branch-protection guidance, teammate joining flow, remoteless-start flow (`002` T2 requires it be a normal state).
- Repo-hosting edge cases: shallow clones, `--single-branch` clones, forks, mirrors.
- Monorepo story: one tuhdoo branch per repo — is that the right grain when a repo hosts many projects?
- Multi-repo story: does a plan ever span repos, or is that explicitly out of scope?
- What does "uninstall" look like — how cleanly can a team walk away (delete branch, done)?
- Epoch compaction triggers and mechanics in practice (D9): when exactly, who initiates, how is the snapshot verified?
- **Working-set retirement** *(grill session 2026-08-02 — distinct from D9 file compaction, and NOT a revival of the status word "archived" retired 2026-08-01)*: a way for old terminal tasks to leave the live working set — an ordinary appended event, no file moves, no deleted history, consciously recoverable (a flag on read surfaces, or an un-retire event). Two motivations: the TUI/views/resolver stop growing without bound, and the short-ID fragment-resolution pool stays bounded at working-set size, which keeps 4-char tails effectively unique at any scale (the T7 revision of 2026-08-02 accepted that ambiguity otherwise grows with total task count — ~38% chance some pair shares a tail by ~1,000 tasks). Design constraint discovered in the grill: resolution must stay loud — a fragment matching one live task *and* retired ones should resolve to the live task but say so, because silently resolving an old short-ID reference (in a PR body or branch name) to a newer tail-colliding task is the worst failure mode. Naming needs care per the vocabulary collision above; the mechanism also wants to compose with D9 compaction (retired tasks are natural snapshot-elision candidates) without being coupled to it.

## Cycle 5 — Later / parked

- Public intake bridge (GitHub Issues → events) for OSS projects — parked per D4; must remain an optional add-on per `002` T2 host-agnosticism.
- Event signing (`sig` field) — parked per D7.
- Kanban/board browser UI — explicitly gated on the steering loop being proven (D8); demoted to v2+ (`002` T7).
- User-customizable view templates — parked (`002` T6); would need view-format-version treatment.
- Webhook-driven fetch (replacing polling) — optional host add-on, parked (`002` T8).
- Per-machine supervisor / cross-project dashboard over per-repo daemons — parked (`002` T4).
- Read-only sharing with non-committers (stakeholder view) — conflicts with D4's trust boundary; revisit only with real demand.
- Name check: is "tuhdoo" the shipping name?
