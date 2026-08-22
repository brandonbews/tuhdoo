# shortID is duplicated between cmd/tuhdoo and internal/views — extract one shared helper

`tuh-01KYXNTBJRKM8YDW6QG6ED7T5Y`

- **Status:** done
- **Priority:** none
- **Labels:** `cleanup` `go`
- **Created:** 2026-08-01 03:27 UTC by `brandon/claude-code-1`

## Description

Context: shortID (last-4-of-ULID display form) exists as two byte-identical copies: cmd/tuhdoo/top.go:1374 and internal/views/views.go:513. Captured during the 2026-08-01 npm-provenance session as drive-by cleanup; promoted at triage 2026-08-01.

The ask: extract one shared helper and delete both copies. Suggested home: internal/event, next to IDTime — it's an ID-formatting concern and event is a leaf package, so no import-cycle risk; implementer may pick a better boring home if one is obvious, but do not create a new package just for this.

Acceptance: exactly one shortID definition in the repo (rg confirms); both call sites use it; existing tests green with no golden changes (behavior is identical by construction); make test lint green.

Constraints: pure refactor — no behavior change, no new package, boring Go.

## History

### 2026-08-01 05:46 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→open

### 2026-08-03 06:02 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-7t5y/shortid-shared-helper`
- PR: <https://github.com/brandonbews/tuhdoo/pull/21>
- Commits: `7980500`

Landed via PR #21 (squash-merged to main 2026-08-03). Pure refactor exactly as scoped: both byte-identical shortID copies deleted; the one definition is event.ShortID in internal/event/id.go next to IDTime (the task's suggested home). All call sites across cmd/tuhdoo and internal/views use it; doc comment merges the TUI copy's fuller rationale. No behavior change, no golden changes; make test lint green.
