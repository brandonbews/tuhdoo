// Every route the site has: the landing page plus the docs nav. lastmod is
// deliberately omitted — the build has no honest per-page date (doc files
// change independently of deploys), and a fabricated timestamp is worse than
// none.
import type { APIRoute } from "astro";
import { docsNav, routeFor } from "../lib/nav";
import { ORIGIN } from "../lib/site";

export const GET: APIRoute = () => {
  const urls = [
    `${ORIGIN}/`,
    ...docsNav.map((entry) => ORIGIN + routeFor(entry)),
  ];
  const body = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...urls.map((url) => `<url>\n<loc>${url}</loc>\n</url>`),
    "</urlset>",
    "",
  ].join("\n");

  return new Response(body, {
    headers: { "content-type": "application/xml" },
  });
};
