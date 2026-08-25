// Site-owned navigation config for the docs section.
//
// Ordering and sidebar titles live HERE, never in content frontmatter —
// the content files in ../docs are the published docs root and must stay
// site-agnostic (GFM + title/description frontmatter only).
//
// `file` is the path relative to the docs root (repo-root docs/).
// `slug` is the route below /docs; [] is the docs index.

export type NavEntry = {
  slug: string[];
  file: string;
  title: string;
};

export const docsNav: NavEntry[] = [
  { slug: [], file: "README.md", title: "Overview" },
  { slug: ["steering"], file: "steering.md", title: "Steering a backlog" },
  { slug: ["adopting"], file: "adopting.md", title: "Adopting tuhdoo" },
  { slug: ["joining"], file: "joining.md", title: "Joining a repo" },
  {
    slug: ["agent-protocol"],
    file: "agent-protocol.md",
    title: "Agent protocol",
  },
  { slug: ["recipes"], file: "recipes/README.md", title: "Workflow recipes" },
  {
    slug: ["recipes", "trunk-based-pr-flow"],
    file: "recipes/trunk-based-pr-flow.md",
    title: "Trunk-based PR flow",
  },
  {
    slug: ["recipes", "vercel"],
    file: "recipes/vercel.md",
    title: "Vercel and the data branch",
  },
  { slug: ["uninstall"], file: "uninstall.md", title: "Uninstalling" },
];

export function routeFor(entry: NavEntry): string {
  return entry.slug.length === 0 ? "/docs" : `/docs/${entry.slug.join("/")}`;
}

export function findBySlug(slug: string[]): NavEntry | undefined {
  const key = slug.join("/");
  return docsNav.find((e) => e.slug.join("/") === key);
}

export function prevNext(entry: NavEntry): {
  prev: NavEntry | undefined;
  next: NavEntry | undefined;
} {
  const i = docsNav.indexOf(entry);
  return { prev: docsNav[i - 1], next: docsNav[i + 1] };
}
