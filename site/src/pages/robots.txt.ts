// robots.txt: everything may be crawled; the sitemap lists every page.
import type { APIRoute } from "astro";
import { ORIGIN } from "../lib/site";

export const GET: APIRoute = () =>
  new Response(`User-Agent: *\nAllow: /\n\nSitemap: ${ORIGIN}/sitemap.xml\n`, {
    headers: { "content-type": "text/plain; charset=utf-8" },
  });
