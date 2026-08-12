# Plan-materialization flow end-to-end; decomposition-quality prompting conventions

`tuh-01KZA0VT234XJYVZWT8Q19P9QM`

- **Status:** cancelled
- **Priority:** 0
- **Labels:** `design` `protocol`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Cancelled as subsumed, 2026-08-06 triage grill (Brandon). The question was "from a human intention to a well-formed task DAG: what does the flow look like, and what prompting conventions make agent decomposition good?" — fact-checked against current code and docs, both halves resolved:

Flow: the mechanism shipped end-to-end and this capture predates most of it. Capture is create_task into inbox (title-only legitimate; TUI quick-capture). Triage/promotion is the 2026-07-31 "capture cheap, promote deliberate" model — inbox→open requires a prompt-quality description in the same call. Decomposition is batch create_task with tmp: refs, the whole DAG landing atomically (T5); a container depends_on its children (D5, edge grill 2026-08-05); D5 explicitly rejects epic/story/subtask tiers in favor of recursive decomposition. Steering is update_task fields plus escalation answers. Nothing mechanical is missing.

Conventions: the descriptions-are-prompts five-part section (context/ask/acceptance/pointers/constraints) in docs/agent-protocol.md applies to every task created as or promoted to open — decomposed children included — and the decomposition section covers the mechanics plus "note why you split it". Distribution of those conventions to foreign repos is owned by the tuhdoo-protocol-command capture (tuh-01KZANB3J4YYH09F0Z6FSZQ5CD).

What was genuinely missing — a human-facing narrative of the whole flow — was folded into the user-facing-docs capture (tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY) before this cancellation; that capture now owns the intention→DAG story and should grill alongside the marketing/docs site task (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2, site now high priority).

Rejected at the grill: promoting a standalone flow-doc task now (would design the same prose twice, ahead of the user-docs grill that owns it); holding for decomposition-quality evidence (no such evidence exists — dogfood decompositions have been fine, and a gate would just shadow the user-docs capture's own timeline).

## History

### 2026-08-06 22:56 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→cancelled
