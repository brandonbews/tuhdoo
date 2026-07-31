# t-01KYTQTEQYGXVQ0QY7FCTAZ3G6 — Agent protocol: name the dangling-pointer anti-pattern (and interactive-session notes)

- Status: open — ready
- Priority: 1
- Labels: `docs`, `protocol`
- Created: 2026-07-31 00:05 UTC by `4099114+brandonbews`

## Description

Context: during the Cycle-4 build (2026-07-30) the working agent wrote a task description saying 'full plan in the session that created this task' — a pointer to session state that dies with the session. Brandon was steering from his phone with only tuhdoo as the record and could not follow it. Nothing in docs/agent-protocol.md names or warns against this failure mode, and it is the *natural lazy move* for an agent whose context already contains the plan.

The ask: revise docs/agent-protocol.md (in place, dated revision note per convention) to add:
1. A named anti-pattern — e.g. 'dangling pointers': ledger entries (descriptions, notes, run summaries) must be self-contained or point only at durable repo state (committed files, design docs, task IDs). Never reference 'this session', chat context, scratchpads, or uncommitted local paths. Include the real example above as the cautionary tale.
2. A line on interactive sessions: when the human is live in the conversation, escalate/release is legitimately bypassed (ask directly), but the ledger discipline (claim, self-contained notes, honest finish_run) still applies BECAUSE the human may actually be reading the ledger remotely, not the session.
3. Reinforce note cadence: note before risky changes and at stopping points — the ledger is thinnest exactly when a confident session dies.

Acceptance: agent-protocol.md revised with the three points and a dated revision note; no code changes; any protocol text asserted by tests (mcp_cli_test) still passes; make test lint green.

Constraints: keep the doc's voice (terse, imperative); do not renumber or rewrite unrelated sections.

## History

_No activity yet._
