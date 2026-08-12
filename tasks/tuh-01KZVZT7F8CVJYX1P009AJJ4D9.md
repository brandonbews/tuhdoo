# T7 doc drift: one-shot WAITING vocabulary and TUI sections omit later-designed tokens; parent-edge remnants

`tuh-01KZVZT7F8CVJYX1P009AJJ4D9`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit findings, 002 T7. (1) T7's one-shot output contract defines WAITING as dep:<id> / esc:<id> only; the shipped, golden-pinned vocabulary (oneshot_golden_test.go) also includes 'cyclic,' and the dep:<id>:cancelled suffix — designed by 001 D5's 2026-08-05 edge grill ('loud marks on every surface') but never folded into T7's enumeration. (2) T7 still lists edge markers '· in <parent>' and --parents among fragment-resolving entry points; the 2026-08-05 edge grill deleted parents (same doc, line ~94) and no code path renders or accepts one. (3) T7 says shelf 'rows stay dim'; code and goldens pin bold titles in every section per a 2026-07-31 visual-hierarchy decision recorded only in a code comment (top.go gridRow); the dead topSection.dim field that fossilized this was removed by the sweep. Sweep T7 in place with revision notes.

## History

_No activity yet._
