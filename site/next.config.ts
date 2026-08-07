import type { NextConfig } from "next";

// Every page is statically generated at build time. `output: 'export'` is
// deliberately NOT set (deferred decision); the pages are static regardless
// because nothing uses a dynamic API or server action.
const nextConfig: NextConfig = {};

export default nextConfig;
