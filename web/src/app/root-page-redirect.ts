export function decideRootRedirectPath(
  session: { idToken?: string | null } | null | undefined,
): "/login" | "/global" {
  return session?.idToken ? "/global" : "/login";
}
