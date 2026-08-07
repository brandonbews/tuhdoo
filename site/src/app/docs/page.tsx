import type { Metadata } from "next";
import { DocArticle } from "@/components/doc-article";
import { readFrontmatter } from "@/lib/docs";
import { docsNav } from "@/lib/nav";

const entry = docsNav[0]; // docs index ← docs/README.md

export function generateMetadata(): Metadata {
  const { title, description } = readFrontmatter(entry.file);
  return { title, description };
}

export default function DocsIndexPage() {
  return <DocArticle entry={entry} />;
}
