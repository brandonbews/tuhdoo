# Task view id line shows only the short form (T7 revision: the full ULID leaves the TUI)

`tuh-01KZ2HCCBM0RY70GJKMKHFHS07`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `tui` `ux` `design-revision`
- **Created:** 2026-08-03 00:46 UTC by `brandon/claude-code-1`

## Description

Context: the 2026-08-02 grill session on ID disparity re-affirmed T7's core (canonical IDs stay ULIDs — canonically-short IDs either need the central counter the no-server design forbids, or accept collision odds whose blast radius is a ledger-wide read-only halt). What changed: T7 currently mandates that the TUI detail screen "keeps the full ULID exactly once, dimmed on its `id` line, as the copyable canonical form" — and that line is now the last full-ID surface humans see. The session decided the full ULID leaves the TUI entirely: the fragment resolver makes the short form a complete key for every human entry point (tuhdoo task / update / answer / --parents / --depends-on all resolve fragments), and when a human genuinely needs the full form (cross-referencing an event filename or tasks/*.md path), one-shot `tuhdoo task tuh-xxxx` prints it.

The ask (two parts, one PR):
1. Revise T7's "Short IDs are the human contract" paragraph in docs/design/002-technology.md in place with a dated revision note (Cycle 2 amendment pattern): the detail screen's `id` line shows the short form only; the full ULID has no TUI surface; one-shot output stays byte-identical with full IDs (plumbing, unchanged). Record the accepted consequence: short-tail ambiguity grows with total task count (~38% chance some pair of tasks shares a 4-char tail by ~1,000 tasks); the resolver already handles it loudly (ambiguity errors listing candidates), and the working-set-retirement open question (docs/design/open-questions.md, Cycle 4), if built, bounds the pool at working-set size.
2. Change the TUI: the task-view id field renders the short form dimmed (shortID(t.ID)) instead of t.ID.

Acceptance:
- docs/design/002-technology.md T7 revised in place, dated revision note, content per above.
- cmd/tuhdoo/top.go:848 renders the short form; no TUI code path renders a full task ID anywhere (grep the TUI for t.ID-style renders to confirm).
- TUI golden/test updates as needed; one-shot output byte-identical — cmd/tuhdoo/oneshot_golden_test.go goldens must NOT change; committed markdown views untouched (full IDs in filenames/link targets are plumbing).
- make test lint green.

Pointers: cmd/tuhdoo/top.go:848 (the id field), top.go:1433 (shortID), top.go:1440 (comment citing the old T7 wording — update it), docs/design/002-technology.md "Short IDs are the human contract" + the tuh- prefix paragraph below it. Related but separate: tuh-01KYXNTBJRKM8YDW6QG6ED7T5Y (dedupe shortID between cmd/tuhdoo and internal/views) — coordinate if both are in flight, don't merge scopes.

Constraints: display-layer only — stored bytes, events, HTTP API, MCP verbs untouched. Escalation IDs are out of scope (bare ULIDs, one-shot only today).

## History

### 2026-08-03 05:20 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-hs07/short-id-only-tui`
- PR: <https://github.com/brandonbews/tuhdoo/pull/19>
- Commits: `3abb75a`

Merged (PR #19, squash). T7 "Short IDs are the human contract" revised in place with a dated 2026-08-02 note: the detail screen's id line shows the short form only, the full ULID has no TUI surface, one-shot output unchanged; accepted consequence (~38% tail-collision chance by ~1,000 tasks, loud resolver errors, working-set retirement bounds the pool) recorded. top.go id field now renders shortID(t.ID) dimmed; the stale shortID doc comment citing the old wording updated. TestTopRowsShowShortIDs now fails on any full-ULID occurrence in the detail view. Pre-work compatibility check: all task pointers (top.go:848, :1433, :1440) still held despite PRs #15/#16; existing TUI goldens use short fixture IDs so were unchanged by construction; oneshot goldens and markdown views untouched. The shortID-dedupe task (tuh-01KYXNTBJRKM8YDW6QG6ED7T5Y) was not in flight — no coordination needed; when claimed, it inherits the updated comment wording in top.go.
