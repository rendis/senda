import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";
import { createMDX } from "fumadocs-mdx/next";
import path from "path";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");
const withMDX = createMDX();
const workspaceRoot =
  path.basename(process.cwd()) === "web"
    ? process.cwd()
    : path.join(process.cwd(), "web");

const nextConfig: NextConfig = {
  turbopack: {
    root: workspaceRoot,
  },
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/v1/:path*`,
      },
    ];
  },
};

export default withMDX(withNextIntl(nextConfig));
