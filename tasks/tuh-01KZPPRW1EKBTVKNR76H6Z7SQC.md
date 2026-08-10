# Logo finals: export and deliver the brand assets (human-led)

`tuh-01KZPPRW1EKBTVKNR76H6Z7SQC`

- **Status:** done
- **Priority:** 1
- **Labels:** `web` `design` `launch`
- **Created:** 2026-08-10 20:45 UTC by `brandon`

## Description

HUMAN-LED — COMPLETED 2026-08-10. Brandon supplied the SVG in-session; landed via PR #61 (main commit f6e1d7a).

DELIVERY RECORD (everything the identity task tuh-01KZF973FY9JKJV5F38SM7BAN7 needs):
- Asset: site/public/brand/tuhdoo-lockup-dark.svg — the full lockup: "tuhd" wordmark + the glassy dividing-cells mark ("oo").
- DARK-MODE VARIANT ONLY, designed against surface #060806 (treat that as the dark-theme background seed). Wordmark fill #f4f7f2.
- Font: Sora, weight 800 (text is outlined to paths in the file — the weight spec lives here, not in the SVG).
- Gradient stops (mark, radial): #f2fff7 → #8bffbb → #2fe884 → #0c9152. Accents: #35e87c (halo), #b8ffd4 (rim stroke), #0c8a4d (satellite flecks). Inner shade base #02160b. Derive the accent/neutral tokens from these source values, not from eyedropping.
- The mark occupies roughly x 245–360 of the 407×220 viewBox; extract it from the lockup for favicon / apple-touch-icon / manifest / og renditions.
- Uses SVG filters (feTurbulence, feDisplacementMap, feGaussianBlur, soft-light blend) — rasterize for contexts that need PNG (og-image, touch icons); browsers render the SVG fine for inline/nav use.

Derivation duties now on the identity task: (1) LIGHT-THEME VARIANT — no light version was delivered; wordmark fill swap is trivial, but the glassy mark must be visually checked on a light surface — if it degrades, escalate to Brandon rather than shipping a bad derivation. (2) TINY-SIZE TEST — render the mark at 16/32px; if the glass/noise turns to mush, escalate back to Brandon for the simplified flat variant (same silhouette, no effects) rather than shipping an unreadable favicon.

Original ask and fallback forms: see task history (2026-08-10 grill).

## History

_No activity yet._
