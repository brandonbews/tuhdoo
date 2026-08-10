// The whole docs pipeline, thin by design:
//
//   remark-parse → remark-frontmatter → remark-gfm
//     → [one custom link-rewrite visitor]
//     → remark-rehype → rehype-slug → rehype-react
//
// Content is parsed as GFM, never MDX: anything that renders on GitHub
// renders here. rehype-slug uses github-slugger, so heading anchors match
// GitHub's slug rules exactly (e.g. "## For the repo admin: branch
// protection and CI" → #for-the-repo-admin-branch-protection-and-ci).

import path from "node:path";
import type { Root } from "mdast";
import type { ReactNode } from "react";
import * as jsxRuntime from "react/jsx-runtime";
import rehypeReact from "rehype-react";
import rehypeSlug from "rehype-slug";
import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import { unified } from "unified";
import { visit } from "unist-util-visit";

import { markdownComponents } from "@/components/markdown-map";

// The one custom step: rewrite relative .md links into site routes.
//
// Handled cases (relative to the directory of the doc being rendered):
//   joining.md                          → /docs/joining
//   recipes/README.md                   → /docs/recipes
//   recipes/trunk-based-pr-flow.md      → /docs/recipes/trunk-based-pr-flow
//   ../agent-protocol.md#the-loop       → /docs/agent-protocol#the-loop  (anchor kept)
//   README.md (from recipes/)           → /docs/recipes
//   #anchor-only, /absolute, https://…  → untouched
//   relative links that are not .md, or that escape docs/ → untouched
function remarkRewriteMdLinks(options: { docDir: string }) {
  return (tree: Root) => {
    visit(tree, ["link", "definition"], (node) => {
      if (node.type !== "link" && node.type !== "definition") return;
      node.url = rewriteMdLink(node.url, options.docDir);
    });
  };
}

export function rewriteMdLink(url: string, docDir: string): string {
  if (/^[a-z][a-z0-9+.-]*:/i.test(url)) return url; // absolute (https:, mailto:, …)
  if (url.startsWith("//") || url.startsWith("/") || url.startsWith("#"))
    return url;

  const hashIndex = url.indexOf("#");
  const filePart = hashIndex === -1 ? url : url.slice(0, hashIndex);
  const hash = hashIndex === -1 ? "" : url.slice(hashIndex);
  if (!filePart.endsWith(".md")) return url;

  // Resolve against the current doc's directory within docs/.
  const resolved = path.posix.normalize(path.posix.join(docDir, filePart));
  if (resolved.startsWith("..")) return url; // escapes the docs root; leave it

  let route = resolved.slice(0, -".md".length);
  if (route === "README") route = "";
  else if (route.endsWith("/README")) route = route.slice(0, -"/README".length);

  return (route === "" ? "/docs" : `/docs/${route}`) + hash;
}

const runtime = {
  Fragment: jsxRuntime.Fragment,
  jsx: jsxRuntime.jsx,
  jsxs: jsxRuntime.jsxs,
};

// Render a doc source file (frontmatter included) to React elements.
// The frontmatter yaml node is tokenized by remark-frontmatter and dropped
// by remark-rehype, so it never appears in the rendered body.
export async function renderMarkdown(
  source: string,
  docFile: string,
): Promise<ReactNode> {
  const processor = unified()
    .use(remarkParse)
    .use(remarkFrontmatter, ["yaml"])
    .use(remarkGfm)
    .use(remarkRewriteMdLinks, { docDir: path.posix.dirname(docFile) })
    .use(remarkRehype)
    .use(rehypeSlug)
    .use(rehypeReact, { ...runtime, components: markdownComponents });

  const file = await processor.process(source);
  return file.result as ReactNode;
}
