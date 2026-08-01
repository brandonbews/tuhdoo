# tuh-01KYXE5376YPXHDS98V3K985M6 — Edit title and description from the task view

- Status: open — blocked on dependencies
- Priority: 1
- Labels: `cli`, `tui`, `ux`
- Depends on: [tuh-01KYXDWWM8S1GF6N9NE5FGA86Y](tuh-01KYXDWWM8S1GF6N9NE5FGA86Y.md) (open), [tuh-01KYXE40ES9YSEGW9Z0GXKYPWW](tuh-01KYXE40ES9YSEGW9Z0GXKYPWW.md) (open)
- Created: 2026-08-01 01:13 UTC by `brandon`

## Description

## Context

Dogfooding capture from Brandon (2026-08-01). Fixing a typo'd title or fleshing out a description currently means dropping to `tuhdoo update` on the CLI; the task view — where you notice the problem — offers no way to edit.

## The ask

An edit affordance in the task view:

- Edit the title in a single-line input, the description in a multi-line entry (a simple line-based editor is fine for v0; opening $EDITOR is NOT the ask — this stays in the TUI).
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
- Depends on tuh-01KYXE40ES9YSEGW9Z0GXKYPWW (the shared text-input widget — build editing on it, don't hand-roll a third input) and tuh-01KYXDWWM8S1GF6N9NE5FGA86Y (the task-view rework — land after it so the screen isn't rebuilt twice).

## History

_No activity yet._
