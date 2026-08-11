import type { MetadataRoute } from "next";

import { docsNav, routeFor } from "@/lib/nav";

const ORIGIN = "https://www.tuhdoo.com";

// Every route the site has: the landing page plus the docs nav. lastModified
// is deliberately omitted — the build has no honest per-page date (doc files
// change independently of deploys), and a fabricated timestamp is worse than
// none.
export default function sitemap(): MetadataRoute.Sitemap {
  return [
    { url: `${ORIGIN}/` },
    ...docsNav.map((entry) => ({ url: `${ORIGIN}${routeFor(entry)}` })),
  ];
}
