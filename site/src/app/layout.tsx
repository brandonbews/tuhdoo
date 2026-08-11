import type { Metadata } from "next";
import { Sora } from "next/font/google";
import Link from "next/link";
import { TuhdooLogo } from "@/components/logo";
import "./globals.css";

// Sora carries the brand (the wordmark is Sora 800); headings use it via
// --font-sora → --font-heading in globals.css. Body text stays a system
// sans stack — long-form docs readability over brand at paragraph sizes.
const sora = Sora({
  subsets: ["latin"],
  variable: "--font-sora",
});

const SITE_TITLE = "tuhdoo — a coordination fabric for agent fleets";
const SITE_DESCRIPTION =
  "A shared backlog, work queue, and activity ledger for agent fleets, living in a git branch inside the repo it plans. No server, no vendor, no accounts.";

export const metadata: Metadata = {
  metadataBase: new URL("https://tuhdoo.com"),
  title: {
    default: SITE_TITLE,
    template: "%s · tuhdoo",
  },
  description: SITE_DESCRIPTION,
  openGraph: {
    siteName: "tuhdoo",
    type: "website",
    url: "/",
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
  },
  twitter: {
    card: "summary_large_image",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={sora.variable}>
      <body>
        <header className="site-header">
          <div className="site-header-inner">
            <Link href="/" className="lockup" aria-label="tuhdoo — home">
              <TuhdooLogo />
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
