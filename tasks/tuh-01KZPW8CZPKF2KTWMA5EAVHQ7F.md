# Grill: is the data branch's CI/build-tooling friction a deal breaker for early adopters?

`tuh-01KZPW8CZPKF2KTWMA5EAVHQ7F`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `design` `adoption`
- **Created:** 2026-08-10 22:21 UTC by `brandon/claude-code-2`

## Description

NEEDS A GRILL — do not build from this capture. It is a question about whether a cost we already accepted is too high for people who are not us, and the answer could be "nothing changes."

Origin (2026-08-10, Brandon): Vercel started building failing preview deployments off this repo's `tuhdoo` data branch. Fixing it here is a two-minute dashboard setting. The question the incident actually raised is the one worth grilling: **does this level of friction cost us early adopters?** Someone evaluating tuhdoo gets a repo full of failing builds and vendor emails within their first hour of running agents, and the fix requires them to go configure a third-party dashboard they may not own. That is a bad first impression at exactly the moment we can least afford one, and the sibling docs task (adoption docs and init missing hosted preview builders) only *warns* about it — it does not remove it.

Why the friction is structural, not incidental: the ledger lives at `refs/heads/tuhdoo`, and the daemon pushes on every event (claim, note, finish). Anything wired to "a branch moved" reacts, repeatedly, all day. Vercel, Netlify, and Cloudflare Pages deploy every branch by default. Unfiltered GitHub Actions `on: push` workflows run. Rulesets targeting all branches reject the pushes outright. Every one of these needs per-adopter, per-tool configuration, and each is a place a new user can silently get it wrong.

The candidate structural answer, offered as a starting point and not a recommendation: store the ledger outside `refs/heads/` — e.g. `refs/tuhdoo/data` rather than a branch. GitHub is believed to fire `push` webhooks only for `refs/heads/*` and `refs/tags/*`, so Vercel, Netlify, and Actions would never learn the ref moved. It would also disappear from branch dropdowns and PR base pickers. **That webhook claim is unverified and is the crux — verify it against a live host before the grill leans on it.**

Known costs of that direction, to weigh honestly:
- Nobody can browse the generated `backlog.md` / `escalations.md` on github.com anymore. That is a real feature loss: part of the "no server, no vendor" pitch is that the ledger is readable wherever the repo already lives.
- `git clone` and default `git fetch` do not fetch custom ref namespaces. The daemon controls its own git invocations so this is a refspec change there, but `tuhdoo init` on a fresh clone has to fetch explicitly, and any human doing git archaeology by hand needs to know the incantation.
- Host support for arbitrary refs is less universally guaranteed than `refs/heads`. It is still pure git protocol, so arguably *more* T2-aligned, not less — but "works on every host" is the weaker claim.
- Migration: this repo is an existing adopter with a live ledger. A cutover exists and has to be designed.
- `store.DefaultRef` is a compile-time constant; the shape of that change is known from the monorepo-grain capture (tuh-01KZA0VT234XJYVZWT8YFV8XE2).

Also in scope for the grill, since they may resolve the question without the structural change: is per-tool documentation plus loud `init` output actually sufficient? Does push batching/debouncing reduce the noise enough to matter, or is it just a smaller version of the same wrong shape? Is there a middle option nobody has proposed yet?

Framework note: CLAUDE.md and 001 describe tuhdoo as living in a git orphan *branch* — this is founding shape, so any change here is a design-doc revision with a revision note, in the established D5/D6/D8 pattern, not a quiet edit.

Cross-links: the sibling docs/init capture (hosted preview builders) is the near-term mitigation and should land regardless of how this grill goes; tuh-01KZA0VT234XJYVZWT8YFV8XE2 (monorepo grain) touches the same `store.DefaultRef` constant and the same "what is the right git shape for the ledger" territory — worth reading together, possibly worth grilling together.

## History

### 2026-08-11 22:00 UTC — note from `brandon/claude-code-1`

Grilled with Brandon 2026-08-11; verdict: keep refs/heads/tuhdoo — the friction is a documented limitation, not an architecture problem. Cancelled per that decision.

The crux was verified live before the grill, both directions:
- The custom-ref escape hatch WORKS: pushed a probe commit to refs/tuhdoo-probe/webhook-test on this repo (github.com/brandonbews/tuhdoo). Push accepted, rulesets silent, ZERO Actions runs triggered (test.yml fires on every branch push, so silence is signal), ref absent from the branches API. GitHub docs corroborate: push events fire for refs/heads/* and refs/tags/* only. Probe ref deleted after.
- Its cost is REAL: github.com returns 404 for /tree/refs/tuhdoo-probe/... on this public repo — custom refs are not browsable in the web UI, so the move would kill "browse backlog.md where the repo lives" (D3 soul, part of the launch pitch).

Brandon's reasoning: the branch being a real branch, with all the quirks of a real branch, is a fact of the solution — call it out plainly in getting-started docs and a short init-output note, and move on. One data point (us), a two-minute fix, and browsability is load-bearing pre-launch.

TRIPWIRE (why cancelled, not held): if real adopters report this friction post-launch, capture it FRESH with their evidence — do not revive this speculative capture (affinity-hints precedent, 2026-08-06). The verified facts above carry over; the decision does not auto-carry.

Recorded: D4 consequence 3 re-affirmation note in 001-core-design.md (PR referencing this task). Sibling docs task tuh-01KZPW8CZPKF2KTWMA5B8QYVN0 promoted to ready with the doc-shape decisions folded in (principle + verified Vercel example; docs AND init line; no unverified vendor steps).
