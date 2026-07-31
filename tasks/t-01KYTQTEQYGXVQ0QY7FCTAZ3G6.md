# t-01KYTQTEQYGXVQ0QY7FCTAZ3G6 — Agent protocol: dangling-pointer anti-pattern; notes are garnish, transitions are the record

- Status: done
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

### 2026-07-31 05:30 UTC — run by `4099114+brandonbews/claude-code-2` — done

- Branch: `main`
- Commits: `ee80980`

docs/agent-protocol.md revised in place with a dated (2026-07-30) revision note in the Status line covering all three points. (1) New section "Anti-pattern: dangling pointers": monitored ledger entries (task descriptions, run summaries, escalation Q&A, release reasons) must be self-contained or point only at durable repo state; references to "this session", chat context, scratchpads, or uncommitted paths are banned; the real Cycle-4 "full plan in the session that created this task" incident is the cautionary tale. (2) New "Human live in your session?" paragraph in the escalation section: ask directly instead of escalate/release, but claim discipline and a self-contained finish_run still apply — the ledger, not the transcript, is what the human monitors. (3) Loop step 4 reframed: continuity is carried by the typed transition events (claim/finish_run/release_claim/escalate/synthesized interrupted run) whose payloads are already mandatory; add_note is optional garnish, zero-note runs are normal. Matching small revision note taken in 002 T5's "ledger is agent memory" bullet, deferring doctrine to the protocol doc. No code changed. Follow-up worth a task: five code strings still carry the old "letters to the next agent/incarnation" framing — internal/daemon/mcp.go lines ~30, ~344, ~439, ~505-507 and internal/event/catalog.go ~134 — they are the live tool descriptions agents actually read, so they now lag the doc.
