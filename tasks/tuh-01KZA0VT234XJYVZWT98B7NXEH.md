# v2+ parked features (pointer): intake bridge, signing, kanban, view templates, webhook fetch, supervisor, read-only sharing

`tuh-01KZA0VT234XJYVZWT98B7NXEH`

- **Status:** cancelled
- **Priority:** 0
- **Labels:** `design`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Migrated from open-questions.md (parked thread), 2026-08-05 sweep. One pointer task for the whole v2+ parked set — each is already gated with unpark conditions in docs/plan/roadmap.md v2+; unpark this (or split it) when any gate opens. The set: public intake bridge (D4; must stay optional per T2), event signing (sig reserved, D7), kanban/browser UI (D8/T7), user-customizable view templates (T6 view-format-version treatment), webhook-driven fetch (T8), per-machine supervisor / cross-project dashboard (T4), read-only sharing with non-committers (conflicts with D4 trust boundary; real demand only).

Added 2026-08-06 (multi-repo grill): cross-repo task edges — declared out of scope as mechanism (mechanical edges across ledgers conflict with D4's repo-write trust boundary and the repo-scoped founding definition; the migrated multi-repo open-question was cancelled on this reasoning). Parked here in case an adopter ever demands it. The served-today answer: a task in one repo's ledger may describe work in another repo as prose — durable string pointers, exactly like PR links under T2; the read-side want is the supervisor/dashboard entry above.

## History

### 2026-08-06 21:39 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-07 18:55 UTC — edit by `brandon/claude-code-1`

status held→cancelled
