# Grill: run Matt Pocock's improve-codebase-architecture skill here? (parked until Brandon can read the Go)

`tuh-01KZSB8EAZ2H3618SZVWH66K4Y`

- **Status:** on hold — deliberately paused
- **Priority:** none
- **Labels:** `design` `go`
- **Created:** 2026-08-11 21:22 UTC by `brandon`

## Description

Held at Brandon's direction 2026-08-12 — needs its own grill before anything runs. NOT NOW, for reasons decided that day:

1. The skill (github.com/mattpocock/skills, engineering/improve-codebase-architecture) is findings-first and human-steered: scan for "deepening opportunities" (Ousterhout vocabulary — deep/shallow modules, seams, adapters, deletion test) → visual HTML report of candidates → the human picks one → a grilling loop before any refactor. Its value depends on the human holding ground truth in the grill, and Brandon has not yet read the Go in this codebase — an architecture grill today would be one-sided rubber-stamping, worse than not running it because the decisions would look human-ratified.
2. No pressure signal: ~14k non-test LOC, three weeks old, no felt architectural friction from humans or agents. The skill's own YAGNI framing says deepen where change keeps happening. The Go sweep (tuh-01KZ9YBF1N06FQ37XV65940SSG) files captures for structural smells its readers hit — real friction will surface as evidence.

UNPARK WHEN: Brandon has worked through the Go reading companion (tuh-01KZ9Z6647C3TBCYGGTXQJYE8V) and can read ops.go and disagree — or when sweep captures / dogfood friction show real architectural pressure, whichever comes first.

Notes for the eventual grill (verified 2026-08-12):
- The skill must be added to the skill library first (Brandon's ask: bring it into his skill box), likely with companions /codebase-design and /domain-modeling; a /grilling skill already exists here.
- The report step runs `open` on an HTML file — this machine is SSH-only, no display; adapt to SendUserFile or an artifact.
- It expects CONTEXT.md + docs/adr/; tuhdoo has neither. internal-docs 001/002 (D1-D11, T1-T8) must be presented as the ADR corpus or the skill will re-litigate settled decisions (boring Go, deterministic core, host-agnosticism).
- Any accepted refactor is design-shaped work: it goes through the repo's own grill convention and revision notes, and must respect the tests-as-spec posture established on the Go sweep.

## History

### 2026-08-12 21:21 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→held · labels +design +go
