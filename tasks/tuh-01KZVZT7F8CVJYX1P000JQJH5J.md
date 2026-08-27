# T6 doc drift: view stamp documented at views/.meta, lives at .views-meta.json

`tuh-01KZVZT7F8CVJYX1P000JQJH5J`

- **Status:** cancelled
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Cancelled 2026-08-27 at the audit-findings triage (Brandon-delegated): folded into the 002 doc-drift sweep task tuh-01KZVZT7F8CVJYX1P009AJJ4D9 as its item 1. Re-verified before folding: 002-technology.md:127 still says views/.meta; internal/views/views.go:51-54 still writes root-level .views-meta.json with a comment recording the divergence.

Original capture: Go-sweep audit finding. 002 T6 says the view stamp lives at views/.meta; internal/views/views.go ~48-51 knowingly writes root-level .views-meta.json because views render at the branch root — the code comment acknowledges the divergence but T6 was never revised in place. One-line design-doc revision with a note, per convention.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→cancelled
