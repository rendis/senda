import { readFile } from "node:fs/promises";
import path from "node:path";
import { NextResponse } from "next/server";
import { authWithoutRefresh } from "@/auth";

function resolveOpenAPIPath() {
  const cwd = process.cwd();
  const repoRoot = path.basename(cwd) === "web" ? path.dirname(cwd) : cwd;

  return path.join(repoRoot, "cmd", "senda", "docs", "openapi.yaml");
}

export async function GET(request: Request) {
  const session = await authWithoutRefresh();

  if (!session) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  const specPath = resolveOpenAPIPath();
  const raw = await readFile(specPath, "utf8");

  return new NextResponse(raw, {
    headers: {
      "content-type": "application/yaml; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}
