import type { InjectorDefinition } from "@/types/injectors";

const INJECTOR_PLACEHOLDER_PATTERN = /\{\{\s*(injector\.[^}]+?)\s*\}\}/g;
const INJECTOR_DATA_TOKEN_PATTERN = /data-variable-token=["'](injector\.[^"']+?)["']/g;
const INJECTOR_TOKEN_VALUE_PATTERN = /^injector\..+$/;

export type TemplateInjectorUsage = Record<string, string[]>;

export function extractInjectorNamesFromTemplateMarkup(markup: string): string[] {
  return Object.keys(extractInjectorUsageFromTemplateMarkup(markup));
}

export function extractInjectorUsageFromTemplateMarkup(markup: string): TemplateInjectorUsage {
  const usage = new Map<string, Set<string>>();
  collectInjectorUsageFromString(markup, usage);
  return toTemplateInjectorUsage(usage);
}

export function extractInjectorNamesFromTemplateSources(sources: unknown[]): string[] {
  return Object.keys(extractInjectorUsageFromTemplateSources(sources));
}

export function extractInjectorUsageFromTemplateSources(sources: unknown[]): TemplateInjectorUsage {
  const usage = new Map<string, Set<string>>();
  const seenObjects = new Set<object>();

  function walk(value: unknown) {
    if (typeof value === "string") {
      collectInjectorUsageFromString(value, usage);
      collectInjectorUsageFromTokenValue(value, usage);
      return;
    }

    if (Array.isArray(value)) {
      value.forEach(walk);
      return;
    }

    if (!value || typeof value !== "object") {
      return;
    }

    if (seenObjects.has(value)) {
      return;
    }
    seenObjects.add(value);

    for (const [key, nested] of Object.entries(value)) {
      if (key === "token" && typeof nested === "string") {
        collectInjectorUsageFromTokenValue(nested, usage);
      }
      walk(nested);
    }
  }

  sources.forEach(walk);
  return toTemplateInjectorUsage(usage);
}

export function resolveTestSendInjectorUsage({
  editorMode,
  builderDocument,
  codeMjml,
  metadataValues,
}: {
  editorMode: "visual" | "code";
  builderDocument: unknown;
  codeMjml: string;
  metadataValues: Array<string | undefined>;
}): TemplateInjectorUsage {
  if (editorMode === "visual") {
    return extractInjectorUsageFromTemplateSources([
      builderDocument,
      ...metadataValues,
    ]);
  }

  return extractInjectorUsageFromTemplateSources([
    codeMjml,
    ...metadataValues,
  ]);
}

export function resolveTestSendInjectorNames(args: {
  editorMode: "visual" | "code";
  builderDocument: unknown;
  codeMjml: string;
  metadataValues: Array<string | undefined>;
}): string[] {
  return Object.keys(resolveTestSendInjectorUsage(args));
}

export function filterInjectorDefinitionsByTemplateUsage<
  T extends Pick<InjectorDefinition, "name" | "fields">
>(
  injectors: T[],
  allowedInjectorUsage: TemplateInjectorUsage,
): T[] {
  const allowedInjectorNames = Object.keys(allowedInjectorUsage);
  if (!allowedInjectorNames.length) {
    return [];
  }

  return injectors.reduce<T[]>((acc, injector) => {
    const allowedFields = allowedInjectorUsage[injector.name];
    if (!allowedFields?.length) {
      return acc;
    }

    const filteredFields = (injector.fields ?? []).filter((field) =>
      allowedFields.includes(field.field_name),
    );

    if (!filteredFields.length) {
      return acc;
    }

    acc.push({
      ...injector,
      fields: filteredFields,
    });
    return acc;
  }, []);
}

function collectInjectorUsageFromString(
  value: string,
  usage: Map<string, Set<string>>,
) {
  if (!value.trim()) {
    return;
  }

  for (const pattern of [INJECTOR_PLACEHOLDER_PATTERN, INJECTOR_DATA_TOKEN_PATTERN]) {
    pattern.lastIndex = 0;
    for (const match of value.matchAll(pattern)) {
      const [injectorName, fieldName] = parseInjectorToken(match[1] ?? "");
      if (!injectorName || !fieldName) {
        continue;
      }
      addInjectorFieldUsage(usage, injectorName, fieldName);
    }
  }
}

function collectInjectorUsageFromTokenValue(
  value: string,
  usage: Map<string, Set<string>>,
) {
  if (!INJECTOR_TOKEN_VALUE_PATTERN.test(value)) {
    return;
  }

  const [injectorName, fieldName] = parseInjectorToken(value);
  if (!injectorName || !fieldName) {
    return;
  }

  addInjectorFieldUsage(usage, injectorName, fieldName);
}

function addInjectorFieldUsage(
  usage: Map<string, Set<string>>,
  injectorName: string,
  fieldName: string,
) {
  const fields = usage.get(injectorName) ?? new Set<string>();
  fields.add(fieldName);
  usage.set(injectorName, fields);
}

function parseInjectorToken(token: string): [string | undefined, string | undefined] {
  const normalized = token.trim().replace(/^injector\./, "");
  const separatorIndex = normalized.indexOf(".");
  if (separatorIndex <= 0) {
    return [undefined, undefined];
  }

  const injectorName = normalized.slice(0, separatorIndex).trim();
  const fieldName = normalized.slice(separatorIndex + 1).trim();
  return [injectorName || undefined, fieldName || undefined];
}

function toTemplateInjectorUsage(
  usage: Map<string, Set<string>>,
): TemplateInjectorUsage {
  return Object.fromEntries(
    Array.from(usage.entries()).map(([injectorName, fields]) => [
      injectorName,
      Array.from(fields),
    ]),
  );
}
