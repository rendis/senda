export function normalizeWorkspaceTestRecipientAddresses(
  value: readonly string[] | null | undefined,
): string[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter((item): item is string => typeof item === "string");
}

export function formatWorkspaceTestRecipientAddresses(
  value: readonly string[] | null | undefined,
): string {
  return normalizeWorkspaceTestRecipientAddresses(value).join("\n");
}
