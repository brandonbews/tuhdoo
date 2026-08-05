# Dependency loops and cancelled deps: reject at edit, mark loudly at replay (edge grill 2026-08-05)

`tuh-01KZ9Y3THHH5B8GT22T1A1WPYP`

- **Status:** open — blocked on dependencies
- **Priority:** 2
- **Labels:** `go` `edges` `tui`
- **Depends on:** [`tuh-wzrg`](tuh-01KZ9Y3THHH5B8GT22SY3FWZRG.md) (open)
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: the 2026-08-05 edge grill resolved open question 3.8(2). Cycle-freedom structurally cannot be a global invariant — set-union merge (D3) can union two individually-acyclic writes into a dependency loop no daemon ever saw — so the decided posture is honest both ways: refuse to knowingly write a loop, and loudly surface any loop that exists. Also decided: a dependent of a *cancelled* dependency stays blocked (cancelled never counts as done) but must be visibly marked — re-pointing the edge is a human decision. A dangling dep ID counting as met stays as-is; document it as defensive posture in a code comment. Depends on the parents-removal task: with one edge type the graph logic is simpler and ops.go won't conflict.

The ask: (1) update_task rejects a depends_on replacement that would create a loop visible in current local state, with a clear error naming the tasks in the loop. (2) Core detection as pure functions: for blocked tasks, detect membership in a depends_on loop (loops among not-done tasks) and detect waiting-on-cancelled deps; expose alongside ClaimBlockers/Situation so every surface reads one answer. (3) Surfacing: backlog view, TUI, one-shot `backlog`/`task` output, and get_backlog blocked rows mark loop members ("cyclic — a human must cut an edge") and cancelled-dep waiters ("waiting on cancelled <id>") distinctly from ordinary waiting. Additive output fields only — verb count and status vocabulary unchanged (blocked stays the status; these are annotations, not new statuses).

Acceptance: table-driven core tests for a 2-loop, a 3-loop, a loop with a non-loop tail, and cancelled-dep marking; a daemon test proving update_task refuses a loop-closing edit with the named-tasks error; a test proving the markers appear in backlog output; `make test lint` green.

Pointers: internal/core/state.go:164-199 (Ready/ClaimBlockers) and :201-219 (Situation), internal/daemon/ops.go:215 (opUpdateTask) and :1303 (hasCycle), internal/views/views.go, cmd/tuhdoo/top.go.

Constraints: deterministic core stays pure (data in, data out); never claim prevention in any message or doc — detection only for the merge case; no new status words (2026-08-01 status grill); boring Go; one PR.

## History

_No activity yet._
