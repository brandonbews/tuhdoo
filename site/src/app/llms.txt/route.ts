import { readFrontmatter } from "@/lib/docs";
import { docsNav, routeFor } from "@/lib/nav";

// llms.txt (https://llmstxt.org): a machine-readable index of the site for
// agents and LLM crawlers. Built from the same nav config and frontmatter as
// the human site, so it cannot drift from the published docs. Statically
// generated at build time (route handlers are dynamic by default since
// Next 15, hence force-static).
export const dynamic = "force-static";

const ORIGIN = "https://www.tuhdoo.com";

export function GET(): Response {
  const docLines = docsNav.map((entry) => {
    const { description } = readFrontmatter(entry.file);
    return `- [${entry.title}](${ORIGIN}${routeFor(entry)}): ${description}`;
  });

  const body = [
    "# tuhdoo",
    "",
    "> A coordination fabric for agent fleets, steered by humans: a shared backlog, work queue, and activity ledger living in a git orphan branch inside the repo it plans. Synced through an ordinary git remote — no server, no vendor, no accounts.",
    "",
    "Agents connect over a twelve-verb MCP surface and run one loop — claim, work, escalate, finish — recorded on the ledger. Humans capture and sculpt tasks, then steer by answering escalations and reviewing outcomes.",
    "",
    "The full documentation is concatenated in [llms-full.txt](" +
      `${ORIGIN}/llms-full.txt); the markdown source of every page lives in ` +
      "the repo at https://github.com/brandonbews/tuhdoo/tree/main/docs.",
    "",
    "## Docs",
    "",
    ...docLines,
    "",
  ].join("\n");

  return new Response(body, {
    headers: { "content-type": "text/plain; charset=utf-8" },
  });
}
