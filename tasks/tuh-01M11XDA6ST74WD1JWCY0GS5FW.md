# TUI pane grew to 3.4GB RSS overnight; a longer-lived pane sits at 21MB

`tuh-01M11XDA6ST74WD1JWCY0GS5FW`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `go` `tui`
- **Created:** 2026-08-27 15:28 UTC by `brandon/claude-code-1`

## Description

Observed 2026-08-27 07:47 PDT via ps: TUI pid 12683 (bare `tuhdoo`, started 2026-08-26 21:59 PDT) at 3,589,584 KB RSS (~3.4GB) and 7h40m CPU; TUI pid 8460 (started Fri) at ~21MB RSS. The MUCH older pane is fine, so growth is not simply session age — suspect an interaction with last night's conditions (the overnight drain's event volume, the post-#93 saturated daemon slowing responses, or a binary difference: 12683 predates the 02:17 redeploy).

No repro or profile yet — this is a facts-only capture. Investigation shape: pprof/heap on a TUI reproducing the growth, or restart a pane under today's daemon and watch RSS; check whether per-tick hydration retains snapshots/history (the 2s poll x ~104 hydrations x overnight hours is a lot of allocations if anything is retained).

Triage 2026-09-10 (live-replica grill with Brandon): the likely mechanism is the TUI's unguarded poll. Every 2s tick issues a full hydrate (one GET per task, ~104 then, 162 now) with no in-flight guard; when the daemon answers slower than a tick (the post-#93 per-read refresh under load, exactly last night's conditions), fetches start faster than they finish, and every in-flight fetch holds its partial snapshot map, so goroutines and RSS grow without bound. The 7h40m CPU on the 3.4GB pane is that loop; the 21MB pane is consistent with a period when responses fit inside the tick. Step 3 of the plan (tuh-01M26H99QVANVJCXR9FM2Z8Q7X) deletes the tick and the N+1 hydrate and keeps exactly one request in flight, and its acceptance includes a pane-RSS check. DO NOT investigate before tuh-01M26H99QVANVJCXR9FM2Z8Q7X lands. After it: restart a pane under the new daemon, leave it 24h, check ps RSS; flat means cancel this task, growth means reopen with a heap profile.

## History

### 2026-09-10 20:59 UTC — edit by `brandon`

description edited
