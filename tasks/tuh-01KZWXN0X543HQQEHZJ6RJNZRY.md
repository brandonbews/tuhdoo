# TUI chrome pass: repo name in header, quiet sync glyph, badge-only-when-special, feedback-only status line

`tuh-01KZWXN0X543HQQEHZJ6RJNZRY`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `tui` `go`
- **Created:** 2026-08-13 06:41 UTC by `brandon`

## Description

Context: grew from a "sense of place" capture into a header/chrome pass at the 2026-08-21 grill (Brandon). All four changes are DECIDED — build them, don't re-litigate; each gets a dated revision note in 002 T7. Current state: header renders bold " tuhdoo · <syncline>" with a right badge ("watch mode " dim, or "acting as <actor> " always when armed) — cmd/tuhdoo/top.go:1449 listHead; syncLine (cmd/tuhdoo/render.go:58) prints `syncing with "origin" · last fetch <absolute UTC stamp>`; a status line below the header shows action feedback including success confirmations ("set tuh-x to p2").

The four changes:
1. **Sense of place:** bold repo-root directory basename replaces the bold product name. No product name in the TUI — you know what you launched; you need to know which ledger this is. (In this repo it happens to still read "tuhdoo".)
2. **Quiet sync:** healthy (last fetch within ~2 sync cycles) renders a static dim ⇅ glyph only — no timestamp, no ticking relative time, no spinner (sync is a ~60s background cycle; a spinner would deceive). Stale sync adds relative age, dim/yellow: "⇅ 8m". `local-only` stays a word (remoteless is a normal mode — a bare missing glyph would look broken). Sync errors stay loud text. Note: the "relative times rot" discipline (render.go:75) governs written views; a live screen redrawing every 2s can't rot — but we avoid ticking anyway because motionless chrome was the explicit ask.
3. **Badge only when special:** dim "watch" when disarmed; "as <principal>" only when --as overrode the derived identity; an armed pane acting as your derived identity shows NOTHING. Absence is the normal state (vim convention). This revises the 2026-08-03 "armed must be glanceable" note — glanceability now means the special states are marked.
4. **Status line = feedback only:** errors, validation ("title cannot be empty"), and in-flight markers ("updating…") only; success confirmations dropped — the screen updating is the confirmation (accepted: "captured to inbox" may land below the fold; trusted anyway). Render it visually distinct from content — a box/strip treatment, implementer's choice, pinned by golden test. Action-feedback sites: top.go ~336-338, ~850-947 (actionMsg descs).

Acceptance: golden tests pin all four behaviors (header healthy/stale/local-only/error, badge in armed/watch/--as states, status line error + in-flight + no-success); 002 T7 revised in place with dated notes per the repo convention; `make test lint` green. One PR.

Constraints: chrome-hierarchy rule holds (frame never competes with content); any color choices slot into the existing capability ladder (selection.go header, render.go:15-27) — no COLORTERM trust.

## History

### 2026-08-21 23:09 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority 0→1 · labels +tui +go
