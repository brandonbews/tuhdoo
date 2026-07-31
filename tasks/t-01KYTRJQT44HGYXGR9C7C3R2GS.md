# t-01KYTRJQT44HGYXGR9C7C3R2GS — Escalation ergonomics: resolve answered-out-of-band, and the name itself

- Status: open — ready
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

_No activity yet._
