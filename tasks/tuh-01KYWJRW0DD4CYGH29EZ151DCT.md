# TUI selection highlight and visual hierarchy: full-height gutter bar, adaptive tint, bold titles

`tuh-01KYWJRW0DD4CYGH29EZ151DCT`

- **Status:** done
- **Priority:** none
- **Labels:** `cli` `tui`
- **Depends on:** [`tuh-83bn`](tuh-01KYWJWCK26X34J7TGSNVK83BN.md) (done)
- **Created:** 2026-07-31 17:15 UTC by `brandon`

## Description

Context: Steering feedback (2026-07-31): the `▸` selection marker is tiny and easy to miss, and the list lacks visual hierarchy. Grill cycle (2026-07-31) decided: selection becomes a full-height gutter bar plus a Claude-Code-style adaptive background tint; titles go bold in every section. Depends on the needs-input 3-line-row task (tuh-01KYWJWCK26X34J7TGSNVK83BN) — restyle after it lands so rowChunk and the goldens churn once. This task also absorbs the "stronger visual hierarchy / bold titles" capture (tuh-01KYWJT42G4CXYPMB6VBX0RG1Q, archived).

Empirical findings (Brandon's mosh session, 2026-07-31): OSC 11 query goes unanswered through mosh (confirmed at a bare prompt, mosh-server present). COLORTERM=truecolor is nonetheless set under mosh — so COLORTERM must NOT gate the exact-tint rung; only an answered query can (the exact tint needs the actual bg color). Indexed 256-color gray (48;5;236) renders as the desired subtle highlight through mosh (Claude Code parity — it emits indexed grays via chalk downsampling, theme from config, no query).

The ask:
1. Selection marker: `▸ ` dies; a `▌` gutter bar renders on EVERY line of the selected chunk (same 2-cell mark column; continuation lines get it too).
2. Selection background, applied to every line of the selected chunk, padded to full width so it reads as a bar. Capability ladder, best first:
   a. Terminal answers an OSC 11 bg query → truecolor tint: bg lightened ~8% on dark themes, darkened on light. Gate on the ANSWER, never on COLORTERM alone. Use termenv (v0.16.0, already an indirect dep — promote to direct); query ONCE, before the bubbletea program starts (its stdin read would fight bubbletea's input loop). This is the SSH / local-terminal case.
   b. No answer but TERM reports 256-color → indexed grayscale tint: 48;5;236-ish on dark, 48;5;253-ish on light; dark/light via termenv HasDarkBackground (OSC11 → COLORFGBG → default dark). This is the mosh day-to-day case; visually matches Claude Code.
   c. Neither → bright-black bg (ESC[100m), the themed gray.
   d. NO_COLOR / non-TTY → no bg at all; the `▌` glyph alone marks selection (glyph, not color — it survives).
   - Rung selection + tint computation are pure functions (inputs: query answer/absence, TERM, COLORFGBG, dark/light → SGR string), table-driven tests; the query lives in one small seam.
3. Bold titles in every section. Bold and dim are both SGR intensity and don't stack, so shelf (on-hold/inbox) titles drop dim and render truly bold; the rest of the shelf row stays dim. Accepted consequence: shelves recede less than today.
4. Selected row must stay readable over every rung's bg.

Acceptance:
- Goldens/tests cover: (a) selected multi-line chunk — `▌` + bg on every line, full-width; (b) unselected rows unchanged; (c) titles bold everywhere, shelf metadata still dim; (d) NO_COLOR — bar glyph, zero SGR codes; (e) ladder/tint functions table-driven: answered-query dark→lighter and light→darker; unanswered+256color→indexed gray (COLORTERM=truecolor with no answer must land here, per the mosh finding); unanswered+16color→bright-black.
- The render.go colors comment gets a revision note: truecolor/256-color allowed for the selection tint only, everything else stays 16-color.
- `make test lint` green from the repo root.

Pointers: cmd/tuhdoo/top.go (gridRow, rowChunk, secondLine, listChunks), cmd/tuhdoo/render.go (colors + comment), cmd/tuhdoo/top_golden_test.go, github.com/muesli/termenv.

Constraints: one-shot commands untouched (no selection concept there); boring Go — hand-rolled styles stay, no lipgloss adoption; bubbletea input loop untouched.

## History

### 2026-07-31 21:04 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +cli +tui · depends_on +tuh-83bn

### 2026-07-31 21:17 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-07-31 23:24 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-1dct/selection-highlight`
- PR: <https://github.com/brandonbews/tuhdoo/pull/4>
- Commits: `5daf502`

Merged (squash) to main via PR #4 and deployed (daemon restarted on the new binary). The list's selected chunk now renders as a full-height bar: a ▌ gutter on every line plus an adaptive background tint resolved down the capability ladder in cmd/tuhdoo/selection.go (answered OSC 11 query → truecolor ±8% tint; unanswered + 256-color TERM → indexed gray 236/253; else bright-black; NO_COLOR → gutter glyph only). COLORTERM never unlocks the tint rung per the mosh finding — pinned by table test. termenv promoted to a direct dep; the single terminal query runs once in runTUI before bubbletea starts (no mosh launch stall: termenv pairs OSC 11 with a DSR query that mosh answers immediately). Titles bold in every section; shelf id/badge/meta stay dim. All acceptance bullets covered by goldens + table-driven ladder tests; make test lint green. Note: this run closes over a scripted shim session as the same principal after the daemon redeploy killed the original MCP session mid-hold.
