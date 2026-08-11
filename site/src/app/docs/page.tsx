import type { Metadata } from "next";
import { DocArticle } from "@/components/doc-article";
import { docPageMetadata } from "@/lib/docs";
import { docsNav } from "@/lib/nav";

const entry = docsNav[0]; // docs index ← docs/README.md

export function generateMetadata(): Metadata {
  return docPageMetadata(entry.file);
}

export default function DocsIndexPage() {
  return <DocArticle entry={entry} />;
}
