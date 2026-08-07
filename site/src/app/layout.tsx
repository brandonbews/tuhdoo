import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "tuhdoo — a coordination fabric for agent fleets",
    template: "%s · tuhdoo",
  },
  description:
    "A shared backlog, work queue, and activity ledger for agent fleets, living in a git branch inside the repo it plans. No server, no vendor, no accounts.",
  metadataBase: new URL("https://tuhdoo.com"),
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="site-header">
          <div className="site-header-inner">
            <Link href="/" className="wordmark">
              tuhdoo
            </Link>
            <nav aria-label="Site">
              <Link href="/docs">Docs</Link>
              <a href="https://github.com/brandonbews/tuhdoo" rel="noopener">
                GitHub
              </a>
            </nav>
          </div>
        </header>
        {children}
        <footer className="site-footer">
          <div className="site-footer-inner">
            <span>tuhdoo — coordination over git, nothing else.</span>
            <a href="https://github.com/brandonbews/tuhdoo" rel="noopener">
              github.com/brandonbews/tuhdoo
            </a>
          </div>
        </footer>
      </body>
    </html>
  );
}
