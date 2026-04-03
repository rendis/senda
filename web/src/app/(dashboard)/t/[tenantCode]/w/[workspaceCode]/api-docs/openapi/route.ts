import { NextResponse } from "next/server";
import { authWithoutRefresh } from "@/auth";
import dataPlaneOpenAPISpec from "@/lib/data-plane-openapi.json";

export async function GET(request: Request) {
  const session = await authWithoutRefresh();

  if (!session) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  return NextResponse.json(dataPlaneOpenAPISpec, {
    headers: {
      "cache-control": "no-store",
    },
  });
}
