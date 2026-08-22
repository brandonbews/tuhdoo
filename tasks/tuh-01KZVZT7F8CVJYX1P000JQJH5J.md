# T6 doc drift: view stamp documented at views/.meta, lives at .views-meta.json

`tuh-01KZVZT7F8CVJYX1P000JQJH5J`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding. 002 T6 says the view stamp lives at views/.meta; internal/views/views.go ~48-51 knowingly writes root-level .views-meta.json because views render at the branch root — the code comment acknowledges the divergence but T6 was never revised in place. One-line design-doc revision with a note, per convention.

## History

_No activity yet._
