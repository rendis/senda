import NextAuth from "next-auth";

declare module "next-auth" {
  interface Session {
    idToken?: string;
    error?: "RefreshTokenError";
  }
}

declare module "@auth/core/jwt" {
  interface JWT {
    idToken?: string;
    accessToken?: string;
    refreshToken?: string;
    expiresAt?: number;
    error?: "RefreshTokenError";
  }
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
        // First-time login — persist all tokens from the OIDC provider.
        return {
          ...token,
          idToken: account.id_token as string | undefined,
          accessToken: account.access_token as string | undefined,
          refreshToken: account.refresh_token as string | undefined,
          expiresAt: account.expires_at as number | undefined,
        };
      }

      // Subsequent requests — check if token is still valid.
      if (
        typeof token.expiresAt === "number" &&
        Date.now() < token.expiresAt * 1000
      ) {
        return token;
      }

      // Token expired — attempt refresh via Keycloak token endpoint.
      if (typeof token.refreshToken !== "string") {
        token.error = "RefreshTokenError";
        return token;
      }

      try {
        const issuer = process.env.AUTH_OIDC_ISSUER;
        const response = await fetch(
          `${issuer}/protocol/openid-connect/token`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/x-www-form-urlencoded",
            },
            body: new URLSearchParams({
              client_id: process.env.AUTH_OIDC_ID!,
              client_secret: process.env.AUTH_OIDC_SECRET!,
              grant_type: "refresh_token",
              refresh_token: token.refreshToken,
            }),
          }
        );

        const tokens = await response.json();

        if (!response.ok) throw tokens;

        return {
          ...token,
          idToken: tokens.id_token as string | undefined,
          accessToken: tokens.access_token as string | undefined,
          expiresAt: Math.floor(
            Date.now() / 1000 + (tokens.expires_in as number)
          ),
          refreshToken:
            (tokens.refresh_token as string | undefined) ?? token.refreshToken,
          error: undefined,
        };
      } catch (error) {
        console.error("Error refreshing token", error);
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
      // Block access if session is missing or token refresh failed.
      // This forces an immediate redirect to /login via the proxy.
      if (!auth) return false;
      if (auth.error === "RefreshTokenError") return false;
      return true;
    },
  },
});
