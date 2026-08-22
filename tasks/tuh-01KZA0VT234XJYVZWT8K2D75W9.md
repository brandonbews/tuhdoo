# Escalation delivery when the TUI is closed

`tuh-01KZA0VT234XJYVZWT8K2D75W9`

- **Status:** cancelled
- **Priority:** none
- **Labels:** `design` `tui`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

2026-08-07: moved to the Ideas icebox in docs/plan/roadmap.md at Brandon's direction — he wants this off the ledger entirely for now (might be cool someday, zero attention today). Cancelled, not deleted; the settled 2026-08-06 grill record below stays authoritative if the idea is ever picked up, and any future capture should start from it rather than re-grilling from scratch.

Gated: unpark when notifications comes up as a topic on its own — e.g. an escalation sat unanswered long enough to actually hurt, or an external adopter asks how they'd hear about escalations. Decided held at the 2026-08-06 grill (Brandon): a blocking escalation parks one task, not the fleet — the agent releases its claim and claim_next moves on — so with frequent check-ins the pull surfaces (TUI, `tuhdoo escalations`, get_backlog scope) carry v1. The v1 milestone's clause 1 only needs the TUI-open path.

Settled at that grill, so the unpark grill starts here, not from scratch:
- Shape: a generic on-escalation exec hook — the daemon runs a user-configured command (git-config key, precedent: tuhdoo.principal); tuhdoo ships no delivery vendor. Serverless (D2), host-agnostic (T2), boring Go.
- Baked-in OS notification is rejected, not deferred: the founding dogfood machine is headless (SSH/mosh) — it would notify nobody on day one.
- Trigger: every escalation raise, payload carries the blocking flag + task/escalation IDs; the hook script filters. No urgency policy in the binary (labels-grill spirit: mechanics agnostic, meaning lives with the user). Answer events excluded — the human is the answerer.

Left open, first question of the unpark grill: fire-on-sync semantics — locally-raised only vs any newly-observed open escalation post-startup-replay (the two-machine steering case wants the latter; needs the replay-silence carve-out and in-memory dedup by escalation ID).

Constraints: host-agnosticism (T2) and no-server (D2) untouched; hook failure must never block a ledger write.

## History

### 2026-08-06 19:31 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→held

### 2026-08-07 19:28 UTC — edit by `brandon/claude-code-1`

description edited · status held→cancelled
