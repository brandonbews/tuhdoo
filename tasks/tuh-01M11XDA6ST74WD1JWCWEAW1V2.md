# get_task read-time refresh under TUI per-task polling saturates the daemon; writes starve for minutes

`tuh-01M11XDA6ST74WD1JWCWEAW1V2`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `go` `daemon` `tui`
- **Created:** 2026-08-27 15:28 UTC by `brandon/claude-code-1`

## Description

Observed live 2026-08-27 ~07:45-08:00 PDT (adopter-grade evidence; capture, don't build from this without triage). Daemon pid 72885 (v0.4.0-19-g541b56d, started 02:17 PDT at the post-drain deploy) at ~89% CPU with 253 CPU-minutes accrued by 07:47. sample(1) of the process shows the hot paths: net/http -> handleGetTask -> opGetTask -> refreshLocked -> core.Replay (JSON unmarshal hot), plus heavy fork/wait4 (git subprocess loads). daemon.log rates a refresh at this ledger size (745 events, 123 leases) at load ~20-30ms + replay ~14-15ms.

Mechanism: PR #93 (task tuh-01KZVZT7F8CVJYX1NZZS3GK62E) made opGetTask refresh at read time — D6-correct in isolation. But the TUI hydrates every task individually on its poll tick (~104 get_task round-trips per 2s per pane, per the 2026-08-06 pressure snapshot on the retirement capture), and two TUI panes were open. Arithmetic at today's size: ~104 x ~35-45ms x 2 panes vs a 2s tick — the daemon is saturated by read-path replays, indefinitely.

Consequence, observed: writes starve. update_task calls took 4-5 minutes wall each (daemon.log: events 745/746/747 landing minutes after their sends); a caller that treats a multi-minute write as a hang and retries writes duplicate task.updated events (three identical updates landed for tuh-01KZVZT7F8CVJYX1NZZNQZ5X37 exactly this way today — harmless additively, but the pattern is the system teaching clients to dupe).

Honest accounting: the k62e task description (written at the 2026-08-27 audit triage) asserted per-poll refresh was fine off the 2.5us/event replay bench; at 745 events the real refresh cost is load+replay ~35-45ms — the bench measured replay alone, not the git-subprocess load, and not the per-task fan-out multiplier. The acceptance said stop-and-capture if the numbers disagreed; production surfaced the disagreement first. This capture is that stop.

Candidate directions for triage (each has design surface — none decided): TUI-side batch hydration (one state fetch per tick instead of N get_tasks); refreshLocked memo (no-op when head unchanged and now is within the same evaluation instant — replay purity untouched, the daemon memoizes the trigger); lease-verdict-only fast path for reads; working-set retirement (tuh-01KZA0VT234XJYVZWT980V7K2Y) bounds N and was predicted to be this pressure's owner. Cross-link: tuh-01M0XBC1P2NYTPZQ4BAFSWHGY1 owns the write-side view-regen half.

## History

_No activity yet._
