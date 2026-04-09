export function buildInjectorVariableHint({
  fieldDescription,
  injectorDescription,
  fallbackHint,
}: {
  fieldDescription?: string;
  injectorDescription?: string;
  fallbackHint: string;
}): string {
  const normalizedFieldDescription = normalizeDescription(fieldDescription);
  const normalizedInjectorDescription = normalizeDescription(injectorDescription);

  if (normalizedFieldDescription && normalizedInjectorDescription) {
    if (normalizedFieldDescription === normalizedInjectorDescription) {
      return normalizedFieldDescription;
    }
    return `${normalizedFieldDescription} · ${normalizedInjectorDescription}`;
  }

  return (
    normalizedFieldDescription ??
    normalizedInjectorDescription ??
    fallbackHint
  );
}

function normalizeDescription(value?: string): string | undefined {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}
