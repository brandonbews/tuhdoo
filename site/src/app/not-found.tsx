import type { Metadata } from "next";
import Link from "next/link";

// Rendered inside the root layout, so the header and footer chrome come for
// free; the body reuses the landing classes — no new visual language. Next
// serves this with a real 404 status and a noindex robots meta on its own.
export const metadata: Metadata = {
  title: "Page not found",
  // Neutralize the canonical inherited from the root layout — a 404 must not
  // declare itself a copy of the landing page.
  alternates: {
    canonical: null,
  },
};

export default function NotFound() {
  return (
    <main id="main" className="landing">
      <section className="hero">
        <h1>404. No such page.</h1>
        <p className="lede">
          There is no page at this address. Try the docs, or start over from the
          landing page.
        </p>
        <div className="hero-actions">
          <Link className="button" href="/">
            Back to the start
          </Link>
          <Link className="button" href="/docs">
            Read the docs
          </Link>
        </div>
      </section>
    </main>
  );
}
