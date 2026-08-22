# Status vocabulary: cancelled replaces archived everywhere; "on hold" stays as the one display mapping

`tuh-01KZ0ES83SFH6MKWP82Y2HNTPK`

- **Status:** done
- **Priority:** none
- **Labels:** `cleanup` `vocabulary`
- **Created:** 2026-08-02 05:22 UTC by `brandon/claude-code-1`

## Description

## Context

A datashape exploration (2026-08-01, session with Brandon) found dual vocabulary on the intent axis: the stored status `cancelled` renders as "archived" and `held` as "on hold", with the mapping duplicated in two places (`humanStatus` in internal/views/views.go:552 and cmd/tuhdoo/render.go:85) plus a reverse input alias (`archived`->`cancelled`) in cmd/tuhdoo/write_cmds.go:241. "Archived" also collides semantically with `done` — both are terminal and kept forever, yet backlog.md renders "Done" and "Archived" as sibling sections.

Grilled with Brandon 2026-08-01; this description is the settled design and supersedes the original capture (including its "blessed-state write-up may override" clause — this grill is the write-up).

**Settled doctrine:** displayed status words are the stored words, with exactly two sanctioned exceptions: (1) `held` renders as "on hold" — Brandon rejects "held" as a human-facing word; the stored value stays `held` (single-token, no schema change); the mapping must be defined in exactly one place. (2) NEEDS INPUT names the open-escalations section (settled 2026-07-31). The archive vocabulary dies entirely: `cancelled` is displayed as cancelled, and the cancel verb reads as cancel.

## The ask

**Commit 1 — docs-first design revision** (in place, dated, per the Cycle-2 pattern):
- docs/design/002-technology.md T7 "Archive is the porcelain word for `task.cancelled`" paragraph: rewritten with a revision note — archive is retired; cancelled is both the stored and displayed word; record the two-exception doctrine and rationale (archived collided semantically with done; the display mapping was duplicated; the "history stays on the ledger" confirm copy now does the reassurance work that the archive word was doing).
- Same file, one-shot serialization contract: STATE token `archived` -> `cancelled`; `on-hold` stays (it is the sanctioned display token for `held`).
- Same file, shelves paragraph: "on hold" display survives; reword its justification from "same porcelain/plumbing split as archive/cancelled" to the sanctioned-exception doctrine.
- docs/design/001-core-design.md line ~73: update the T7 cross-reference ("'Archived' remains the porcelain word" and "archive works on every non-terminal status" -> cancel wording).
- docs/agent-protocol.md: verified at grill time to have no archive/on-hold vocabulary — no changes needed.

**Commit 2 — code** (may be the same PR; docs commit lands first):
- Delete the `archived`->`cancelled` input alias in write_cmds.go; the usage line becomes `--status open|inbox|held|done|cancelled`. Plumbing word `cancelled` keeps working everywhere it already does.
- Collapse the two `humanStatus` copies into one shared definition containing only the `held` -> "on hold" entry; both call sites (views markdown, CLI render) use it. Implementer picks a boring home; do not create a new package just for this.
- Views markdown: "## Archived" section -> "## Cancelled", README count-table header "Archived" -> "Cancelled"; "On hold" stays. Bump FormatVersion to 6 with a changelog line, per the existing pattern in views.go.
- TUI: the archive verb becomes cancel and the key moves `a` -> `c` (`c cancel` is self-describing; the 2026-08-01 objection that `c` "read as cancel, not archive" dissolves because the verb now IS cancel; `c` is currently unbound in nav mode). Confirm copy: "cancel <title> (<id>)? y/n — history stays on the ledger". Status message: `cancelled <id>`. Update all footer legends and section-bar key hints. ON HOLD bar unchanged. Rename cmd identifiers (`archiveTask`, `modeConfirmArchive`, etc.) to cancel wording; comments recording the naming history may keep the old words.
- CLI `tuhdoo backlog` STATE column: `archived` -> `cancelled`; `on-hold` stays.

## Acceptance

- `rg -i archiv cmd/ internal/` finds no emitted strings and no identifiers (comments recording history are fine).
- Exactly one definition of the held->"on hold" mapping in the repo (rg confirms).
- Views golden/table tests updated to the new bytes; a test asserts FormatVersion is 6.
- TUI interaction/golden tests cover the `c` key opening the cancel confirm, the confirm copy, the `cancelled <id>` status message, and updated legends/hints; `a` no longer archives.
- No surface accepts or emits `archived`; `on hold` appears only via the single shared mapping.
- make test lint green from repo root.

## Constraints

- Stored bytes, event schema, and API/ledger vocabulary are untouched — `held` stays the stored word (no upcaster needed); display surfaces and input aliases only.
- Do NOT fold in classifier unification (separate task) or any design-shaped changes beyond the T7 revision described here.

## History

### 2026-08-02 06:38 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open

### 2026-08-02 06:57 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-ntpk/status-vocabulary`
- PR: <https://github.com/brandonbews/tuhdoo/pull/14>
- Commits: `a1bac90`

Landed as PR #14 (squash a1bac90 on main). Docs-first: 002-technology.md supersedes the archive-porcelain decision in place (displayed status words = stored words; sanctioned exceptions: held->"on hold", NEEDS INPUT), STATE tokens + shelves + 001 D5 cross-ref updated; agent-protocol.md archive verbs reworded to cancel — the grill-time "no changes needed" claim was wrong (grep missed bare "archive"). Code: archived input alias deleted; single views.HumanStatus (held->"on hold" only) used by views/CLI/TUI; views render "## Cancelled" at FormatVersion 6; TUI key a->c with cancel wording throughout (confirm copy keeps "history stays on the ledger"); STATE column and MCP update_task description say cancelled. All acceptance greps hold; make test lint green. Sibling tasks reconciled at claim time: focus-ring ("keep p and c") and history-view (done/cancelled shelf, c key). Daemon redeployed after merge.
