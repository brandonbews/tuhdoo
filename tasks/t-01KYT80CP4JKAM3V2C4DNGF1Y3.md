# TUI readability: short display IDs, width-aware wrapping, list scrolling

`t-01KYT80CP4JKAM3V2C4DNGF1Y3`

- **Status:** done
- **Priority:** 2
- **Labels:** `cli` `tui`
- **Created:** 2026-07-30 19:28 UTC by `4099114+brandonbews`

## Description

Dogfooding feedback from Brandon (2026-07-30) on the Cycle-4 TUI:
1. Full ULIDs obscure content. Display task IDs in TUI rows as `t-` + lowercase last-4 of the ULID (batch-created ULIDs share prefixes, so the tail is the distinguishing part; settled via AskUserQuestion with previews). Display-only — full IDs everywhere under the hood, in the detail screen, and in all one-shot commands.
2. Long lines flow off screen with no horizontal scroll (deps lists, waiting: reasons, descriptions unreadable). Wrap all TUI output ANSI-aware to terminal width using charmbracelet/x/ansi (already in the module graph via bubbletea). Detail body wraps BEFORE line-windowing so scroll math counts real screen lines. List body gets vertical windowing that keeps the cursor row visible; blank line between rows for scanability (Brandon ok'd multi-line rows).

Acceptance: no rendered line wider than the terminal; cursor row always on screen; detail scroll walks wrapped lines; short IDs shown in list/waiting/edge-marker text while detail shows full IDs; one-shot commands byte-identical; make test lint green.

## History

### 2026-07-30 19:33 UTC — run by `4099114+brandonbews` — done

- Branch: `main`
- Commits: `e1c9ff8`, `8c2d61a`

Both pieces landed and pushed, make test lint green. (1) TUI rows display IDs as t- prefix + lowercase last-4 of the ULID (tail, not prefix — same-batch ULIDs share their timestamp prefix); applied to rows, waiting: reasons, edge markers, and escalation task refs; detail screen and all one-shots keep full IDs (blockedReasonDisp keeps backlog byte-identical). (2) All TUI output wraps ANSI-aware to terminal width via charmbracelet/x/ansi (promoted indirect→direct, same version); detail body wraps before line-windowing; list body gets a cursor-following vertical window (cursor pinned to bottom edge when scrolled); blank line between rows. Tests assert no line exceeds width, frame fits height, cursor always visible, tail reachable by scroll. Not verified in a real TTY — Brandon should eyeball bare `tuhdoo` in tmux.
