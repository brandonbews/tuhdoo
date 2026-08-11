# Agent protocol slim-down: de-bloat the protocol doc and its supplementary material (launch gate)

`tuh-01KZSBC7K0GNYNYTTAM6DW7Q0M`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `docs` `protocol` `launch`
- **Created:** 2026-08-11 21:24 UTC by `brandon`

## Description

Context: Brandon's verdict 2026-08-11: the agent protocol "seems very bloated and over specified and all of the supplementary information around it (especially in the docs) seems confusing and over the top." The 2026-08-11 announcement grill made this a launch gate — it is published content on the adopter's evaluation path, and the Show HN moment fires only after this and the copy pass land.

The ask: compress `docs/agent-protocol.md` and the protocol-explaining material around it (supplementary sections in other docs, and the `tuhdoo protocol` command output, which ships the protocol with the binary — see tuh-01KZANB3J4YYH09F0Z6FSZQ5CD) to the minimum an agent actually needs to run the loop correctly. This is compression, not redesign: cut repetition, over-specification, and hedging; keep every load-bearing semantic (claim → work → finish honestly, escalation flow, salvage breadcrumbs, notes doctrine).

Acceptance:
- `docs/agent-protocol.md` substantially shorter with no semantic loss; supplementary protocol mentions in other docs deduplicated to pointers rather than restatements.
- `tuhdoo protocol` output matches the slimmed doc (its tests updated if output is asserted).
- Before the big rewrite lands: escalate with the proposed structure and a before/after sample so Brandon can calibrate the bar — the prior copy pass (#62) was judged insufficient the same day it landed; do not repeat that by guessing.
- Brandon's PR review is the final bar. `make test lint` green; one PR.

Constraints: T5's twelve verbs are untouched — this task changes prose, never surface. MCP tool descriptions in code were deliberately aligned with the notes doctrine (t-01KYVD31CNTR1EVCDHPJGSQAGH); if the slim-down changes doctrine wording, keep them aligned, but do not redesign them. `docs/` publishing rules apply (GFM, frontmatter title + description, relative links).

Sequencing: this lands BEFORE the copy-tightening pass (tuh-01KZSBDXFZCRNEDY7DMD4XGP75 depends on this task) — structural cuts first, tone polish second, so the copy pass never polishes text about to be deleted.

## History

_No activity yet._
