import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";
import { createMDX } from "fumadocs-mdx/next";
import path from "path";
import { buildRewrites } from "./src/config/rewrites";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");
const withMDX = createMDX();
const workspaceRoot =
  path.basename(process.cwd()) === "web"
    ? process.cwd()
    : path.join(process.cwd(), "web");
const apiOrigin = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

const nextConfig: NextConfig = {
  turbopack: {
    root: workspaceRoot,
  },
  async rewrites() {
    return buildRewrites(apiOrigin);
  },
};

export default withMDX(withNextIntl(nextConfig));
