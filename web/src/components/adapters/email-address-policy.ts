const REASONABLE_EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function normalizeReasonableEmailAddress(value: string) {
  const email = value.trim();
  if (!REASONABLE_EMAIL_PATTERN.test(email)) return null;
  return email;
}
