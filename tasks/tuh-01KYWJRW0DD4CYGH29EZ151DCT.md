# tuh-01KYWJRW0DD4CYGH29EZ151DCT — TUI selection highlight and visual hierarchy: full-height gutter bar, adaptive tint, bold titles

- Status: open — blocked on dependencies
- Priority: 0
- Labels: `cli`, `tui`
- Depends on: [tuh-01KYWJWCK26X34J7TGSNVK83BN](tuh-01KYWJWCK26X34J7TGSNVK83BN.md) (open)
- Created: 2026-07-31 17:15 UTC by `brandon`

## Description

Context: Steering feedback (2026-07-31): the `▸` selection marker is tiny and easy to miss, and the list lacks visual hierarchy. Grill cycle (2026-07-31) decided: selection becomes a full-height gutter bar plus a Claude-Code-style adaptive background tint; titles go bold in every section. Depends on the needs-input 3-line-row task (tuh-01KYWJWCK26X34J7TGSNVK83BN) — restyle after it lands so rowChunk and the goldens churn once. This task also absorbs the "stronger visual hierarchy / bold titles" capture (tuh-01KYWJT42G4CXYPMB6VBX0RG1Q, archived).

The ask:
1. Selection marker: `▸ ` dies; a `▌` gutter bar renders on EVERY line of the selected chunk (same 2-cell mark column; continuation lines get it too).
2. Selection background, applied to every line of the selected chunk, padded to full width so it reads as a bar:
   - Terminal answers an OSC 11 bg query → truecolor tint: bg lightened ~8% on dark themes, darkened on light. Use termenv (v0.16.0, already an indirect dep — promote to direct); query ONCE, before the bubbletea program starts (its stdin read would fight bubbletea's input loop).
   - No answer (mosh swallows OSC queries) → bright-black bg (ESC[100m), the themed gray.
   - NO_COLOR / non-TTY → no bg at all; the `▌` glyph alone marks selection (glyph, not color — it survives).
   - Tint computation is a pure function (bg RGB + dark/light → SGR string), table-driven tests; the query lives in one small seam.
3. Bold titles in every section. Bold and dim are both SGR intensity and don't stack, so shelf (on-hold/inbox) titles drop dim and render truly bold; the rest of the shelf row stays dim. Accepted consequence: shelves recede less than today.
4. Selected row must stay readable over tint and fallback bg alike.

Acceptance:
- Goldens/tests cover: (a) selected multi-line chunk — `▌` + bg on every line, full-width; (b) unselected rows unchanged; (c) titles bold everywhere, shelf metadata still dim; (d) NO_COLOR — bar glyph, zero SGR codes; (e) tint function table-driven incl. dark→lighter, light→darker; fallback = bright-black when bg unknown.
- The render.go colors comment gets a revision note: truecolor is allowed for the selection tint only, everything else stays 16-color.
- `make test lint` green from the repo root.

Pointers: cmd/tuhdoo/top.go (gridRow, rowChunk, secondLine, listChunks), cmd/tuhdoo/render.go (colors + comment), cmd/tuhdoo/top_golden_test.go, github.com/muesli/termenv.

Constraints: one-shot commands untouched (no selection concept there); boring Go — hand-rolled styles stay, no lipgloss adoption; bubbletea input loop untouched.

## History

_No activity yet._
