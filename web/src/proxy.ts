import { NextRequest, NextResponse } from "next/server";
import { auth } from "@/auth";
import { LOCALE_COOKIE, DEFAULT_LOCALE } from "@/lib/locale";

// next-auth v5: auth(handler) wraps the handler with auth checks.
// The authorized callback in auth.ts handles redirect to /login.
export default auth(function proxy(req: NextRequest) {
  const res = NextResponse.next();

  // Set default locale cookie on first visit if absent
  if (!req.cookies.get(LOCALE_COOKIE)) {
    res.cookies.set(LOCALE_COOKIE, DEFAULT_LOCALE, {
      path: "/",
      maxAge: 60 * 60 * 24 * 365,
      sameSite: "lax",
    });
  }

  return res;
});

export const config = {
  matcher: [
    "/((?!login|access-denied|onboarding|api/|_next/static|_next/image|favicon.ico).*)",
  ],
};
