# TUI pane grew to 3.4GB RSS overnight; a longer-lived pane sits at 21MB

`tuh-01M11XDA6ST74WD1JWCY0GS5FW`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `go` `tui`
- **Created:** 2026-08-27 15:28 UTC by `brandon/claude-code-1`

## Description

Observed 2026-08-27 07:47 PDT via ps: TUI pid 12683 (bare `tuhdoo`, started 2026-08-26 21:59 PDT) at 3,589,584 KB RSS (~3.4GB) and 7h40m CPU; TUI pid 8460 (started Fri) at ~21MB RSS. The MUCH older pane is fine, so growth is not simply session age — suspect an interaction with last night's conditions (the overnight drain's event volume, the post-#93 saturated daemon slowing responses, or a binary difference: 12683 predates the 02:17 redeploy).

No repro or profile yet — this is a facts-only capture. Investigation shape: pprof/heap on a TUI reproducing the growth, or restart a pane under today's daemon and watch RSS; check whether per-tick hydration retains snapshots/history (the 2s poll x ~104 hydrations x overnight hours is a lot of allocations if anything is retained).

## History

_No activity yet._
