"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { docsNav, routeFor } from "@/lib/nav";

export function Sidebar() {
  const pathname = usePathname();
  return (
    <nav className="docs-sidebar" aria-label="Docs">
      <ul>
        {docsNav.map((entry) => {
          const href = routeFor(entry);
          const nested = entry.slug.length > 1;
          return (
            <li key={href} className={nested ? "nested" : undefined}>
              <Link
                href={href}
                aria-current={pathname === href ? "page" : undefined}
              >
                {entry.title}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
