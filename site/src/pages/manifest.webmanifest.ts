// Web app manifest, served at /manifest.webmanifest. The PNG renditions in
// public/ are rasterized from the flat favicon mark (public/icon.svg).
import type { APIRoute } from "astro";

const manifest = {
  name: "tuhdoo: a coordination fabric for agent fleets",
  short_name: "tuhdoo",
  description:
    "tuhdoo gives agent fleets a shared backlog, work queue, and activity ledger, stored on a git branch inside the repo it plans. It syncs over the remote you already have and needs no server, no vendor, and no accounts.",
  start_url: "/",
  display: "minimal-ui",
  // Manifest colors cannot vary by color scheme; these are the light-theme
  // surface. Dark browser chrome is handled by the theme-color meta pair in
  // src/layouts/Base.astro.
  background_color: "#ffffff",
  theme_color: "#ffffff",
  icons: [
    { src: "/icon.svg", sizes: "any", type: "image/svg+xml" },
    { src: "/icon-192.png", sizes: "192x192", type: "image/png" },
    { src: "/icon-512.png", sizes: "512x512", type: "image/png" },
  ],
};

export const GET: APIRoute = () =>
  new Response(JSON.stringify(manifest), {
    headers: { "content-type": "application/manifest+json" },
  });
