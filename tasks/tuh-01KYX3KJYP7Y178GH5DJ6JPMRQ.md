# tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ — Workflow recipes: recommended dev-flow patterns in init/docs (product feature, not this repo's conventions)

- Status: on hold — deliberately paused
- Priority: 0
- Labels: `docs`, `product`
- Created: 2026-07-31 22:09 UTC by `brandon/claude-code-1`

## Description

Gated: unpark for a grill cycle once v1 confidence exists — the v0 dogfood week has held (t-01KYRMFV10W1N28TCN5NWAGSW5) AND this repo's trunk-based PR flow (tuh-01KYX1D49M9M0EB69HNVBZT906) has been exercised on real tasks. Do NOT scope or write docs before that grill; this note is the triage decision (2026-07-31), not a plan.

Captured from the 2026-07-31 grill cycle: tuhdoo's init/docs could offer a small collection of recommended outer-workflow recipes (e.g. trunk-based ticket→PR→squash) for host repos adopting tuhdoo. tuhdoo itself stays workflow-agnostic (agent-protocol step 3: ordinary git on ordinary branches); recipes are suggestions, never baked into the protocol. This repo's own PR flow is the first recipe candidate — its dogfood results are evidence for the grill. Likely eventual home: the marketing/docs site task (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2), which shares this gate — the two unpark together, or the recipes ship in CLI init text; the grill decides.

## History

### 2026-07-31 23:59 UTC — note from `brandon/claude-code-1`

Position statement for the future grill (2026-07-31 session with Brandon) — inputs, not decisions:

1. The boundary to defend is protocol-vs-workflow, not dumb-vs-smart. The primitives' contract (claim → work → escalate → finish loop, lease semantics, honest finish_run) ships IN the tool — it already does, via the MCP server instructions, and should stay there. Everything outer (branching shape, PR conventions, inbox-grooming rituals) ships in docs as recipes. "Dumb but powerful" is the slogan; "protocol in the tool, workflow in the docs" is the enforceable version.

2. Init should emit links to recipe docs after success. Precedent already exists: init today ends with CI guidance and a "Next: status · backlog · tui" line — non-binding guidance text after success is an established move.

3. Recipes carry copy-pasteable, per-host artifacts: AGENTS.md/CLAUDE.md blocks, vendorable skill files. Explicitly NOT a marketplace — no server, no accounts, no curation platform; that would reintroduce the exact stack the founding no-server/no-vendor principles avoid, through the docs side door. If demand for discovery/sharing materializes later, that's evidence for a future grill, not something to pre-build.

4. Candidate pitch line for the docs/marketing site (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2): "all it's really doing is letting me see and organize stuff while slightly slowing agents down" — the slowdown is the feature: work is forced through typed, visible transitions a human can steer.

5. Stronger delivery mechanism captured as its own held task sharing this gate: tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ (init flavor picker — multiple-choice workflow setup that drops recipe files). Links-in-init is the floor, the picker is the ceiling; the grill decides how far up the axis to go.
