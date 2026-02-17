export { auth as middleware } from "@/auth";

export const config = {
  matcher: [
    /*
     * Match all routes under (dashboard) group.
     * Exclude: /login, /access-denied, /onboarding, /api/*, /_next/*, static assets.
     * API proxy routes (/api/v1/*) are excluded so the Authorization header
     * is forwarded unmodified to the backend via Next.js rewrites.
     */
    "/((?!login|access-denied|onboarding|api/|_next/static|_next/image|favicon.ico).*)",
  ],
};
