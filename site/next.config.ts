import { loadEnvConfig } from "@next/env";
import type { NextConfig } from "next";

// `.env` files are not loaded yet when this config is evaluated, so read them
// explicitly. `.env.local` is picked up in both dev and production modes.
loadEnvConfig(process.cwd());

// Hosts allowed to reach `next dev` cross-origin, e.g. when previewing the
// site from another machine over a VPN or tunnel. Machine-specific, so it is
// configured per checkout rather than committed: set NEXT_DEV_ORIGINS in
// site/.env.local (gitignored) to a comma-separated list of hostnames.
//
//   NEXT_DEV_ORIGINS=my-box.example.ts.net,192.168.1.50
//
// Exact hostnames only — Next does not support wildcards here. Unset is the
// normal case; it only matters when the browser is not on this machine.
const devOrigins =
  process.env.NEXT_DEV_ORIGINS?.split(",")
    .map((origin) => origin.trim())
    .filter(Boolean) ?? [];

// Every page is statically generated at build time. `output: 'export'` is
// deliberately NOT set (deferred decision); the pages are static regardless
// because nothing uses a dynamic API or server action.
const nextConfig: NextConfig = {
  ...(devOrigins.length > 0 ? { allowedDevOrigins: devOrigins } : {}),

  // Security headers. Vercel supplies Strict-Transport-Security on its own;
  // the rest are ours. A full Content-Security-Policy is deliberately not set:
  // Next's inline bootstrap scripts would need nonces or 'unsafe-inline', and
  // on a fully static site with no user content or third-party scripts the
  // added risk surface is minimal — revisit if the site ever embeds anything.
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          {
            key: "Referrer-Policy",
            value: "strict-origin-when-cross-origin",
          },
          {
            key: "Permissions-Policy",
            value: "camera=(), microphone=(), geolocation=()",
          },
        ],
      },
    ];
  },
};

export default nextConfig;
