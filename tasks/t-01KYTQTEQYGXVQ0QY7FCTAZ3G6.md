# t-01KYTQTEQYGXVQ0QY7FCTAZ3G6 — Agent protocol: name the dangling-pointer anti-pattern

- Status: open — ready
- Priority: 1
- Labels: `docs`, `protocol`
- Created: 2026-07-31 00:05 UTC by `4099114+brandonbews`

## Description

Context: during the Cycle-4 build (2026-07-30) the working agent wrote a task description saying 'full plan in the session that created this task' — a pointer to session state that dies with the session. The ledger is the project-native record: whoever launches tuhdoo to see what's going on — the steering human, a successor agent, a teammate browsing the data branch — has only the ledger, never the session that wrote the entry. Nothing in docs/agent-protocol.md names this failure mode, and it is the *natural lazy move* for an agent whose context already contains the plan.

The ask: revise docs/agent-protocol.md (in place, dated revision note per convention) to add:
1. A named anti-pattern — e.g. 'dangling pointers': the ledger entries humans actually monitor (task descriptions, run summaries, escalation questions/answers) must be self-contained or point only at durable repo state (committed files, design docs, task IDs). Never reference 'this session', chat context, scratchpads, or uncommitted local paths. Include the real example above as the cautionary tale.
2. A line on interactive sessions: when the human is live in the conversation, escalate/release is legitimately bypassed (ask directly), but claim and an honest self-contained finish_run still apply — the ledger, not the session transcript, is what the human monitors.

Deliberately NOT in scope: preaching note cadence. Notes are an optional tool for agents that want them, not a discipline the protocol enforces — tuhdoo tracks work and blockers; it does not prescribe how anyone works (steering decision, 2026-07-30).

Acceptance: agent-protocol.md revised with the two points and a dated revision note; no code changes; any protocol text asserted by tests (mcp_cli_test) still passes; make test lint green.

Constraints: keep the doc's voice (terse, imperative); do not renumber or rewrite unrelated sections.

## History

_No activity yet._
