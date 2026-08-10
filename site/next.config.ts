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
};

export default nextConfig;
