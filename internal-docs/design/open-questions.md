# Open questions — migrated into tuhdoo

**This file is a tombstone.** On 2026-08-05 (pre-v0.2.0 release grill) its
remaining questions were migrated onto the tuhdoo ledger, where open
questions live from now on: `tuhdoo backlog` (inbox for untriaged, held for
parked). This file will not accrue new entries. Design docs (`001`, `002`)
are the agent-facing **decision record** only; work-tracking and open
questions belong to the ledger.

## Settled (answers live in the design docs)

- **Milestone semantics** — 2026-08-03, milestone grill → `001` D5
  (done-ness is declared, never computed; `milestone` is a label, not a
  mechanism).
- **Edge semantics: parents, loops, cancelled deps** — 2026-08-05, edge
  grill → `001` D5 (parents removed, epics `depends_on` their children;
  loop detection never prevention; cancelled deps keep blocking; dangling
  deps count as met). PRs #37/#39.
- **The name check** — tuhdoo is the shipping name, affirmed 2026-08-05;
  tuhdoo.com is owned.
- **The agent protocol doc** — delivered: `docs/agent-protocol.md`,
  field-tested since 2026-07-30.
- **`get_backlog` scope (MCP read-self-sufficiency)** — grilled 2026-08-01
  → `002` T5.

## Migrated to the ledger (2026-08-05)

| Question | Task |
|---|---|
| Task-descriptions template | `tuh-01KZA0VT234XJYVZWT8C4X2TMA` |
| `claim_next` discovery/filters | `tuh-01KZA0VT234XJYVZWT8EXV78J5` |
| Salvage flow (superseded/interrupted) | `tuh-01KZA0VT234XJYVZWT8GVB4HQT` |
| Escalation delivery, TUI closed | `tuh-01KZA0VT234XJYVZWT8K2D75W9` |
| Run↔code linkage robustness | `tuh-01KZA0VT234XJYVZWT8KT0BSDH` |
| Plan-materialization flow | `tuh-01KZA0VT234XJYVZWT8Q19P9QM` |
| Init UX remainder (joining flow, branch protection) | `tuh-01KZA0VT234XJYVZWT8S09PK06` |
| Repo-hosting edge cases | `tuh-01KZA0VT234XJYVZWT8VGFG3NX` |
| Monorepo grain | `tuh-01KZA0VT234XJYVZWT8YFV8XE2` |
| Multi-repo story | `tuh-01KZA0VT234XJYVZWT91KSJGJR` |
| Uninstall story | `tuh-01KZA0VT234XJYVZWT93P2EK1S` |
| Compaction triggers in practice | `tuh-01KZA0VT234XJYVZWT95JM25KW` |
| Working-set retirement | `tuh-01KZA0VT234XJYVZWT980V7K2Y` |
| v2+ parked set (pointer task; cancelled 2026-08-07 — deferrals live beside their decisions in `001`/`002`) | `tuh-01KZA0VT234XJYVZWT98B7NXEH` |

An epic-UX exploration capture also exists:
`tuh-01KZ9Y3THHH5B8GT22T92BPEZ8`.

## History

The question lists themselves — the candidate-cycle groupings, the
edge-semantics analysis that fed the 2026-08-05 grill, the milestone
resolution written out in full — are preserved in this file's git history
(`git log -p -- docs/design/open-questions.md`).
