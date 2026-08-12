# Task-descriptions-are-prompts: a template or convention worth shipping?

`tuh-01KZA0VT234XJYVZWT8C4X2TMA`

- **Status:** cancelled
- **Priority:** 0
- **Labels:** `docs` `protocol`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Migrated from docs/design/open-questions.md (agent-loop thread) during the 2026-08-05 doc-sync sweep. The bar exists (agent-protocol "descriptions are prompts" section); open is whether a reusable template/checklist should ship in init output or docs for host repos.

Cancelled as subsumed at the 2026-08-06 grill (Brandon): nothing separate is left to ship. The convention already reaches agents at write-time via create_task's tool description ("descriptions are prompts: include acceptance criteria, constraints, and file pointers") and reaches readers via the protocol doc's five-part section with enforcement teeth. Distribution of that doc to host repos is exactly the protocol-shipping capture's job (tuh-01KZANB3J4YYH09F0Z6FSZQ5CD, which now notes it absorbs this question). Init output rejected as a third surface: init is deliberately lean (#42), nobody writes tasks at init-time, and a third copy of the convention is a third thing to drift.

## History

### 2026-08-06 21:20 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→cancelled
