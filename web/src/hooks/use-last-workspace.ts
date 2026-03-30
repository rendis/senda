const STORAGE_KEY = "senda:last-workspace-path";

export function getLastWorkspacePath(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

export function setLastWorkspacePath(path: string): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(STORAGE_KEY, path);
  } catch {
    // localStorage may be full or disabled
  }
}
