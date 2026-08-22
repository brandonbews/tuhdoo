# One-shot steering surface: two-rule contract in design docs, serialized backlog/escalations output

`tuh-01KYWWH4DZH4TR7ASVGTDBT14P`

- **Status:** done
- **Priority:** none
- **Labels:** `cli` `design`
- **Created:** 2026-07-31 20:05 UTC by `brandon/claude-code-1`

## Description

Context: Decided in the 2026-07-31 grill cycle. The original capture asked keep-or-kill on `tuhdoo backlog`/`tuhdoo escalations`; grilling dissolved that framing. The real complaint: run bare, these render a designed digest (sections, counts, waiting-prose) — a fossilized older cousin of the TUI, i.e. a second look at the same information that must be maintained and that every TUI grill has to rule on ("one-shot output untouched" clauses, twice on 2026-07-31 alone). Functionality stays; the second LOOK dies. The CLI surface already has a principled line documented in its help text: the work loop (claim/finish_run/release) is deliberately absent because leases are session-bound.

The ask, two parts:

1. Design-doc revision note (docs/design/002-technology.md near T5, or 001 if it fits the decision register better — follow the Cycle 2 in-place amendment pattern) stating the two-rule contract plus output contract:
   - Rule 1 (steering parity): steering capabilities — read state, shape the backlog, create/update/answer — ship in both TUI and one-shot CLI form. A steering feature added to one without the other is a decision someone must make explicitly, not drift.
   - Rule 2 (work loop is session-only): claim/finish_run/release are never one-shot commands; leases renew only while a live MCP session holds them (already in the help text — promote it to the design record).
   - Output contract: one-shot output is serialization, not design — stable, plain, line-oriented; it changes when the data model changes, never because the TUI was redesigned. No future TUI design decision owes the one-shots a clause.
2. Re-render `backlog` and `escalations` in that register: header row, aligned columns, one record per line, a STATE column instead of section headers (so `tuhdoo backlog | grep ready` works), no ANSI styling in the output at all (which also deletes the TTY-vs-pipe degradation surface for these commands). Everything shown today stays available — all states, priority, labels, blocked waiting-reasons (condensed to a column; dep IDs / escalation IDs rather than prose), escalation blocking flags, task/actor/timestamp attribution. Aim for the kubectl-get / git-branch -v register: readable to an eye, trivial for grep, zero opinions. `status` and `task <id>` are covered by the contract note but their output is out of scope here — reshape only if trivially cheap; otherwise leave for a follow-up capture.

Acceptance:
- The design revision note exists, in-place, following the established amendment pattern.
- `tuhdoo backlog` and `tuhdoo escalations` emit the serialized form: golden tests replaced accordingly; identical bytes TTY vs piped; a grep for a state name selects exactly that state's rows.
- No behavior change to the TUI, MCP surface, or internal/views (the committed markdown views are a different surface with their own contract).
- CLI help text updated where it describes these commands; make test lint green from the repo root.

Pointers: cmd/tuhdoo/commands.go, cmd/tuhdoo/snapshot.go (blockedReason — the prose reason-namer these columns condense), cmd/tuhdoo/render.go, cmd/tuhdoo/cli_test.go, main.go (help text), docs/design/002-technology.md (T5, Cycle 2 amendment pattern).

Constraints: eleven MCP tools untouched (T5). internal/views untouched. Completeness is the non-negotiable: the serialized form must not show less than the digest did.

## History

### 2026-07-31 21:24 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-07-31 22:09 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open

### 2026-08-01 01:15 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-t14p/one-shot-serialized-output`
- PR: <https://github.com/brandonbews/tuhdoo/pull/8>

Merged to main (PR #8, squash). T7 in docs/design/002-technology.md carries the in-place revision note: Rule 1 steering parity, Rule 2 work-loop-is-session-only, and the output contract (one-shot output is serialization, not design). `tuhdoo backlog` and `tuhdoo escalations` now emit header + one aligned row per record via text/tabwriter: STATE column (grep-selectable, hyphenated one-token values incl. on-hold/in-progress), waiting-reasons as dep:/esc: IDs, "-" for empty cells, zero ANSI by construction (printers no longer take a colors argument — identical bytes TTY vs pipe). Done/archived got full rows (digest showed only counts). Byte-exact goldens in cmd/tuhdoo/oneshot_golden_test.go. One deliberate narrowing: escalations rows carry the task ID but not the task title annotation the digest had — the ID is the join key; story lives in `tuhdoo task <id>`. status/task-id output untouched per scope (follow-up capture territory). make test lint green.
