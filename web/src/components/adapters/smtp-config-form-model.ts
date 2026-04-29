import type { SmtpConfig, SmtpTLSMode } from "@/types/adapters";

type SmtpAuthMode = NonNullable<SmtpConfig["auth_mode"]>;
type SmtpConfigMeta = Partial<Record<"host" | "port" | "tls_mode" | "auth_mode", string>>;

type SmtpAuthValidationParams = {
  username: string;
  password: string;
  isEdit: boolean;
  hasExistingAuth: boolean;
  clearAuth: boolean;
};

type BuildSmtpConfigParams = {
  host: string;
  port: string;
  tlsMode: SmtpTLSMode;
  authMode: SmtpAuthMode;
  username: string;
  password: string;
  isEdit: boolean;
  clearAuth: boolean;
  previousConfig?: SmtpConfigMeta;
};

export function hasExistingSmtpAuth(config: SmtpConfigMeta | undefined) {
  return !!config?.auth_mode;
}

export function validateSmtpAuthFields({
  username,
  password,
  isEdit,
  hasExistingAuth,
  clearAuth,
}: SmtpAuthValidationParams):
  | { valid: true }
  | { valid: false; message: string } {
  if (clearAuth) return { valid: true };

  const cleanedUsername = username.trim();
  const cleanedPassword = password.trim();

  if (!cleanedUsername && !cleanedPassword) return { valid: true };
  if (cleanedUsername && cleanedPassword) return { valid: true };
  if (isEdit && hasExistingAuth && cleanedUsername && !cleanedPassword) {
    return { valid: true };
  }

  return {
    valid: false,
    message: "SMTP username and password must be provided together.",
  };
}

export function buildSmtpConfig({
  host,
  port,
  tlsMode,
  authMode,
  username,
  password,
  isEdit,
  clearAuth,
  previousConfig,
}: BuildSmtpConfigParams): SmtpConfig | undefined {
  const cleanedHost = host.trim();
  const cleanedPort = port.trim();
  const cleanedUsername = username.trim();
  const cleanedPassword = password.trim();
  const previousHost = previousConfig?.host ?? "";
  const previousPort = previousConfig?.port ?? "587";
  const previousTLSMode = (previousConfig?.tls_mode as SmtpTLSMode | undefined) ?? "starttls";
  const previousAuthMode = (previousConfig?.auth_mode as SmtpAuthMode | undefined) ?? "plain";
  const smtpChanged =
    cleanedHost !== previousHost ||
    cleanedPort !== previousPort ||
    tlsMode !== previousTLSMode ||
    authMode !== previousAuthMode ||
    !!cleanedUsername ||
    !!cleanedPassword ||
    clearAuth;

  if (isEdit && !smtpChanged) return undefined;

  return {
    host: cleanedHost,
    port: Number(cleanedPort),
    tls_mode: tlsMode,
    ...(clearAuth ? { username: "" } : {}),
    ...(!clearAuth && (cleanedUsername || cleanedPassword) ? { auth_mode: authMode } : {}),
    ...(!clearAuth && cleanedUsername ? { username: cleanedUsername } : {}),
    ...(!clearAuth && cleanedPassword ? { password: cleanedPassword } : {}),
  };
}
