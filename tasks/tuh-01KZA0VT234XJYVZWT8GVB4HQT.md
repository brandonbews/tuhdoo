# Agent protocol: document how a successor finds a predecessor's branch (salvage breadcrumbs)

`tuh-01KZA0VT234XJYVZWT8GVB4HQT`

- **Status:** done
- **Priority:** 0
- **Labels:** `docs` `protocol`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Context: shrunk from the migrated open-question "salvage flow for superseded and interrupted runs" at the 2026-08-06 grill (Brandon). The larger question dissolved: the salvage *record* was settled by the 2026-08-04 loser-handling rewrite in 001 (a reporting loser's finish_run is coerced to superseded keeping branch/PR/commits/summary; silent losers get synthesized branch-less runs; late losers are pointed at add_note); reviewing superseded branches is anti-valuable by D6's accepted residual-waste consequence (the winner already did the work — the run is disclosure, not a review queue), with aggregate waste surfaced via Status.Collisions; and GC of stray branches is host-repo hygiene outside tuhdoo's jurisdiction (T2 — tuhdoo never manages host branches; D9 compacts ledger events, not code branches). What survives is one gap: a synthesized interrupted run is branch-less, so the ledger alone cannot point a successor at the abandoned branch.

The ask: a small addition to docs/agent-protocol.md telling (a) successors where to look for a predecessor's work when the run record has no branch — the predecessor's notes first, then host-repo branches carrying the task's short id; and (b) working agents what breadcrumbs to leave so that search works — put the task id in your branch name, and add_note the branch once it exists if there's any chance of interruption. A sentence on why interrupted runs can be branch-less (the daemon synthesizes them; it cannot know a name only the agent knew) keeps the doc honest.

Acceptance: docs/agent-protocol.md contains the successor-search guidance and the breadcrumb guidance, phrased as practice/suggestion, not protocol law; the doc's revision header notes the change and this grill. No code changes; make test lint stays green (docs-only).

Pointers: docs/agent-protocol.md (the notes/step-4 section and the escalation section are the neighboring prose); 001-core-design.md loser-handling clause 3 (2026-08-04) for the settled record semantics.

Constraints: stay workflow-agnostic (agent-protocol step 3: ordinary git on ordinary branches) — the tuh-<short-id>/<slug> convention is this repo's shape, so frame id-in-branch-name as a recommended breadcrumb, never a requirement; the workflow-recipes held task (tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ) is where the full convention may eventually ship.

## History

### 2026-08-06 23:18 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-4hqt/salvage-breadcrumbs`
- PR: <https://github.com/brandonbews/tuhdoo/pull/45>
- Commits: `ed51b48`

Added the salvage-breadcrumbs paragraph to docs/agent-protocol.md, placed after the loop (right below the "never end holding a claim silently" rule): why daemon-synthesized interrupted runs are branch-less, the successor search order (predecessor's notes first, then host-repo branches carrying the task's short id), and the breadcrumbs to leave (task id in branch name, add_note the branch once it exists) — framed as recommended practice, not protocol law, keeping step 3's workflow-agnosticism. Revision header updated citing the 2026-08-06 salvage grill. Docs-only; make test lint green; squash-merged to main via PR #45.
