// Reads doc sources from the repo-root docs/ directory at build time.
//
// The site consumes ../docs by fs-read: on Vercel with root directory =
// site/, the full repo is still checked out, so ../docs exists at build.
// Docs are never vendored or copied into site/.

import fs from "node:fs";
import path from "node:path";
import { parse as parseYaml } from "yaml";

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
