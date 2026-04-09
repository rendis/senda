import { AsyncLocalStorage } from "node:async_hooks";
import NextAuth from "next-auth";

// ---------------------------------------------------------------------------
// Refresh-skip context: Server Components CANNOT write cookies, so token
// refresh inside their auth() call is wasted work that causes double-refresh
// race conditions.  authWithoutRefresh() sets a flag via AsyncLocalStorage
// so the JWT callback skips the refresh and returns the (possibly stale) token.
// Only proxy.ts (middleware) and /api/auth/session (SessionProvider refetch)
// should perform token refresh — they CAN write Set-Cookie headers.
// ---------------------------------------------------------------------------
const refreshContext = new AsyncLocalStorage<{ skip: boolean }>();

/**
 * Read session data without triggering token refresh.
 * Use this in Server Components (layouts, pages) where cookies can't be written.
 */
export async function authWithoutRefresh() {
  return refreshContext.run({ skip: true }, () => auth());
}

async function refreshAccessToken(refreshToken: string) {
  const issuer = process.env.AUTH_OIDC_ISSUER;
  const maxRetries = 3;
  let lastError: unknown;

  for (let attempt = 0; attempt < maxRetries; attempt++) {
    if (attempt > 0) {
      await new Promise((r) => setTimeout(r, 500 * Math.pow(3, attempt - 1)));
    }

    try {
      const response = await fetch(
        `${issuer}/protocol/openid-connect/token`,
        {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: new URLSearchParams({
            client_id: process.env.AUTH_OIDC_ID!,
            client_secret: process.env.AUTH_OIDC_SECRET!,
            grant_type: "refresh_token",
            refresh_token: refreshToken,
          }),
        }
      );

      const tokens = await response.json();
      if (!response.ok) throw tokens;
      return tokens as {
        id_token?: string;
        access_token?: string;
        expires_in: number;
        refresh_token?: string;
      };
    } catch (error) {
      lastError = error;
    }
  }

  throw lastError;
}

export const { handlers, auth, signIn, signOut } = NextAuth({
  providers: [
    {
      id: "oidc",
      name: "SSO",
      type: "oidc",
      issuer: process.env.AUTH_OIDC_ISSUER,
      clientId: process.env.AUTH_OIDC_ID,
      clientSecret: process.env.AUTH_OIDC_SECRET,
      authorization: {
        params: {
          scope: "openid profile email",
        },
      },
    },
  ],
  session: {
    strategy: "jwt",
    maxAge: 24 * 60 * 60, // 24 hours
  },
  pages: {
    signIn: "/login",
  },
  callbacks: {
    async jwt({ token, account }) {
      if (account) {
        return {
          ...token,
          idToken: account.id_token as string | undefined,
          accessToken: account.access_token as string | undefined,
          refreshToken: account.refresh_token as string | undefined,
          expiresAt: (account.expires_at as number | undefined) ?? Math.floor(Date.now() / 1000 + 300),
        };
      }

      // Subsequent requests — check if token is still valid.
      if (
        typeof token.expiresAt === "number" &&
        Date.now() < token.expiresAt * 1000
      ) {
        return token;
      }

      // Skip refresh when called from Server Components (they can't write cookies).
      if (refreshContext.getStore()?.skip) {
        return token;
      }

      // Token expired — attempt refresh via Keycloak token endpoint.
      if (typeof token.refreshToken !== "string") {
        token.error = "RefreshTokenError";
        return token;
      }

      try {
        const tokens = await refreshAccessToken(token.refreshToken);
        return {
          ...token,
          idToken: tokens.id_token as string | undefined,
          accessToken: tokens.access_token as string | undefined,
          expiresAt: Math.floor(Date.now() / 1000 + tokens.expires_in),
          refreshToken: tokens.refresh_token ?? token.refreshToken,
          error: undefined,
        };
      } catch (error) {
        console.error("Error refreshing token after retries", error);
        token.error = "RefreshTokenError";
        return token;
      }
    },
    session({ session, token }) {
      session.idToken = token.idToken as string | undefined;
      if (token.error) {
        session.error = token.error as "RefreshTokenError";
      }
      return session;
    },
    authorized({ auth }) {
      if (!auth) return false;
      if (auth.error === "RefreshTokenError") return false;
      return true;
    },
  },
});
