# t-01KYTRJQT44HGYXGR9C7C3R2GS — Escalation ergonomics: resolve answered-out-of-band, and the name itself

- Status: done
- Priority: 2
- Labels: `mcp`, `design`, `escalations`
- Created: 2026-07-31 00:18 UTC by `4099114+brandonbews`

## Description

Context (steering direction, 2026-07-30): escalations are the part of tuhdoo Brandon values most — the "does the system need me" signal. But his intended flow has a gap: when an agent raises an escalation AND the human answers it out of band (in the live chat session, not the TUI), the agent has no way to resolve the escalation — `answer` exists only on the daemon HTTP steering API (TUI path). The open escalation then lingers in the inbox even though it's settled, polluting exactly the signal escalations exist to provide. The MCP surface (T5, ten verbs) has no resolution verb.

The ask, part 1 (buildable, needs a T5 design-doc revision since the verb set is contractual): give agents a way to record an out-of-band answer on an escalation — e.g. a `resolve_escalation` MCP verb, or an argument on an existing verb. Requirements:
- The answer text must land in the ledger with attribution, e.g. answered_by the human principal with a marker that it was relayed by the agent (never silently deleted — whoever monitors the inbox later should see question AND answer, not a vanished entry). Consider actor semantics: the event actor is the agent sub-principal, the answer is attributed to its root human — decide and document.
- Works for blocking and non-blocking escalations; unblocks readiness exactly like a TUI answer.
- Guard rails worth considering: only the raising agent's root human's sub-principals? Any claimant of the task? Decide, document in the design doc revision.

The ask, part 2 (design question, decide before or with part 1): Brandon is considering renaming "escalation" to something more specific like "clarifying blocker" — the current name overstates severity and understates that most are questions. Naming touches the MCP verb (`escalate`), TUI/CLI copy, views, and docs — but NOT stored event bytes (T3: event schema names stay; display/verb naming can diverge). Scope the rename honestly: if it's display-copy only, it's cheap; if verbs change, it's a T5 revision. Make a recommendation and get it steered before implementing.

Acceptance: design-doc revision(s) landed per convention; new/changed MCP behavior covered by mcp_cli_test-style tests; TUI inbox shows out-of-band-answered escalations as answered with the relayed answer text; make test lint green.

Constraints: boring Go; stored event bytes never rewritten (additive events only); fewer-better-verbs bar applies — if this can ride an existing verb well, prefer that.

## History

### 2026-07-31 00:59 UTC — note from `4099114+brandonbews`

Design steered by Brandon and the T5 revision is landed + pushed (commit affb078, docs/design/002-technology.md + CLAUDE.md). Decisions: (1) new eleventh MCP verb `relay_answer` — rides the existing escalation.answered event, no new event type; (2) attribution via additive answered_by payload field, daemon-derived from the session principal's root (never agent-supplied); envelope actor stays the agent, replay marks "relayed by" when they differ, old events fall back to envelope actor; (3) guard rails: open escalations only on the agent surface, no claim required, any session may relay; (4) the name "escalation" stays everywhere — verb, events, docs; only the TUI inbox header softens to "Needs Input", deliberately not naming who answers (a future answerer may not be a human). Implementation not started yet. Remaining: catalog answered_by field + golden regen, replay handler + RelayedBy on core.Escalation, opRelayAnswer in ops.go (open-only guard, root derivation), MCP tool registration, escalationJSON relayed_by on both daemon and CLI sides, views relayed attribution (bump FormatVersion 1→2 — old generators would render relayed answers differently), CLI/TUI answered rendering, "Needs Input" header in top.go secs + render.go renderOpenEscalations, tests per mcp_cli_test style.

### 2026-07-31 01:08 UTC — run by `4099114+brandonbews` — done

- Branch: `main`
- Commits: `affb078`, `5fff97b`, `4d37d1f`

Shipped in three commits on main. affb078 (design, landed first per convention): T5 revised to eleven verbs — relay_answer records an out-of-band escalation answer; attribution semantics and guard rails documented in 002-technology.md, T7 notes the "Needs Input" inbox header, CLAUDE.md law updated. 5fff97b (implementation): additive answered_by field on escalation.answered (stored bytes untouched; golden regenerated); replay attributes to answered_by with envelope-actor fallback for pre-field events and sets RelayedBy when they differ; opRelayAnswer derives the root principal server-side (never agent-supplied), rejects already-answered escalations with a 409-shaped tool error; HTTP steering path unchanged in behavior (still last-answer-wins) but now stamps answered_by; eleventh MCP tool registered; relayed_by exposed on escalationJSON both sides; views/CLI/TUI detail render "(relayed by ...)" and view format bumped 1→2; agent-protocol.md gains the answered-out-of-band flow. Tests: TestMCPRelayAnswer (real SDK client: blocking flow, attribution on the event bytes, readiness unblock via claim_next, re-relay + unknown-escalation rejections, root-session self-relay with no marker), TestAnswerAttribution replay table, views golden with a relayed answer, TUI detail assertion. 4d37d1f: inbox header "Open escalations" → "Needs Input" in TUI and escalations command; entity name kept everywhere per the steering decision. make test lint green at each commit. Two readings to be aware of: (1) "TUI inbox shows out-of-band-answered escalations as answered" — implemented as: answered escalations leave the Needs Input section (settled questions stop polluting the signal, per this task's own rationale) and render as answered with the relayed text in the task detail screen and the escalations command's Answered section; if the inbox itself should list recently-answered entries, that's a small follow-up. (2) The MCP behavior tests live at the daemon layer (internal/daemon/mcp_test.go, real SDK client over the real socket); the shim-level mcp_cli_test asserts relay_answer is in the mirrored surface but has no shim round-trip of the verb — the shim is a transparent proxy already round-tripped by other verbs. Deferred deliberately: the daemon running on any given machine needs a rebuild+restart (make build) before relay_answer is live there; no restart was performed.
