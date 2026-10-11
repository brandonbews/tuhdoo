// What goes in each page's <head>: title, description, canonical URL, and
// the link-preview (og:/twitter:) tags. Base.astro renders a PageHead; the
// three builders below are the only kinds of page the site has.

import { readFrontmatter } from "./docs";
import { type NavEntry, routeFor } from "./nav";
import { ORIGIN, SITE_DESCRIPTION, SITE_NAME, SITE_TITLE } from "./site";

export type PageHead = {
  title: string; // the full <title>
  description: string;
  canonical: string | null; // null: no canonical link (the 404)
  noindex: boolean;
  og: {
    type: "website" | "article";
    url: string;
    title: string;
    description: string;
  };
  // The landing page's card names the image with its size, type, and alt
  // text; doc pages name the image only.
  imageDetails: boolean;
};

// The landing page. Its canonical URL is the bare origin, no trailing slash.
export function landingHead(): PageHead {
  return {
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
    canonical: ORIGIN,
    noindex: false,
    og: {
      type: "website",
      url: ORIGIN,
      title: SITE_TITLE,
      description: SITE_DESCRIPTION,
    },
    imageDetails: true,
  };
}

// A doc page: title and description come from the doc's frontmatter.
export function docHead(entry: NavEntry): PageHead {
  const { title, description } = readFrontmatter(entry.file);
  const url = ORIGIN + routeFor(entry);
  return {
    title: `${title} · ${SITE_NAME}`,
    description,
    canonical: url,
    noindex: false,
    og: { type: "article", url, title, description },
    imageDetails: false,
  };
}

// The 404 page: noindex, and no canonical link — a 404 must not declare
// itself a copy of the landing page. The link-preview tags are the
// landing page's.
export function notFoundHead(): PageHead {
  return {
    ...landingHead(),
    title: `Page not found · ${SITE_NAME}`,
    canonical: null,
    noindex: true,
  };
}
