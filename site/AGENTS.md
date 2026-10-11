# tuhdoo website

Static HTML and CSS built with Astro 7, with no JavaScript on any page. The host is Cloudflare Pages, connected to the repo: it builds on every push to `main` with root directory `site/`, build command `npm run build` and output directory `dist/`, and serves `dist/` as is.

**Astro 7 is newer than your training data.** It changed the compiler (stricter HTML: close every tag, no `<div>` in a `<p>`), made Sätteri the default Markdown processor (this site doesn't use Astro's Markdown; see below), and defaults `compressHTML` to `'jsx'`, which this site sets to `true`. Read https://docs.astro.build/en/guides/upgrade-to/v7/ and the config types in `node_modules/astro/dist/types/public/config.d.ts` before relying on what you remember. Heed deprecation notices.

## Commands

Run them from `site/`. The build reads `../docs`, so it must run inside the repo.

```sh
npm ci              # install exactly what package-lock.json says (no install scripts run: .npmrc)
npm run dev         # dev server at http://localhost:4321
npm run lint        # Biome: lint and format check (npm run format fixes)
npm run typecheck   # astro check: types in .astro and .ts files
npm run build       # writes dist/, then fails if dist/ has any JavaScript (scripts/check-no-js.mjs)
npm run preview     # serves dist/ (Astro's server, not Cloudflare's)
```

From the repo root, `make test lint` runs the same build, lint and typecheck after the Go suite.

When Astro detects an agent, `dev` and `preview` start in the background and return. Stop them when you're done: `npx astro dev stop`, `npx astro preview stop`.

To preview from another machine (over Tailscale, say), put `DEV_ALLOWED_HOSTS=<host>[,<host>…]` in `site/.env.local` (gitignored): the dev server then listens on the network and answers to those hostnames.

To check what Cloudflare Pages will actually serve (status codes, redirects, `_headers`), build and run `npx wrangler pages dev dist`. Wrangler isn't a dependency; `npx` fetches it.

The npm scripts set `ASTRO_TELEMETRY_DISABLED=1`. Calling `npx astro …` directly doesn't, so set it yourself.

## Rules

- **No JavaScript on any page.** No client components, no `<script>`, no prefetch. `npm run build` fails if a built page has a `<script>` or the build writes a `.js` file, so CI fails too. If a page ever needs some, that is a deliberate decision that changes this rule, not a workaround.
- **The docs stay in `../docs`,** as GitHub-flavoured Markdown, never copied into `site/`, never MDX. Everything in `docs/` is public content; frontmatter holds only `title` and `description`, which feed `<title>`, the meta description and the link-preview tags. `src/lib/nav.ts` lists the published pages, their sidebar titles and their order. `src/lib/markdown.ts` is the whole rendering pipeline (unified/remark/rehype, not Astro's Markdown): it rewrites relative `.md` links into site routes and gives headings GitHub's anchor ids.
- **URLs have no trailing slash.** `build.format: 'file'` writes `/docs/joining.html`, which Cloudflare Pages serves at `/docs/joining` (and 308-redirects `/docs/joining/` and `/docs/joining.html` there). Canonical URLs, the sitemap and `llms.txt` all use the slashless form on `https://www.tuhdoo.com` (`ORIGIN` in `src/lib/site.ts`).
- **Response headers** live in `public/_headers` (Cloudflare Pages' format). There is no server, so nothing else can set them.
- **Styles** are plain CSS in `src/styles/global.css`: design tokens as custom properties, light and dark via `prefers-color-scheme`, and semantic component classes. No Tailwind, no CSS reset. Every text colour pair clears WCAG AA (4.5:1); the computed ratios are in the comment there — recompute them when a colour changes.
- **Fonts:** headings use Sora, self-hosted from `public/fonts/` (SIL OFL, licence beside the files); body text uses the system font stack. No request ever goes to Google Fonts.
- **Dependencies** are pinned to exact versions (`save-exact` in `.npmrc`). Add one only when the site needs it.
