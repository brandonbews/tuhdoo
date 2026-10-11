// llms-full.txt: every published doc page, raw markdown, in reading order —
// one fetch for an agent that wants the whole manual. Each page is prefixed
// with its canonical URL so excerpts stay attributable. Written to
// dist/llms-full.txt at build time from the same sources the rendered pages
// use.
import type { APIRoute } from "astro";
import { readDocSource } from "../lib/docs";
import { docsNav, routeFor } from "../lib/nav";
import { ORIGIN } from "../lib/site";

export const GET: APIRoute = () => {
  const sections = docsNav.map((entry) => {
    // Drop the frontmatter block; it is site plumbing, not content.
    const source = readDocSource(entry.file).replace(
      /^---\r?\n[\s\S]*?\r?\n---\r?\n/,
      "",
    );
    return `<!-- source: docs/${entry.file} | rendered: ${ORIGIN}${routeFor(entry)} -->\n\n${source.trim()}\n`;
  });

  const body = sections.join("\n\n---\n\n");

  return new Response(body, {
    headers: { "content-type": "text/plain; charset=utf-8" },
  });
};
