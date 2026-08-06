# Epics after parents removal: is any epic UX worth having?

`tuh-01KZ9Y3THHH5B8GT22T92BPEZ8`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `design` `tui`
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Capture from the 2026-08-05 edge grill. The parents field is removed; an epic is now just a container task that depends_on its children. Open exploration: does any epic-specific UX earn its keep — e.g. reverse-edge display (a child showing which container needs it, a container listing its children), a container marker in the TUI list, anything else. Also: sweep the codebase for leftover parent/epic traces after the removal task lands. May well resolve to "nothing needed" — worth a small grill before building anything.

## History

### 2026-08-06 22:41 UTC — note from `brandon/claude-code-1`

Partial progress on the "sweep for leftover parent/epic traces" item, 2026-08-06 triage session: docs/agent-protocol.md's two stale parents references (the Decomposition section's "parent edges pointing at the task you hold" and update_task's list-field note) were fixed directly in PR #44 — decomposition now reads "create children in one batch, then point the held task at them with depends_on". The eventual sweep still owes the rest of the codebase/docs; internal/event/catalog.go's "stored events may still carry parents (retired)" comment is deliberate read-side legacy handling, not drift — leave it.
