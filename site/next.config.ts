import type { NextConfig } from "next";

// Every page is statically generated at build time. `output: 'export'` is
// deliberately NOT set (deferred decision); the pages are static regardless
// because nothing uses a dynamic API or server action.
const nextConfig: NextConfig = {
  // Dev-only. Next blocks cross-origin dev requests, which breaks previewing
  // `next dev` from another machine over Tailscale. These are MagicDNS names:
  // they resolve only inside the tailnet and grant no access on their own, so
  // they are safe to commit. Exact strings only — no wildcard support.
  allowedDevOrigins: ["agentbox.dove-bangus.ts.net"],
};

export default nextConfig;
