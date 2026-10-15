// Site-wide constants: the origin every absolute URL is built on, and the
// landing page's title and description (also the fallbacks for the 404).

// The apex is the canonical host: www redirects to it (a redirect rule on
// the domain's Cloudflare zone), so every absolute URL we emit (canonical,
// og:url, og:image, sitemap, llms.txt) must live on the apex or crawlers
// see a redirect hop.
export const ORIGIN = "https://tuhdoo.com";

export const SITE_NAME = "tuhdoo";

export const SITE_TITLE = "tuhdoo: a coordination fabric for agent fleets";

export const SITE_DESCRIPTION =
  "tuhdoo gives your coding agents a shared backlog, work queue, and activity ledger, stored on a git branch inside the repo it plans. It syncs over the git remote you already have — no server, no vendor, no accounts.";

export const REPO_URL = "https://github.com/brandonbews/tuhdoo";

// The link-preview card (public/opengraph-image.png), 1200×630.
export const OG_IMAGE = {
  url: `${ORIGIN}/opengraph-image.png`,
  type: "image/png",
  width: 1200,
  height: 630,
  alt: "The tuhdoo wordmark, its double o drawn as two glassy dividing cells, above the tagline: a coordination fabric for agent fleets — on a git branch inside the repo it plans.",
};
