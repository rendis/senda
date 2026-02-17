export { auth as middleware } from "@/auth";

export const config = {
  matcher: [
    /*
     * Match all routes under (dashboard) group.
     * Exclude: /login, /access-denied, /api/auth/*, /_next/*, static assets.
     */
    "/((?!login|access-denied|onboarding|api/auth|_next/static|_next/image|favicon.ico).*)",
  ],
};
