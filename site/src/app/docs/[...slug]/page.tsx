import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { DocArticle } from "@/components/doc-article";
import { docPageMetadata } from "@/lib/docs";
import { docsNav, findBySlug } from "@/lib/nav";

// Every docs route is known from the nav config; anything else is a 404 at
// build time, never a runtime render.
export const dynamicParams = false;

export function generateStaticParams(): { slug: string[] }[] {
  return docsNav
    .filter((e) => e.slug.length > 0)
    .map((e) => ({ slug: e.slug }));
}

type Props = { params: Promise<{ slug: string[] }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const entry = findBySlug(slug);
  if (!entry) return {};
  return docPageMetadata(entry);
}

export default async function DocPage({ params }: Props) {
  const { slug } = await params;
  const entry = findBySlug(slug);
  if (!entry) notFound();
  return <DocArticle entry={entry} />;
}
