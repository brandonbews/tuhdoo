# Remove the parents edge: epics are depends_on containers (edge grill 2026-08-05)

`tuh-01KZ9Y3THHH5B8GT22SY3FWZRG`

- **Status:** done
- **Priority:** 3
- **Labels:** `go` `edges`
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: the 2026-08-05 edge-semantics grill (release grill for v0.2.0) resolved open question 3.8(1). The `parents` field never had mechanics — its only consumer is a display line in the task view — and Brandon decided it goes away entirely. The epic pattern inverts: a container is an ordinary task that `depends_on` its children (blocked while they're open, ready when they finish, done when a human declares it). A separate inbox task captures any future epic-UX exploration; a separate task (doc-sync sweep) records the D5/T5 revision notes.

The ask: remove `parents` end to end. Core: drop the field from `Task` state and stop mapping it in replay — but replay must *tolerate* stored events that carry the field (additive posture: readers ignore unknown fields; stored bytes are never rewritten, no event-schema version bump — removal is reader-side only). Daemon: drop the parameter from create_task/update_task in ops.go, mcp.go, api.go; the batch tmp-ref cycle check now covers depends_on only. CLI: drop --parents from create/update. Views: drop the Parents line. TUI: remove any parents rendering.

Acceptance: `Parents` absent from core state and every verb schema; a table-driven replay test proves an event whose payload carries `parents` replays without error and produces state without it; replaying the real data branch still succeeds (existing migrator-written parent payloads become inert history); existing tests updated; `make test lint` green.

Pointers: internal/core/state.go:38, internal/core/replay.go:205 and :258-259, internal/daemon/ops.go:66,76,147,197,237,253, internal/daemon/mcp.go:408,621, internal/views/views.go:346-347, cmd/tuhdoo/write_cmds.go (--parents flags), internal/event/catalog.go.

Constraints: stored event bytes never rewritten (T3); no version bump; boring Go; one PR.

## History

### 2026-08-05 21:58 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-wzrg/remove-parents-edge`
- PR: <https://github.com/brandonbews/tuhdoo/pull/37>

Landed via PR #37 (squash-merged, checks green). parents removed from core state, replay mapping, event payload structs, all daemon verb schemas (ops/mcp/api), CLI flags, TUI rendering, and views. Stored events carrying parents replay tolerated — the field rides the event layer's Unknown map and re-encodes byte-identically, so no version bump was needed (T3 additive read in reverse). New table-driven tolerance test in internal/core/replay_test.go. Note for the doc-sweep claimant (tuh-01KZ9Y3THHH5B8GT22T5D72HVF): newly written task.created/task.updated events no longer carry a parents key — a wire-shape change for new events only, worth a line in the D5/T5 revision notes. The loop task (tuh-01KZ9Y3THHH5B8GT22T1A1WPYP) is now unblocked; its cycle logic covers a single edge type as intended.
