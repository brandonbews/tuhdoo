// tuhdoo's website: static HTML and CSS, no JavaScript on any page.
// The docs are read from ../docs at build time (src/lib/docs.ts); nothing is
// copied. `npm run build` writes dist/, which Cloudflare Pages serves as is.
import fs from "node:fs";
import { defineConfig } from "astro/config";
import { ORIGIN } from "./src/lib/site.ts";

// Hosts allowed to reach `astro dev` from another machine, e.g. when
// previewing over a VPN or tunnel. Machine-specific, so it is configured per
// checkout rather than committed: set DEV_ALLOWED_HOSTS in site/.env.local
// (gitignored) to a comma-separated list of hostnames.
//
//   DEV_ALLOWED_HOSTS=my-box.example.ts.net,192.168.1.50
//
// When it is set, the dev server also listens on the network, not just on
// localhost. Unset is the normal case.
if (fs.existsSync(".env.local")) process.loadEnvFile(".env.local");
const devHosts =
  process.env.DEV_ALLOWED_HOSTS?.split(",")
    .map((host) => host.trim())
    .filter(Boolean) ?? [];

export default defineConfig({
  site: ORIGIN,
  output: "static",
  // Pages build to /docs/joining.html, which Cloudflare Pages serves at
  // /docs/joining with a 200 (and redirects /docs/joining/ and
  // /docs/joining.html there). The canonical URLs have no trailing slash;
  // 'never' makes the dev server agree.
  build: { format: "file" },
  trailingSlash: "never",
  // Astro 7 defaults to 'jsx', which drops the space between inline elements
  // on separate lines. true removes whitespace only where HTML would ignore
  // it anyway, so the page renders exactly as written.
  compressHTML: true,
  // Off: no prefetch script on any page.
  prefetch: false,
  devToolbar: { enabled: false },
  ...(devHosts.length > 0
    ? { server: { host: true, allowedHosts: devHosts } }
    : {}),
});
