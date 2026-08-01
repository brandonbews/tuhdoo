# tuh-01KYWVNF91Y7H9GK0X1RAE2SBW — Audit: agents via MCP can perform the main steering actions users ask for

- Status: open — in progress, claimed by `brandon/claude-code-1`
- Priority: 0
- Labels: `mcp`, `dx`, `docs`
- Created: 2026-07-31 19:50 UTC by `brandon`

## Description

Context: Brandon's stated goal (2026-07-31): tuhdoo should be usable 100% agentically — everything he can do from the TUI (steer, move tasks around, work the inbox, answer questions) he wants to be able to do by asking an agent in chat. This task is the coverage audit that grounds that goal. The MCP surface is deliberately eleven verbs (T5), so the audit does not build new capability — but a gap finding here is not a dead end: it is the evidence that feeds a design revision (grill cycle), which Brandon has already said he wants when parity is genuinely missing. Current belief from code reading: most of it is already reachable — create_task accepts status inbox (title-only capture is legitimate there), update_task changes status/priority/labels/edges/title/description and status cancelled is the delete (event-sourced ledger, no hard deletes), relay_answer records out-of-band human answers, get_backlog now serves ready + inbox + held. Verify that belief instead of trusting it.

The ask: enumerate the main user-asks and verify each is achievable end-to-end through the eleven tools as an agent would actually do it (through the shim against the live daemon). At minimum: capture to inbox (title-only); cancel an inbox item ("delete"); promote inbox→open with a prompt-bar description; hold and unhold; reprioritize; retitle/redescribe; add/remove parent and depends_on edges; answer an open escalation via relay_answer; read any single task; read backlog state. Known probe point: get_backlog omits in-progress/blocked/done tasks — an agent asked "what's in progress right now?" or "what did we finish this week?" may have no MCP path at all; that is the most likely real parity gap, so probe it explicitly. Use the TUI's sections (needs input / in progress / ready / blocked / inbox / held / done) as the parity checklist — full-agentic means an agent can answer for each section and act on its rows.

Fixes in scope: tool descriptions in internal/daemon/mcp.go, guidance in docs/agent-protocol.md, and tests for any verified-but-untested path. If a genuine capability gap needs a new verb or a new input field — STOP and escalate with the concrete gap list; that becomes a T5 design-doc revision (grill cycle), and the escalation should say so. Do not quietly build around gaps.

Acceptance:
- A parity checklist mapping each user-ask → tool call(s) → verified (test or transcript) or GAP, recorded in the finish_run summary or a task note.
- Daemon/MCP tests exist for at minimum: create into inbox, update to cancelled, promote inbox→open. Add tests for any other path the audit found untested.
- Any tool-description or agent-protocol doc fixes landed; make test lint green from the repo root.
- If gaps were found: an escalation (or follow-up inbox task) exists carrying the gap list toward a design revision — parity gaps must not evaporate when this task closes.

Pointers: internal/daemon/mcp.go (tool registrations, ~line 381 on), internal/daemon/ops.go (opCreateTasks, opUpdateTask, opBacklog), internal/core/state.go (status vocabulary and transition permissiveness), docs/agent-protocol.md, docs/design/002-technology.md (T5), cmd/tuhdoo/top.go (the TUI sections that define the parity bar).

Constraints: eleven tools stay eleven within this task (T5) — gaps escalate with evidence, they don't get built here. Host-agnostic (T2) and event-schema (T3) rules untouched.

## History

_No activity yet._
