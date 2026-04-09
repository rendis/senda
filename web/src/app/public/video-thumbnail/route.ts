import { NextRequest } from "next/server";
import { proxyVideoThumbnail } from "./proxy";

export const runtime = "nodejs";

export async function GET(request: NextRequest) {
  return proxyVideoThumbnail(request);
}
