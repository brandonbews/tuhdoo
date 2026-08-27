# TUI: quiet-chrome bars ride the background ladder (theme tint when OSC-answered; theme fg + neutral bg under mosh)

`tuh-01M10S0DK46ZJNNXHK22M2A3NG`

- **Status:** done
- **Priority:** 1
- **Labels:** `tui` `go`
- **Created:** 2026-08-27 04:52 UTC by `brandon`

## Description

## Context

Brandon's steering after PR #87 (2026-08-27): the task-view/CANCELLED bars' pinned fg 250 on bg 238 read as out-of-theme colors under gruvbox dark. True theme derivation is only possible when the terminal answers OSC 11 (mosh swallows it — 2026-07-31 finding); under mosh the closest is theme-default foreground on a dark/light-appropriate neutral background.

## The ask

1. **bgDarkGray becomes TUI-resolved down the existing background ladder** (chromeBG beside selectionBG in selection.go, consuming the same one queryTermBG answer in runTUI): answered OSC 11 → truecolor tint of the actual theme background, ~15% toward the opposite extreme (a step stronger than selection's 8% so bar and selection never collide); unanswered on a 256color TERM → `\x1b[48;5;238m` when dark, `\x1b[48;5;251m` when light — no pinned foreground, the theme's default fg rides the bar; floor → plain `\x1b[100m`.
2. **newColors/termColors keep bgDarkGray at plain `\x1b[100m` on every rung** — the rung override moves out of termColors into runTUI (bgDarkGray is TUI-only: task-view bars + CANCELLED history bar; the CLI renders bold headers, no bars). Struct comment gains the selBG-style "re-resolved by runTUI" note; law comment gets a dated revision.
3. Factor the tint percentage so selectionBG (8%) and chromeBG (~15%) share the tint helper. COLORTERM never consulted.

## Acceptance

- chromeBG unit table (selection_test-style): answered dark/light pin exact tint bytes; 256color dark → 48;5;238, light → 48;5;251; floor "" TERMs → \x1b[100m.
- rungColors/ansiColors literals: bgDarkGray back to \x1b[100m; TermColorsLadder still exact-matches.
- Rung task-view golden sets m.col.bgDarkGray explicitly (selBG-test pattern) and pins `\x1b[48;5;238m` bars with no pinned fg; floor goldens unchanged (\x1b[100m).
- make test lint green from the repo root.

## Pointers

- cmd/tuhdoo/selection.go — selectionBG/tintSGR/queryTermBG; cmd/tuhdoo/top.go runTUI ~2092; render.go comments
- cmd/tuhdoo/top_golden_test.go RungTaskViewBars; selection_test.go for the ladder-test shape

## Constraints

- Rendering only. Indexed/truecolor codes stay rung-and-above; 16-color floor untouched. One PR.

## History

### 2026-08-27 04:56 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-a3ng/chrome-bar-ladder`
- PR: <https://github.com/brandonbews/tuhdoo/pull/88>
- Merged as: `bb28b7f746b062d0fbe3363ad03d54e04189261d`

All acceptance criteria met; squash-merged to main as bb28b7f (PR #88), make test lint green. Quiet-chrome bars now ride the background ladder: chromeBG (beside selectionBG) resolves bgDarkGray in runTUI from the one OSC 11 query — answered terminals get a truecolor tint of the actual theme background (~15 percent toward the opposite, a step stronger than selection s 8 so they never collide); unanswered 256color TERMs (mosh) get 48;5;238 dark or 48;5;251 light with NO pinned foreground, so the theme s own default fg rides the bar; the floor keeps plain 100, which newColors now returns on every rung (the termColors rung override moved out — bgDarkGray is TUI-only). Ladder test added; rung golden pins 48;5;238 with theme fg; floor goldens unchanged. make test lint green. Daemon redeploy follows this finish_run.
