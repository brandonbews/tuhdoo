import Link from "next/link";
import { type NavEntry, prevNext, routeFor } from "@/lib/nav";

export function PrevNext({ entry }: { entry: NavEntry }) {
  const { prev, next } = prevNext(entry);
  if (!prev && !next) return null;
  return (
    <nav className="prev-next" aria-label="Docs pages">
      {prev && (
        <Link href={routeFor(prev)} className="prev">
          <span className="direction">← Previous</span>
          <span>{prev.title}</span>
        </Link>
      )}
      {next && (
        <Link href={routeFor(next)} className="next">
          <span className="direction">Next →</span>
          <span>{next.title}</span>
        </Link>
      )}
    </nav>
  );
}
