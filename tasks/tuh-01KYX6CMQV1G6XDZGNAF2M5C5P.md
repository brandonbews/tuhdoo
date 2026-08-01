# Slash command /drain-backlog: the reusable drain-the-backlog prompt

`tuh-01KYX6CMQV1G6XDZGNAF2M5C5P`

- **Status:** done
- **Priority:** 0
- **Labels:** `dx` `docs`
- **Created:** 2026-07-31 22:58 UTC by `brandon/claude-code-1`

## Description

Context: Brandon has re-derived the "work the backlog until claim_next returns claimed:false" orchestration prompt more than once; the substance lives in docs/agent-protocol.md but the paste-ready session prompt is captured nowhere. The ask: add .claude/commands/drain-backlog.md — a Claude Code slash command carrying the drain loop updated for the trunk-based PR flow: claim_next → work per acceptance criteria → land per the CLAUDE.md loop (branch, PR, auto-squash-merge, finish_run after merge) → repeat until claimed:false; deploy-after-landing daemon restart between binary-changing tasks (finish_run first — the restart kills the MCP session); escalate rather than guess; stop cleanly on empty pool. Acceptance: the command file exists and reads as a complete standalone prompt; no changes outside .claude/commands/; lands via the PR flow. Constraints: docs/agent-protocol.md untouched — the command points at it, never restates protocol semantics that could drift.

## History

### 2026-07-31 22:59 UTC — run by `brandon/claude-code-1` — done

.claude/commands/drain-backlog.md landed via PR https://github.com/brandonbews/tuhdoo/pull/2 (squash commit d48ebed on main). The command is the paste-ready drain loop: claim_next → work per acceptance criteria → land per the CLAUDE.md PR loop → finish_run only after merge → repeat until claimed:false, with the deploy-after-landing daemon-restart step (finish_run before restart — it kills the MCP session), escalate-rather-than-guess, and a context-budget stop condition. Points at CLAUDE.md and docs/agent-protocol.md for semantics rather than restating them. No changes outside .claude/commands/. No binary change — no daemon restart needed.
