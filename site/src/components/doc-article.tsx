import { PrevNext } from "@/components/prev-next";
import { readDocSource } from "@/lib/docs";
import { renderMarkdown } from "@/lib/markdown";
import type { NavEntry } from "@/lib/nav";

export async function DocArticle({ entry }: { entry: NavEntry }) {
  const content = await renderMarkdown(readDocSource(entry.file), entry.file);
  return (
    <main className="doc">
      <article>{content}</article>
      <PrevNext entry={entry} />
    </main>
  );
}
