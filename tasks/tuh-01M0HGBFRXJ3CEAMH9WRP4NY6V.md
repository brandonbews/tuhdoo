# Flip priority to true P0-highest: 0 = most urgent, absent = unprioritized (sorts last); badge color ramp

`tuh-01M0HGBFRXJ3CEAMH9WRP4NY6V`

- **Status:** done
- **Priority:** 3
- **Labels:** `go` `agent-protocol` `docs` `tui`
- **Created:** 2026-08-21 06:32 UTC by `brandon/claude-code-1`

## Description

Context: an agent bootstrapping tuhdoo in Brandon's second repo assigned p0-p5 with the industry P0-is-most-critical instinct — backwards for tuhdoo's higher-number-wins. P0-highest is the de facto standard (PagerDuty, Bugzilla, Chromium, Linear) and tuhdoo's users are LLM agents carrying that instinct. Two ledgers exist, both Brandon's — it will never be cheaper to flip than now. GRILLED 2026-08-21 (Brandon); the shape below is decided — build it, don't re-litigate.

**Decided shape — true P0-highest with nullable priority:**
- `0` = most urgent; ascending = less urgent; unbounded int, no clamp (unchanged).
- ABSENT = unprioritized: sorts after all prioritized tasks, renders with no badge. Key fact making this cheap: the create payload already serializes priority with omitempty, so every historically unprioritized task stores NO priority field — absent already means "nobody prioritized this" in the stored bytes. No stored event bytes are rewritten (T3); this is an in-memory reinterpretation.
- Explicit p0 must be storable, so the create payload's priority becomes pointer-typed (plain int would omitempty-drop a meaningful 0). A has-priority distinction plumbs through state/replay/daemon requests/views/TUI — boring Go, a *int or a bool, nothing clever.
- update_task: nil still means "leave unchanged"; consequence, ACCEPTED: a set priority cannot be cleared back to none (no sentinel invented — set a large number for "least urgent" instead).
- Ordering everywhere: prioritized ascending by number, ULID order within a priority, unprioritized last (ULID order among themselves).
- Rejected alternatives, for the record: Linear-style (0=none, 1=highest) keeps the observed trap — an agent writing p0-as-critical gets silently bottom-sorted; non-zero default is a magic number.

**TUI badge ramp (decided):** p0 red, p1 orange (256-color rung, e.g. index 208; falls back to yellow on the 16-color floor — p1/p2 collision there accepted), p2 yellow, p3+ the same dim gray, unprioritized = no badge. Slot into the existing capability ladder (cmd/tuhdoo/selection.go, render.go:15-27); pin exact codes with golden tests like the selection-bar ladder. ACCEPTED drop: the current yellow attention badge on Priority==0 ready rows (top.go:1813) disappears — nothing nags unprioritized ready tasks anymore; prioritization happens at triage/promotion.

The ask:
1. Core flip: comparator at internal/core/state.go:338 (serves both claim_next and get_backlog), internal/views/views.go:133, cmd/tuhdoo/snapshot.go:98; introduce the has-priority distinction end to end. Sweep for any other buried higher-wins assumptions: backlog one-shot ordering (cmd/tuhdoo/commands.go), test fixtures encoding direction, TUI priority-input handling.
2. Agent-facing surfaces state P0-highest plainly, none imply the old direction: internal/daemon/ops.go:64 and :73 jsonschema strings; docs/agent-protocol.md:37 (embedded in the binary — same file); get_backlog tool description; views' backlog headers if direction is implied; main.go usage text; docs/ anywhere priority direction appears.
3. 002 gets a dated revision note where ordering is stated (T5 get_backlog row area), per the revision convention.
4. Migration, same change window (mixed-version peers disagree on order — no window where an old daemon runs against the flipped ledger): hand-correct stored nonzero priorities via ordinary update events in BOTH ledgers — this repo (at execution time enumerate; currently the held epoch-compaction task at p1, plus the four priorities assigned at the 2026-08-21 triage which are under OLD semantics: this task p3, the Vercel docs task p2, the two p1s — remap all to P0-highest intent, confirming values with Brandon); the second repo's p0-p5 spread re-mapped to its original P0-highest intent. Then `make build`, restart every daemon (both ledgers' daemons are on this machine; follow the CLAUDE.md deploy steps — TERM the pid, wait for exit, respawn), same day.

Acceptance: comparator flip + nullable priority with table-driven core tests; every agent-facing surface consistent with P0-highest; golden TUI tests pin the badge ramp and its 16-color fallback; live priorities corrected in both ledgers in the same window as the daemon rebuilds; `make test lint` green. One task, at most two PRs (core flip + docs/schema; TUI ramp may be the second).

## History

### 2026-08-21 06:50 UTC — edit by `brandon/claude-code-1`

retitled · description edited

### 2026-08-21 06:50 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-21 23:10 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority none→3 · labels +go +tui −adoption-friction

### 2026-08-22 00:05 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-ny6v/p0-highest-flip`
- PR: <https://github.com/brandonbews/tuhdoo/pull/81 https://github.com/brandonbews/tuhdoo/pull/82>
- Merged as: `d19adb9`, `342d09b`

P0-highest flip shipped end to end. PR #81 (squash d19adb9): task.created/task.updated v2->v3 — priority nullable (null = unprioritized, sorts last), lower number = more urgent, 0 the most; v2->v3 upcasters lift historical explicit-0 defaults to null (grill's omitempty assumption was false — event payloads always encode every field, so the T3 version-bump path was required, precedent 2026-07-31); one comparator core.MoreUrgent serves claim_next/get_backlog/views/TUI; all agent-facing surfaces restated (jsonschema strings, agent-protocol.md, steering.md, CLI usage); view format 10; 002 T3+T5 revision notes. PR #82 (squash 342d09b): TUI ready-badge ramp p0 red / p1 orange (256-color rung, yellow on 16-color floor) / p2 yellow / p3+ dim, unprioritized unbadged, held stays dim; golden-pinned; 002 T7 note; orange recorded as second sanctioned 16-color-law exception. Migration same window: v0.4.0 tagged -> GitHub Release + npm publish (trusted publishing); this repo's daemon rebuilt+restarted on v0.4.0; live priorities corrected by Brandon's decision (qct8->p1, 364k->p2, nzry->p2, held epoch-compaction->p2; flip task skipped, terminal). Panocash (second ledger, npm consumer): package.json bumped to ^0.4.0, reinstalled, daemon restarted on v0.4.0, p1-p5 inverted to p4-p0 restoring the bootstrap agent's original P0-highest intent (dep bump left uncommitted in its working tree for Brandon).
