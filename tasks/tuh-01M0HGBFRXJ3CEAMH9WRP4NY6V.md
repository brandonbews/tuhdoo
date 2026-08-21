# Priority direction is a live agent foot-gun: name the P0-is-critical collision at both doc sites

`tuh-01M0HGBFRXJ3CEAMH9WRP4NY6V`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `docs` `adoption-friction` `agent-protocol`
- **Created:** 2026-08-21 06:32 UTC by `brandon/claude-code-1`

## Description

## Context (adopter report, 2026-08-21)

An agent bootstrapping tuhdoo in a second repo assigned priorities p0–p5 with the industry P0-is-most-critical instinct — exactly backwards for tuhdoo, where **higher number claims first**. It caught and fixed the inversion itself, but the first pass shipped wrong. The direction is already documented in both places an agent would look (`internal/daemon/ops.go:64` schema string: "higher claims first; 0 is the default"; `docs/agent-protocol.md:37`: "higher number wins") — stating the rule was not enough, because neither site names the colliding convention the agent walks in with.

Also noted: priority is an unbounded plain int end-to-end — no clamp, no range validation; the TUI badge is `fmt.Sprintf("p%d")` (`cmd/tuhdoo/top.go:1812`) and views print `%d`, so p5/p500/negatives all store and render. p0–p2 was only ever this repo's usage convention. That open-endedness seems fine as-is; it is context, not part of the ask.

## The ask

At both doc sites, add one clause that names the trap, not just the rule — e.g. "higher number = more urgent (note: the reverse of the P0-is-critical convention)." Keep each site to a sentence; the fix is naming the wrong instinct at the point of use, not more prose.

- `internal/daemon/ops.go:64` — the jsonschema string on `Priority` (this is what MCP agents actually see at call time)
- `docs/agent-protocol.md:37` — the claim-selection paragraph
- Check whether any other agent-facing surface states priority direction (e.g. views' backlog.md header, get_backlog description) and treat it the same if so.

## Explicitly out of scope (decided against, 2026-08-21 — overrule only via grilling)

Flipping the semantics to P0-highest was considered and recommended against: stored event bytes are never rewritten (T3), so a new comparator would silently invert the *intent* of every existing ledger's priorities, and there is no upcaster for intent. Higher-number-wins is a legitimate convention; the failure mode is the unnamed collision, which this task fixes. If triage disagrees, that reversal is a design-doc revision and needs a /grill-me cycle, not this task.

## Acceptance

- Both sites (plus any others found) name the P0 collision in one clause.
- Schema-string change is verified to surface in the actual MCP tool listing (rebuild + inspect, or the existing test that snapshots tool schemas if one exists).
- `make test lint` green; one PR.

## History

_No activity yet._
