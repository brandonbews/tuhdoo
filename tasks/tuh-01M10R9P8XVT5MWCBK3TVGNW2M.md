# TUI/CLI: muted text goes theme-derived (ANSI 90); quiet-chrome bars darken on the rung (238)

`tuh-01M10R9P8XVT5MWCBK3TVGNW2M`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 1
- **Labels:** `tui` `go`
- **Created:** 2026-08-27 04:40 UTC by `brandon`

## Description

## Context

Brandon's steering after PR #86 (2026-08-26, structured question answered): the muted-text gray 38;5;245 is a fixed palette index, not theme-derived, and reads a hair too dark on gruvbox dark; the quiet-chrome bars (bgDarkGray = ESC[100m) render slot-8 bright-black as a background — #928374 under gruvbox — too loud for shelf chrome.

## The ask

1. **col.dim becomes ANSI 90 (bright-black foreground) on every rung** — the theme's own muted gray (gruvbox dark: #928374, warm and slightly lighter than 245). The dim ladder entry retires; floor byte-identity for dim is deliberately ended (steering). Accepted caveat, record it in the comment: solarized-style themes repurpose slot 8 near-background and will mute harder.
2. **bgDarkGray gains a rung entry**: on 256color TERMs the task-view section bars and the CANCELLED history bar render fg gray-250 on bg 238 (`\x1b[38;5;250;48;5;238m`) — a genuinely dark bar, legible on light themes via the pinned fg, one visible step off the 236 selection tint so a bar never reads selected. The 16-color floor keeps plain ESC[100m.
3. dimRed (waiting: lead) unchanged: 2;31 floor / 38;5;131 rung. termColors rung overrides become dimRed, orange, bgDarkGray.

## Acceptance

- Golden tests pin: dim = ESC[90m in both color literals and every dim surface pin (TUI + CLI + textinput hint); rung bars 38;5;250;48;5;238; floor bars ESC[100m unchanged; rung noFaint sweeps still pass.
- make test lint green from the repo root.

## Pointers

- cmd/tuhdoo/render.go — colors law comment (dated revision note), termColors
- cmd/tuhdoo/top_golden_test.go — ansiColors/rungColors, legendKey/legendSep, row pins
- cmd/tuhdoo/top_test.go history-entry pins; cmd/tuhdoo/textinput_test.go hint pin

## Constraints

- Rendering only — no stored-byte or event changes. Indexed codes rung-only; 90 is in-palette everywhere. One PR.

## History

_No activity yet._
