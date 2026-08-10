# Logo finals: export and deliver the brand assets (human-led)

`tuh-01KZPPRW1EKBTVKNR76H6Z7SQC`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `web` `design` `launch`
- **Created:** 2026-08-10 20:45 UTC by `brandon`

## Description

HUMAN-LED: this is Brandon's design session. Agents must not work this task — if you are an agent holding this claim, release it with reason "human-led design asset".

Context: settled at the 2026-08-10 launch-polish grill. Brandon has a logo in progress: a clean Sora "tuhd" wordmark followed by a glassy, frosted, noisy, rich-gradient 3D mark of two lime/emerald cells dividing — the "oo", fleet multiplication. The site identity task (tuh-01KZF973FY9JKJV5F38SM7BAN7) derives the whole visual identity from these assets and is blocked on this task.

The deliverable, preferred form (settled 2026-08-10): ONE SVG of the full lockup ("tuhd" + mark), committed at site/public/brand/tuhdoo-lockup.svg (create the directory) via a small PR. Everything downstream derives from it: the identity task crops the mark out of the lockup for favicon / apple-touch-icon / og work; gradient hexes are read from the SVG source; embedded-raster effects inside the SVG are acceptable. Add the Sora weight/tracking used for the wordmark to the PR description (the file loses it if text was outlined). Rough-with-margins is fine — cropping and cleanup are the identity task's job.

Fallback form if SVG export isn't practical: (1) the mark alone, transparent, 1024–2048px+; (2) the full lockup, transparent, ~2000px+ wide; (3) gradient stop hexes; (4) Sora spec.

CONDITIONAL follow-up: a simplified flat variant of the mark (same silhouette, no glass/noise) ONLY if the real mark turns to mush at 16–32px — the identity task tests the real mark at favicon sizes first and escalates back here if it fails.

Explicitly NOT Brandon's job: favicon size sets, apple-touch-icon, og-image layout, manifest icons — all derived downstream.

Acceptance: the asset(s) committed on main and this task marked done (Brandon: TUI, or `bin/tuhdoo update tuh-01KZPPRW1EKBTVKNR76H6Z7SQC --status done`); the identity task unblocked. The drain then serves the identity task automatically once the toolchain task has also landed.

## History

_No activity yet._
