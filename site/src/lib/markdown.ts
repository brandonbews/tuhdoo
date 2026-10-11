// The whole docs pipeline, thin by design:
//
//   remark-parse → remark-frontmatter → remark-gfm
//     → [rewrite relative .md links into site routes]
//     → remark-rehype → rehype-slug
//     → [doc elements: heading anchors, link rel, code/table wrappers, …]
//     → rehype-stringify
//
// Content is parsed as GFM, never MDX: anything that renders on GitHub
// renders here. Raw HTML in a doc is dropped (remark-rehype's default).
// rehype-slug uses github-slugger, so heading anchors match GitHub's slug
// rules exactly (e.g. "## For the repo admin: branch protection and CI" →
// #for-the-repo-admin-branch-protection-and-ci).
//
// The output is an HTML string, placed into the page with set:html. It runs
// at build time only; no JavaScript reaches the browser.

import path from "node:path";
import type { Element, Root as HastRoot } from "hast";
import type { Root as MdastRoot } from "mdast";
import rehypeSlug from "rehype-slug";
import rehypeStringify from "rehype-stringify";
import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import { unified } from "unified";
import { SKIP, visit } from "unist-util-visit";

// Step one: rewrite relative .md links into site routes.
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
  return (tree: MdastRoot) => {
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

// Step two: the complete list of what the site changes in the rendered
// HTML. An element not handled here is emitted exactly as remark-rehype
// produced it, so this function IS the full extent of custom rendering.
//
//   headings    h1–h6 with an id → text wrapped in <a class="anchor" href="#id">
//   links       a                → rel="noopener" unless the href starts with / or #
//   images      img              → loading="lazy", alt="" when the doc gave none
//   checkboxes  input            → disabled and readonly (GFM task lists)
//   table cells th td            → align="…" becomes style="text-align:…"
//   code blocks pre              → wrapped in <div class="codeblock" data-lang="…">
//   tables      table            → wrapped in <div class="table-scroll">
function rehypeDocElements() {
  return (tree: HastRoot) => {
    // Pass one: attributes and heading anchors, element by element.
    visit(tree, "element", (node) => {
      const tag = node.tagName;

      // No aria-label on the anchor: the wrapped heading text is the
      // accessible name. An override like "Link to this section" fails
      // WCAG 2.5.3 (label in name) — flagged by axe/Lighthouse on every
      // heading.
      if (/^h[1-6]$/.test(tag) && typeof node.properties.id === "string") {
        node.children = [
          {
            type: "element",
            tagName: "a",
            properties: {
              className: ["anchor"],
              href: `#${node.properties.id}`,
            },
            children: node.children,
          },
        ];
        return;
      }

      if (tag === "a") {
        const href = String(node.properties.href ?? "");
        if (!href.startsWith("/") && !href.startsWith("#")) {
          node.properties.rel = ["noopener"];
        }
        return;
      }

      // Docs images are plain markdown images; the CSS constrains them.
      if (tag === "img") {
        node.properties.alt ??= "";
        node.properties.loading = "lazy";
        return;
      }

      if (tag === "input") {
        node.properties.disabled = true;
        node.properties.readOnly = true;
        return;
      }

      // An inline style beats the stylesheet's text-align: left, so a
      // column aligned in the markdown (|:---:|) stays aligned.
      if ((tag === "th" || tag === "td") && node.properties.align) {
        node.properties.style = `text-align:${node.properties.align}`;
        delete node.properties.align;
      }
    });

    // Pass two: wrap code blocks and tables in a container that scrolls
    // when they are wider than the column.
    visit(tree, "element", (node, index, parent) => {
      if (!parent || index === undefined) return;

      if (node.tagName === "pre") {
        parent.children[index] = wrap(node, {
          className: ["codeblock"],
          dataLang: codeLanguage(node),
        });
        return [SKIP, index + 1];
      }

      if (node.tagName === "table") {
        parent.children[index] = wrap(node, { className: ["table-scroll"] });
        return [SKIP, index + 1];
      }
    });
  };
}

function wrap(node: Element, properties: Element["properties"]): Element {
  return { type: "element", tagName: "div", properties, children: [node] };
}

// Fenced code arrives as pre > code.language-xyz; the language becomes the
// badge on the code block. undefined (no data-lang) when there is none.
function codeLanguage(pre: Element): string | undefined {
  if (pre.children.length !== 1) return undefined;
  const code = pre.children[0];
  if (code.type !== "element") return undefined;
  const classes = code.properties.className;
  if (!Array.isArray(classes)) return undefined;
  const match = /language-([\w-]+)/.exec(classes.join(" "));
  return match ? match[1] : undefined;
}

// Render a doc source file (frontmatter included) to an HTML string.
// The frontmatter yaml node is tokenized by remark-frontmatter and dropped
// by remark-rehype, so it never appears in the rendered body.
export async function renderMarkdown(
  source: string,
  docFile: string,
): Promise<string> {
  const file = await unified()
    .use(remarkParse)
    .use(remarkFrontmatter, ["yaml"])
    .use(remarkGfm)
    .use(remarkRewriteMdLinks, { docDir: path.posix.dirname(docFile) })
    .use(remarkRehype)
    .use(rehypeSlug)
    .use(rehypeDocElements)
    .use(rehypeStringify)
    .process(source);
  return String(file);
}
