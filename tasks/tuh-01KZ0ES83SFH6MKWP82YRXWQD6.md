# One classifier: the daemon serves the derived situation

`tuh-01KZ0ES83SFH6MKWP82YRXWQD6`

- **Status:** done
- **Priority:** none
- **Labels:** `cleanup` `datashape`
- **Created:** 2026-08-02 05:22 UTC by `brandon/claude-code-1`

## Description

## Context

Grilled 2026-08-03 (Brandon + Claude); this description supersedes the 2026-08-02 capture and records the settled design. The anticipated blessed-state write-up never landed in docs — this description is the source of truth.

The derived-situation ("weather") rules are implemented three times:

1. `core.State.Ready` / `ClaimBlockers` (internal/core/state.go) — authoritative, table-driven-tested. `Ready` consumes `ClaimBlockers`, so the escalation clause is inside `Ready`.
2. A hand-mirror in cmd/tuhdoo/snapshot.go (`claimable`, `hasUnmetDeps`, `blockingEscalation`, plus `statusOf`'s `!= "done"` comparisons) — exists only because `/v0/state` carries no verdict. Consumed by the CLI `classify`, `waitingOn` (serialized backlog blocked rows, commands.go), `blockedReasonTUI` (top.go ~1466), and the no-BLOCKED-row rule (top.go ~169).
3. internal/views/views.go: a **dead** re-filter in `classify` (the ready-loop re-check ~109 and the `|| blockingEscalation(...)` disjunct ~128 — the comment predates `Ready` gaining the escalation clause and is now false) **plus two live duplicates the original capture missed**: the local `blockingEscalation` helper (~136, used ~285/~407 for "open question" markers) and the dep-status loop inside views' `waitingOn` (~281).

Blessed rule (Brandon 2026-08-01, confirmed at grill): exactly one implementation of the derived-situation logic, in core.

## Settled design (grill 2026-08-03)

- **Core owns the whole classifier, not just the predicates.** Core gains one exported function — `Situation(taskID)` returning one word per task: `ready` | `in_progress` | `blocked` for open tasks, the status word itself for inbox/held/done/cancelled — table-driven-tested next to `Ready`. Bucket membership and `Ready`'s meaning are unchanged everywhere; the existing switches move into core, they don't change.
- **Wire shape**: `stateTask` in `/v0/state` gains
  - `situation` string — always present, never empty (repeats the status word for non-open tasks), so every consumer does one switch on one field;
  - `unmet_deps` []string — task IDs, omitempty, served for every task regardless of status (matches `ClaimBlockers`' documented contract);
  - `blocking_escalations` []string — escalation IDs, likewise.
  - `situation` joins the one-status-vocabulary doctrine: stored words are the served words (`in_progress` with underscore as a wire field); the TUI keeps owning any display mapping.
- **Nothing is stored.** `situation` is computed at serve time from event-derived state, like `Ready` today. Event log, task shape, and every agent verb untouched — zero agent overhead; it can't go stale because there is no second copy.
- **`get_task`'s hydrated response is untouched** — classification is a listing concern; the task view renders edges from hydration and doesn't classify.
- **internal/views folds in**: `views.classify` becomes a grouping loop over `core.Situation` (dead filter and stale comment deleted); views' `waitingOn` and the escalation markers consume `core.ClaimBlockers`; the local `blockingEscalation` helper dies.
- **CLI**: `classify` switches on served `situation`; `waitingOn`/`blockedReasonTUI`/no-BLOCKED-row consume the served lists (`len(t.UnmetDeps)` etc.); mirror predicates `claimable`, `hasUnmetDeps`, `blockingEscalation`, `statusOf` are deleted.
- **N+1 hydration stays as-is** (detail view, escalations shelf, edge rendering still consume it; cheap over a local socket at v0 volumes). Delete only what becomes dead. Scope is one-classifier, not performance.

## Acceptance

- One implementation of the situation/bucket rules, in internal/core; grep proves no dep-status comparisons or open-escalation loops in cmd/ **or internal/views/** outside calls into core.
- TUI, CLI, and committed markdown output byte-unchanged (the vocabulary task #14 landed; current output is the baseline) — including the settled rule that escalation-only-blocked tasks render no BLOCKED row in the TUI.
- Core's table-driven tests cover `Situation` alongside `Ready`; daemon tests cover the new `stateTask` fields.
- `make test lint` green from repo root.

## Constraints

- Do not change what `Ready` means or bucket membership on any surface — consolidation, not redesign.
- Stored event bytes untouched; `situation` is never persisted (T3 additive rule doesn't even engage — no schema change).

## History

### 2026-08-03 19:40 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→open

### 2026-08-03 19:51 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-wqd6/one-classifier`
- PR: <https://github.com/brandonbews/tuhdoo/pull/23>

Grilled with Brandon 2026-08-03 (design settled in the task description), then built and merged as PR #23. Core gained Situation(taskID) — the one classifier: ready/in_progress/blocked for open tasks, the status word otherwise. /v0/state's stateTask now serves situation + unmet_deps + blocking_escalations (ClaimBlockers' ID lists, all tasks regardless of status). views.classify groups over core.Situation (dead escalation re-filter deleted); views' waitingOn/statusLine consume ClaimBlockers; the CLI mirror (claimable, hasUnmetDeps, blockingEscalation, statusOf) is deleted — the CLI renders served verdicts only. Output byte-unchanged on all surfaces; golden expectations untouched. One deliberate edge change: the serialized backlog's WAITING cell lists every open blocking escalation, not just the earliest. make test lint green; CI green; squash-merged.
