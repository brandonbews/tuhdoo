# Ship the agent protocol with the binary: tuhdoo protocol command

`tuh-01KZANB3J4YYH09F0Z6FSZQ5CD`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `cli` `protocol` `onboarding`
- **Created:** 2026-08-06 04:29 UTC by `brandon/claude-code-1`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Promoted from inbox at the 2026-08-07 launch-epic structuring. NO dependency on the strategy grill — this is the epic's one immediately claimable task.

Context: docs/agent-protocol.md is the prose half of the agent contract (claim discipline, blocking-escalation sequence, confirm-before-merge, descriptions-are-prompts) but it lives only in the tuhdoo repo — a foreign repo has no way to hand it to its agents without manually copying a file that then forks silently from canon. The design docs never decided a distribution mechanism (T5 only calls the doc a first-class deliverable).

The ask: embed the doc in the binary (go:embed) and print it via a protocol subcommand; init output and README point at it (e.g. pipe into the host repo's CLAUDE.md or agent instructions). One canonical text, versioned with the binary the agents actually talk to.

BEFORE building, settle two open calls with Brandon via blocking escalation (one escalation covering both is fine):
1. the exact command name (`tuhdoo protocol` is the sketch, not a decision);
2. whether `tuhdoo init` should offer to write the doc into the host repo, or only print a pointer.

Acceptance: the subcommand prints the embedded doc byte-for-byte from the canonical source (a test proves embed and file cannot drift); init output and README reference it per Brandon's answer; the doc's five-part descriptions-are-prompts section (context / ask / acceptance / pointers / constraints) survives shipping intact; make test lint green; PR per repo conventions.

Constraints: no third copy of the descriptions-are-prompts convention (it exists in create_task's tool description and the protocol doc; two surfaces is already one drift risk). Host-agnosticism (T2) untouched. T5's twelve-verb budget governs the MCP surface only, not CLI subcommands — but if any MCP change tempts you, stop and escalate.

History: captured at the 2026-08-05 release session as the remaining essential for the first external adopter; absorbed the descriptions-template open-question (tuh-01KZA0VT234XJYVZWT8C4X2TMA, cancelled as subsumed) at the 2026-08-06 grill; promoted 2026-08-07.

## History

_No activity yet._
