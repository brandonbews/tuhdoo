# tuh-01KYWWH4DZH4TR7ASVGTDBT14P — Do we still need the one-shot backlog/escalations commands?

- Status: inbox — untriaged capture
- Priority: 0
- Labels: `cli`, `design`
- Created: 2026-07-31 20:05 UTC by `brandon/claude-code-1`

## Description

Captured from the 2026-07-31 grill cycle: Brandon forgot these commands existed and is inclined to remove them. Counterpoint raised there: MCP get_backlog is deliberately claimable-only, so the one-shot `tuhdoo backlog` is currently the only complete read surface outside the TUI (piping, watch, pasting state into a conversation, laggy SSH); they also share snapshot/render code with the TUI, so deletion saves little. Decide deliberately — removal would touch docs and the agent protocol references.

## History

_No activity yet._
