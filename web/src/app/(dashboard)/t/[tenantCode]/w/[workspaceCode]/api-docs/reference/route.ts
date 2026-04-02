import { NextResponse } from "next/server";
import { ApiReference } from "@scalar/nextjs-api-reference";
import { authWithoutRefresh } from "@/auth";

const scalarReference = ApiReference({
  url: "../openapi",
  theme: "purple",
  darkMode: true,
  metaData: {
    title: "Senda API Docs",
    description: "Interactive API reference for workspace integrations.",
  },
});

export async function GET(
  request: Request,
) {
  const session = await authWithoutRefresh();

  if (!session) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  return scalarReference();
}
