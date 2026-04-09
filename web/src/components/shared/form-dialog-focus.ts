export interface DialogFocusCandidate {
  isConnected: boolean;
  disabled: boolean;
  ariaHidden: string | null;
}

export function shouldRestoreDialogFocus(
  candidate: DialogFocusCandidate | null,
): boolean {
  if (!candidate) return false;
  if (!candidate.isConnected) return false;
  if (candidate.disabled) return false;
  if (candidate.ariaHidden === "true") return false;
  return true;
}
