# t-01KYTQTEQYGXVQ0QY7FCTAZ3G6 — Agent protocol: dangling-pointer anti-pattern; notes are garnish, transitions are the record

- Status: open — in progress, claimed by `4099114+brandonbews/claude-code-2`
- Priority: 1
- Labels: `docs`, `protocol`
- Created: 2026-07-31 00:05 UTC by `4099114+brandonbews`

## Description

Context: during the Cycle-4 build (2026-07-30) the working agent wrote a task description saying 'full plan in the session that created this task' — a pointer to session state that dies with the session. The ledger is the project-native record: whoever launches tuhdoo to see what's going on — the steering human, a successor agent, a teammate browsing the data branch — has only the ledger, never the session that wrote the entry. Nothing in docs/agent-protocol.md names this failure mode, and it is the *natural lazy move* for an agent whose context already contains the plan.

The ask: revise docs/agent-protocol.md (in place, dated revision note per convention) to add/adjust:
1. A named anti-pattern — e.g. 'dangling pointers': the ledger entries humans actually monitor (task descriptions, run summaries, escalation questions/answers, release reasons) must be self-contained or point only at durable repo state (committed files, design docs, task IDs). Never reference 'this session', chat context, scratchpads, or uncommitted local paths. Include the real example above as the cautionary tale.
2. A line on interactive sessions: when the human is live in the conversation, escalate/release is legitimately bypassed (ask directly), but claim and an honest self-contained finish_run still apply — the ledger, not the session transcript, is what the human monitors.
3. Reframe notes (steering decision, 2026-07-30): continuity across agents is carried by the TYPED transition events, all of which already require their payloads — claim (who/when + full hydration on pick-up), finish_run (outcome + summary), release_claim (required reason), escalate (question + context), and the daemon's synthesized interrupted run on lease expiry. add_note is optional garnish for mid-flight context an agent chooses to pass on — not a protocol obligation, not 'letters to the next incarnation' doctrine. Tone down the current framing that presents free-form notes as the primary continuity mechanism (T5's 'ledger is agent memory' passage may deserve a matching revision note in 002 — small, in place).

Acceptance: agent-protocol.md revised with the three points and a dated revision note (plus the small 002 T5 note if taken); no code changes; any protocol text asserted by tests (mcp_cli_test) still passes; make test lint green.

Constraints: keep the doc's voice (terse, imperative); do not renumber or rewrite unrelated sections.

## History

_No activity yet._
