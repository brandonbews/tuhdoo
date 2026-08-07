# User-facing docs: the human story of steering tuhdoo, platform-agnostic

`tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY`

- **Status:** done
- **Priority:** 0
- **Labels:** `docs` `product`
- **Depends on:** [`tuh-qf4g`](tuh-01KZEPBEE8HFDQVK96AQNCQF4G.md) (done), [`tuh-kk4j`](tuh-01KZF1DNJ3T77A01NJXF1VKK4J.md) (done)
- **Created:** 2026-08-05 21:48 UTC by `brandon`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Promoted from inbox at the 2026-08-07 launch-epic structuring. Depends on the strategy grill (tuh-01KZEPBEE8HFDQVK96AQNCQF4G) and on the docs-swap task (tuh-01KZF1DNJ3T77A01NJXF1VKK4J), which creates the content root this task writes into.

REPRESENTATION — settled at the strategy grill, agenda item 5, 2026-08-07 (write in exactly this form):
- Render targets: tuhdoo.com, GitHub file browsing, raw terminal text. Not the binary (the protocol command stays the only embedded doc).
- Format: GFM + YAML frontmatter restricted to title + description — no MDX, no framework-dialect keys. Navigation/ordering lives in the site's own config, never in content files.
- Location: root docs/ is the published content root; the directory is the publish boundary (post-swap; working docs live in internal-docs/).
- Links: relative paths to real .md files and real asset paths; GitHub is the semantic baseline — if a link works clicking through GitHub it must work everywhere; the site rewrites .md links to routes at build time (standard remark/rehype); the site adapts to the content, never the reverse.

WRITING BAR (Brandon, 2026-08-07): tight, straight-to-the-point, comprehensive. Follow docs best practices and carry the informative tone and vibe of great library/framework docs (TanStack, Next.js). No weird lingo or vocabulary that doesn't make sense outside this repo — define product terms on first use (escalation, lease, data branch, claim); internal session jargon (e.g. "grill", "B12", cycle numbers) never appears. Written so Brandon can maintain and iterate on it easily.

Context: this task owns the human-facing narrative of using tuhdoo — the prose that explains, for a person (not an agent), the intention→DAG flow the mechanism already supports: capture (inbox, title-only is fine) → triage/grill → promote (prompt-quality description: context / ask / acceptance / pointers / constraints) → decompose (atomic batch create_task with tmp: refs; a container depends_on its children) → steer (priority, edges, held, escalation answers). The mechanism all exists; the prose for humans exists nowhere. The 2026-08-06/07 triage-and-structuring sessions are the living example of the flow — the launch epic itself was built exactly this way and can be the worked example.

The ask: write the user-facing docs in the representation above and to the writing bar — the steering flow, plus what adopting tuhdoo looks like for a team (init, the TUI, escalation answering, onboarding a teammate). Post-swap, docs/joining.md and docs/uninstall.md are siblings in the same content root and direct source material.

Acceptance: the capture→triage→promote→decompose→steer flow documented for humans with a worked example; files live under docs/ in the settled representation, renderable on the site and readable standalone on GitHub and in a terminal; prose meets the writing bar (plain vocabulary, terms defined, no repo-internal jargon); make test lint untouched/green.

Constraints — two audiences, two documents, no forking: the agent-facing conventions ship in agent-protocol.md (post-swap a public sibling in docs/, delivered to foreign repos by tuh-01KZANB3J4YYH09F0Z6FSZQ5CD). These docs are for humans; link the protocol doc, never copy its content into a divergent version.

History: captured by Brandon 2026-08-05; absorbed the plan-materialization open-question at the 2026-08-06 triage grill (tuh-01KZA0VT234XJYVZWT8Q19P9QM, cancelled as subsumed); promoted into the launch epic 2026-08-07; representation decision recorded from the strategy grill 2026-08-07; writing bar added same day.

## History

### 2026-08-07 22:23 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-m6qy/user-docs`
- PR: <https://github.com/brandonbews/tuhdoo/pull/55>
- Merged as: `997fd770baec05f10c1b52d70dd4daa4a01ec42c`

Landed on main as 997fd77 (PR 55, squash). Two pages under docs/: steering.md (terms defined up front; five-status model with derived ready/in-progress/blocked; the capture-triage-promote-decompose-steer lifecycle; worked example telling the tuhdoo.com launch exactly as this ledger records it, in plain product vocabulary) and adopting.md (one init, teammates join, agents connect over MCP, pick a workflow recipe, steer from TUI/CLI; ends with Leaving pointing at uninstall.md). Frontmatter title+description only; relative links; agent conventions linked never copied. docs/README.md index and site/src/lib/nav.ts updated in matching order; /docs/steering and /docs/adopting verified rendering statically with all links and anchors resolving in the built HTML. Drive-by catch: tuhdoo top and tuhdoo watch are retired stubs, so the CLI table documents the real surface verified against cmd/tuhdoo source (including --desc - stdin and --depends-on full-replacement). make test lint and site build green. No Go change, no daemon restart needed.
