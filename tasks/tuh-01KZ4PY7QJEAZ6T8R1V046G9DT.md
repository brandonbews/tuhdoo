# Task-view history omits task.updated — field edits leave no visible trace

`tuh-01KZ4PY7QJEAZ6T8R1V046G9DT`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `tui` `ledger`
- **Created:** 2026-08-03 21:02 UTC by `brandon`

## Description

Noticed 2026-08-03 while rewriting the v1 milestone description: the task view's History section renders runs, notes, and escalations, but not task.updated events. After editing a description the history still said "no activity yet", so a field edit — title, description, priority, status, labels, edges — is invisible on the task it happened to.

Worth grilling before building: which updates are worth a history line vs. noise (a priority bump probably yes, a labels tweak maybe not), and whether the entry shows the diff, the field names, or just "brandon edited the description".

## History

_No activity yet._
