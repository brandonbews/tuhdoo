// Reads doc sources from the repo-root docs/ directory at build time.
//
// The site consumes ../docs by fs-read: on Vercel with root directory =
// site/, the full repo is still checked out, so ../docs exists at build.
// Docs are never vendored or copied into site/.

import fs from "node:fs";
import path from "node:path";
import type { Metadata } from "next";
import { parse as parseYaml } from "yaml";
import { type NavEntry, routeFor } from "@/lib/nav";

export const DOCS_ROOT = path.join(process.cwd(), "..", "docs");

export type DocFrontmatter = {
  title: string;
  description: string;
};

export function readDocSource(file: string): string {
  return fs.readFileSync(path.join(DOCS_ROOT, file), "utf8");
}

// Frontmatter is restricted (by content contract) to title + description.
// It feeds <title> and the meta description; it is never rendered as body
// text — the remark pipeline drops the yaml node separately.
export function readFrontmatter(file: string): DocFrontmatter {
  const source = readDocSource(file);
  const match = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/.exec(source);
  if (!match) {
    throw new Error(`missing frontmatter in docs/${file}`);
  }
  const data = parseYaml(match[1]) as Record<string, unknown>;
  const { title, description } = data;
  if (typeof title !== "string" || typeof description !== "string") {
    throw new Error(
      `frontmatter in docs/${file} must have string title and description`,
    );
  }
  return { title, description };
}

// Page metadata for a doc route. Next replaces nested `openGraph` objects
// wholesale (no per-field merge with the root layout), so each doc page must
// carry the full og object — siteName and images included; setting openGraph
// here would otherwise drop the image the app/opengraph-image.png file
// convention gives the root route. (Verified against the running site: an
// openGraph object without `images` loses og:image entirely.)
//
// Takes the nav entry (not just the file) because canonical and og:url need
// the route; relative URLs resolve against metadataBase (www.tuhdoo.com).
export function docPageMetadata(entry: NavEntry): Metadata {
  const { title, description } = readFrontmatter(entry.file);
  const route = routeFor(entry);
  return {
    title,
    description,
    alternates: {
      canonical: route,
    },
    openGraph: {
      siteName: "tuhdoo",
      type: "article",
      url: route,
      title,
      description,
      images: "/opengraph-image.png",
    },
  };
}
