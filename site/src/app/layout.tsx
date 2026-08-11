import type { Metadata, Viewport } from "next";
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
  // www is the canonical host: the apex 308-redirects to it (Vercel domain
  // config), so every absolute URL we emit (canonical, og:url, og:image,
  // sitemap) must live on www or crawlers see a redirect hop.
  metadataBase: new URL("https://www.tuhdoo.com"),
  title: {
    default: SITE_TITLE,
    template: "%s · tuhdoo",
  },
  description: SITE_DESCRIPTION,
  // Canonical for the landing page. Docs pages set their own canonical (and
  // og) in docPageMetadata — nested metadata objects replace wholesale, so
  // this only ever applies to routes that don't override it.
  alternates: {
    canonical: "/",
  },
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

export const viewport: Viewport = {
  // Browser chrome color, matched to --color-bg in each theme.
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#ffffff" },
    { media: "(prefers-color-scheme: dark)", color: "#060806" },
  ],
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={sora.variable}>
      <body>
        <a className="skip-link" href="#main">
          Skip to content
        </a>
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
