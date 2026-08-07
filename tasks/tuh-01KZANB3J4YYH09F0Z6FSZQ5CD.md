# Ship the agent protocol with the binary: tuhdoo protocol command

`tuh-01KZANB3J4YYH09F0Z6FSZQ5CD`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `protocol` `onboarding`
- **Created:** 2026-08-06 04:29 UTC by `brandon/claude-code-1`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Promoted from inbox at the 2026-08-07 launch-epic structuring. NO dependency on the strategy grill — this is the epic's one immediately claimable task.

Context: docs/agent-protocol.md (path unchanged by the 2026-08-07 docs swap — root docs/ is now the published content root, and agent-protocol.md is one of its three public docs) is the prose half of the agent contract (claim discipline, blocking-escalation sequence, confirm-before-merge, descriptions-are-prompts) but it lives only in the tuhdoo repo — a foreign repo has no way to hand it to its agents without manually copying a file that then forks silently from canon. The design docs never decided a distribution mechanism (T5 only calls the doc a first-class deliverable).

The ask: embed the doc in the binary (go:embed) and print it via a protocol subcommand; init output and README point at it (e.g. pipe into the host repo's CLAUDE.md or agent instructions). One canonical text, versioned with the binary the agents actually talk to.

BEFORE building, settle two open calls with Brandon via blocking escalation (one escalation covering both is fine):
1. the exact command name (`tuhdoo protocol` is the sketch, not a decision);
2. whether `tuhdoo init` should offer to write the doc into the host repo, or only print a pointer.

Acceptance: the subcommand prints the embedded doc byte-for-byte from the canonical source (a test proves embed and file cannot drift); init output and README reference it per Brandon's answer; the doc's five-part descriptions-are-prompts section (context / ask / acceptance / pointers / constraints) survives shipping intact; make test lint green; PR per repo conventions.

Constraints: no third copy of the descriptions-are-prompts convention (it exists in create_task's tool description and the protocol doc; two surfaces is already one drift risk). Host-agnosticism (T2) untouched. T5's twelve-verb budget governs the MCP surface only, not CLI subcommands — but if any MCP change tempts you, stop and escalate.

History: captured at the 2026-08-05 release session as the remaining essential for the first external adopter; absorbed the descriptions-template open-question (tuh-01KZA0VT234XJYVZWT8C4X2TMA, cancelled as subsumed) at the 2026-08-06 grill; promoted 2026-08-07.

## History

### 2026-08-07 21:55 UTC — escalation from `brandon/claude-code-1` (blocking)

> Two decisions before building the ship-the-protocol command: (1) What is the subcommand's exact name? (2) Should `tuhdoo init` offer to WRITE the protocol doc into the host repo, or only PRINT a pointer telling the operator how to get it?

The task (its own text mandates this escalation) embeds docs/agent-protocol.md in the binary via go:embed and prints it via a subcommand, so foreign repos get one canonical text versioned with the binary instead of a hand-copied file that forks from canon.

(1) Command name. The existing CLI surface is all bare single words: help, version, daemon, init, status, backlog, task, escalations, create, update, answer, watch, top, mcp. `tuhdoo protocol` (the task's sketch) fits that style exactly. Alternatives considered: `tuhdoo agent-protocol` (more precise, but the only hyphenated verb on the surface) or `tuhdoo docs protocol` (implies a doc-family subtree that doesn't exist yet). Recommendation: `tuhdoo protocol`, printing the embedded doc to stdout, nothing else.

(2) Init behavior. Option A — pointer only: init output shows the exact command (e.g. "agents: pipe `tuhdoo protocol` into your agent instructions, or reference it from CLAUDE.md"); no file is written. Option B — init offers to write the doc into the host repo (a flag or prompt, e.g. --write-protocol <path>). Recommendation: Option A. The task's own context names the failure mode this feature exists to kill — a copied file that forks silently from canon — and Option B re-creates that fork one step later: the written file goes stale the moment the binary upgrades. With a pointer, a host that truly wants a file can pipe stdout itself and owns the refresh. Option A is also less init-surface to maintain and keeps init non-interactive.

Answer can be as short as: "protocol; pointer only" (or name another spelling / pick B with a path convention).

**Answer** (`brandon`): your recs both work for me

### 2026-08-07 21:56 UTC — note from `brandon/claude-code-1`

Resume state: no code written, no branch — work stopped at the task's mandated pre-build gate. Escalation 01KZF3KJ9CF9BKM9XNA4YK3VSD carries both open decisions (command name; init write-vs-pointer) with recommendations; its answer unblocks the whole build. Groundwork for the claimant: existing subcommand dispatch is the switch in cmd/tuhdoo/main.go (~line 27); all current verbs are bare single words; docs/agent-protocol.md path is post-docs-swap-correct (PR #52) and is the canonical source to go:embed.

### 2026-08-07 21:56 UTC — run by `brandon/claude-code-1` — blocked

Blocked on escalation 01KZF3KJ9CF9BKM9XNA4YK3VSD (command name; init write-vs-pointer) — the task's own pre-build gate. No build attempted; recommendations are in the escalation.

### 2026-08-07 23:11 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-q5cd/protocol-command`
- PR: <https://github.com/brandonbews/tuhdoo/pull/56>
- Merged as: `e5abb6315fb0964896b369385e2adc9adb6f0164`

Landed on main as e5abb63 (PR 56, squash), per the answered escalation: command name tuhdoo protocol, init pointer-only. docs/agent-protocol.md ships in the binary via a tiny root package (embed.go, package tuhdoo) since go:embed cannot climb out of a package dir; tuhdoo protocol prints it byte-for-byte to stdout — pure print, no repo, no daemon, extra args rejected. Tests pin both guarantees: TestProtocolEmbedMatchesDoc (embed cannot drift from the file) and TestProtocolCommandPrintsDocAnywhere (real binary in a non-repo temp dir: byte-equal output, exit 0, no files created, descriptions-are-prompts heading and its five parts present, help lists it). init success output gained the pointer line (asserted in TestInitRemoteless); README and docs/adopting.md each gained a sentence; the convention is never restated anywhere. No MCP changes. make test lint green. Binary changed — deploy restart happening right after this finish.
