# Flip priority semantics to P0-highest (de facto standard); grill the default-value wrinkle first

`tuh-01M0HGBFRXJ3CEAMH9WRP4NY6V`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `docs` `adoption-friction` `agent-protocol`
- **Created:** 2026-08-21 06:32 UTC by `brandon/claude-code-1`

## Description

## Context (adopter report 2026-08-21, direction revised same day)

An agent bootstrapping tuhdoo in a second repo assigned priorities p0–p5 with the industry P0-is-most-critical instinct — exactly backwards for tuhdoo, where **higher number claims first**. It caught and fixed the inversion itself, but the first pass shipped wrong, despite the direction being stated at both doc sites an agent would look (`internal/daemon/ops.go:64` schema string; `docs/agent-protocol.md:37`).

P0-highest (lower number = more urgent) is confirmed the de facto standard across task/bug/incident tracking: the P0–P4 product/incident scheme, PagerDuty P1–P5, Bugzilla P1–P5, Chromium Pri-0, Linear (1=Urgent…4=Low, 0=no priority). Higher-number-wins survives mainly in OS-scheduler/queue-API contexts. tuhdoo is a backlog whose users are LLM agents carrying the P0 instinct; the live misread is the evidence.

**Decision context on cost (Brandon, 2026-08-21):** adoption today is exactly two ledgers, both Brandon's (this repo + a handful of tasks in the second project). No external adopters. The earlier "no upcaster for intent" objection only bites when ledgers with unknown-intent priorities exist — none do. **It will never be cheaper to flip than now.** Stored event bytes still never get rewritten (T3): the migration is flip the comparator in core, hand-correct the few live tasks via ordinary update events (done/cancelled priorities are inert), rebuild + restart every daemon. Mixed-version peers reading one ledger would disagree on order, so rebuild everywhere the same day — trivial at two ledgers, the exact reason to do it now.

## The ask

Route through a /grill-me cycle first (design-shaped; touches the deterministic core's ordering rule and the agent protocol). The grill must settle at minimum:

1. **The default-value wrinkle (the real design question):** today `0` = default AND least urgent, so unprioritized tasks sort last for free. Naive flip makes default-0 *most* urgent — every unprioritized capture outranks everything. Candidate shapes: Linear-style (0/absent = no priority, sorts last; 1 = highest) vs a non-zero default. Sweep for other buried assumptions that `higher = more urgent` (sorting in views/TUI, claim_next selection, any test fixtures encoding direction).
2. Migration plan per above (comparator flip + hand-correction of live tasks + same-day rebuild of all daemons; enumerate the live tasks needing correction at execution time).
3. Doc/schema updates: `internal/daemon/ops.go:64` jsonschema string, `docs/agent-protocol.md:37`, any other agent-facing surface stating direction (views' backlog headers, get_backlog description). After the flip these should state the P0-highest rule plainly — and the naming-the-collision clause from the earlier framing of this task becomes unnecessary (the semantics will match agent instincts, which is the whole point).

## Superseded framing

This task originally proposed keeping higher-wins and merely naming the P0 collision in the docs, with the flip "explicitly out of scope" on migration-cost grounds. That reasoning assumed unknown-intent ledgers; at two known ledgers it doesn't hold, and Brandon has signaled intent to flip while it's cheap. If grilling nonetheless rejects the flip, fall back to the wording fix: add one clause at each doc site naming the reverse-of-P0 collision.

## Acceptance

- Design-doc revision note recording the decision (grilling convention), then implementation per its outcome.
- All agent-facing surfaces state the final rule consistently; no surface still implies the old direction.
- Live tasks' priorities corrected in the same change window as the comparator flip; daemons rebuilt/restarted on every machine running one.
- `make test lint` green; one PR per coherent piece.

## Pointers

- `internal/daemon/ops.go:64` — Priority jsonschema string
- `docs/agent-protocol.md:37` — claim-selection ordering paragraph
- `internal/core/` — deterministic core ordering (comparator lives here; find exact site during grilling sweep)
- `cmd/tuhdoo/top.go:1812` — TUI `p%d` badge (unbounded int, renders anything)
- Priority is a plain unbounded int end-to-end — no clamp anywhere; p0–p2 was only ever this repo's usage convention.

## History

### 2026-08-21 06:50 UTC — edit by `brandon/claude-code-1`

retitled · description edited

### 2026-08-21 06:50 UTC — edit by `brandon/claude-code-1`

description edited
