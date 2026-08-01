# Audit: agents via MCP can perform the main steering actions users ask for

`tuh-01KYWVNF91Y7H9GK0X1RAE2SBW`

- **Status:** done
- **Priority:** 0
- **Labels:** `mcp` `dx` `docs`
- **Created:** 2026-07-31 19:50 UTC by `brandon`

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

### 2026-08-01 00:19 UTC — escalation from `brandon/claude-code-1`

> Parity audit found real MCP visibility gaps: agents cannot list in-progress, blocked, done, or cancelled tasks, nor open escalations. Do you want a T5 design revision (grill cycle) to add an orientation path — e.g. more get_backlog arrays or a /v0/state-shaped read verb?

Audit (PR #7) verified all ten steering write-paths work through the eleven tools, but read-side parity fails against the TUI's sections. Evidence, per section: (1) Needs Input — no verb lists open escalations; a blocking escalation removes its task from ready, so an agent can't even discover the task ID to get_task it; a non-blocking escalation is invisible in the task's backlog row. (2) In Progress — a claimed task vanishes from all three get_backlog arrays, and taskJSON carries no holder field, so "what's in progress and who holds it?" has no MCP path. (3) Blocked — dep-blocked and escalation-blocked tasks appear in no array. (4) Done/Archived — done and cancelled tasks appear nowhere; "what did we finish this week?" is unanswerable. All such tasks remain readable by known ID via get_task; the gap is discovery, not access. Also worth settling in the same revision: T5's sentence "curation (cancel, reprioritize, archive) is human work via CLI/TUI on the HTTP API" no longer matches code or intent — update_task mechanically accepts cancelled/priority from agents and the full-agentic goal depends on that; PR #7 already corrected agent-protocol.md wording, but 002-technology.md T5 itself still says it. Options: (a) grow get_backlog with in_progress/blocked/done/cancelled/escalations arrays (token-heavy — rows carry full descriptions), (b) a new state-shaped orientation verb mirroring GET /v0/state (breaks eleven-verbs), (c) filter/scope inputs on get_backlog. Recommendation: run the grill cycle; my lean is (c) or (a-with-slim-rows) since a twelfth verb needs the stronger justification. This escalation is the audit's designed output — the task said gaps become a design revision, not a workaround.

_Unanswered._

### 2026-08-01 00:19 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-ae2sbw/mcp-parity-audit`
- PR: <https://github.com/brandonbews/tuhdoo/pull/7>

Audit complete; PR #7 merged (squash d2acfa0 lineage). Parity checklist — each user-ask → tool call → verdict: (1) capture to inbox title-only → create_task status:"inbox" → VERIFIED (probe + TestMCPInboxCaptureAndPromotion); (2) cancel/"delete" → update_task status:"cancelled" → VERIFIED (probe + new TestMCPCurationUpdates: leaves all backlog arrays, stays get_task-readable, unclaimable); (3) promote inbox→open with description in one call → VERIFIED (probe + TestMCPInboxCaptureAndPromotion); (4) hold/unhold → update_task status held/open → VERIFIED (probe + tests); (5) reprioritize → update_task priority → VERIFIED (new test); (6) retitle/redescribe → VERIFIED, unsent fields untouched (new test); (7) add/remove parent + depends_on edges → full-replacement lists, empty list removes → VERIFIED incl. unknown-edge rejection (new test); (8) answer open escalation → relay_answer → VERIFIED (probe + TestMCPRelayAnswer; attribution to root principal, relayed_by recorded); (9) read single task any status incl. done/cancelled → get_task → VERIFIED; (10) read backlog → get_backlog ready+inbox+held → VERIFIED. GAPS (read-side, vs TUI sections): no MCP path lists in-progress, blocked, done/cancelled tasks, or open escalations — TUI reads these from /v0/state, agents have no equivalent; find-by-title only works for ready/inbox/held; taskJSON carries no holder field. Gap list + design tension (T5's "curation is human work" sentence vs update_task accepting cancelled/priority) recorded in non-blocking escalation 01KYXB0WN9QKZ1PHPQ8WWM8HQG → feeds a T5 grill cycle. Landed: TestMCPCurationUpdates, get_backlog/update_task description fixes, agent-protocol.md corrections. make test lint green.
