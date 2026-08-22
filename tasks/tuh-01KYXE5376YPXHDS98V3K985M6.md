# Edit title and description from the task view

`tuh-01KYXE5376YPXHDS98V3K985M6`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `tui` `ux`
- **Depends on:** [`tuh-a86y`](tuh-01KYXDWWM8S1GF6N9NE5FGA86Y.md) (done), [`tuh-ypww`](tuh-01KYXE40ES9YSEGW9Z0GXKYPWW.md) (done)
- **Created:** 2026-08-01 01:13 UTC by `brandon`

## Description

## Context

Dogfooding capture from Brandon (2026-08-01). Fixing a typo'd title or fleshing out a description currently means dropping to `tuhdoo update` on the CLI; the task view — where you notice the problem — offers no way to edit.

## The ask

An edit affordance in the task view:

- Edit the title in the shared text-input widget's single-line mode, the description in its multi-line mode (tuh-01KYXE40ES9YSEGW9Z0GXKYPWW — Brandon decided 2026-07-31 that multi-line lives in the widget itself; do not build a separate editor here). Opening $EDITOR is NOT the ask — this stays in the TUI.
- Writes go through the same plumbing as `tuhdoo update` (task.updated), acting as the TUI's principal.
- Pick an unclaimed key (e.g. `e`) and put it in the footer.

## Acceptance

- From the task view you can change the title and the description; the view re-renders with the new content and the change survives a restart (it's in the ledger).
- Esc cancels without writing; an unchanged submit writes nothing.
- Behavioral + golden tests. make test lint green.

## Pointers

- cmd/tuhdoo/top.go, cmd/tuhdoo/render.go; the update path used by cmd/tuhdoo/write_cmds.go.

## Constraints

- Boring Go; TUI-only.
- Depends on tuh-01KYXE40ES9YSEGW9Z0GXKYPWW (the shared widget, including its multi-line mode) and tuh-01KYXDWWM8S1GF6N9NE5FGA86Y (the task-view rework — land after it so the screen isn't rebuilt twice).

## History

### 2026-08-01 01:21 UTC — edit by `brandon`

retitled · description edited · status inbox→open · priority none→1 · labels +cli +tui +ux · depends_on +tuh-a86y +tuh-ypww

### 2026-08-01 02:09 UTC — edit by `brandon`

description edited

### 2026-08-01 03:40 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-85m6/task-view-editing`
- PR: <https://github.com/brandonbews/tuhdoo/pull/12>
- Commits: `fdb4a02`

Landed on main as fdb4a02 (PR #12, squash, CI green). Armed task view gains e (edit title, single-line widget) and E (edit description, multi-line mode first real consumer); footer says e/E edit. Both prefill current value, cursor at end; esc cancels without writing; unchanged submit writes nothing; empty title rejected in place; emptying description is a legitimate clear. Writes ride the same PATCH /v0/tasks plumbing as tuhdoo update (same task.updated event, TUI principal); real-daemon test greps the data branch for the event. Watch mode gains nothing (pinned). One deliberate footer trade flagged for Brandon in the PR: the redundant enter-answer legend item moved out of the detail footer (the NEEDS INPUT section bar owns it, dashboard convention) to keep the legend inside 80 columns. Deliberately not done: no e/E on the dashboard list; labels/status/edges not editable; no pre-edit re-fetch (same ~2s poll staleness as all steering writes).
