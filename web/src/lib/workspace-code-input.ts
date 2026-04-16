const MAX_WORKSPACE_CODE_LENGTH = 50;

function clampWorkspaceCode(value: string) {
  return value.slice(0, MAX_WORKSPACE_CODE_LENGTH);
}

export function sanitizeWorkspaceCodeInput(value: string): string {
  return clampWorkspaceCode(
    value
      .toLowerCase()
      .replace(/[^a-z0-9_-]/g, "")
      .replace(/^[\-_]+/, ""),
  );
}

export function normalizeWorkspaceCodeInput(value: string): string {
  return sanitizeWorkspaceCodeInput(value).replace(/[\-_]+$/, "");
}
