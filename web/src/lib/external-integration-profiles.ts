import type {
  ExternalIntegrationCapabilities,
  ExternalIntegrationMethodDescriptor,
  ExternalIntegrationProfile,
  ExternalIntegrationsSettings,
} from "@/types/settings";

export const EXTERNAL_INTEGRATION_CAPABILITY_KEYS = [
  "list_templates",
  "view_versions",
  "edit_versions",
  "publish_versions",
  "test_send",
  "builder_access",
  "metadata_access",
  "locale_access",
] as const satisfies ReadonlyArray<keyof ExternalIntegrationCapabilities>;

export const DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES: ExternalIntegrationCapabilities =
  {
    list_templates: false,
    view_versions: false,
    edit_versions: false,
    publish_versions: false,
    test_send: false,
    builder_access: false,
    metadata_access: false,
    locale_access: false,
  };

function normalizeList(values?: readonly string[] | null) {
  const normalized: string[] = [];
  const seen = new Set<string>();

  for (const rawValue of values ?? []) {
    const value = rawValue.trim();
    if (!value || seen.has(value)) {
      continue;
    }
    seen.add(value);
    normalized.push(value);
  }

  return normalized;
}

export function parseDelimitedEntries(value: string) {
  return normalizeList(
    value
      .split(/[\n,]/g)
      .map((entry) => entry.trim())
      .filter(Boolean),
  );
}

export function stringifyDelimitedEntries(values: readonly string[]) {
  return values.join("\n");
}

export function normalizeExternalIntegrationCapabilities(
  capabilities?: Partial<ExternalIntegrationCapabilities> | null,
): ExternalIntegrationCapabilities {
  return {
    list_templates:
      capabilities?.list_templates ??
      DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.list_templates,
    view_versions:
      capabilities?.view_versions ??
      DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.view_versions,
    edit_versions:
      capabilities?.edit_versions ??
      DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.edit_versions,
    publish_versions:
      capabilities?.publish_versions ??
      DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.publish_versions,
    test_send:
      capabilities?.test_send ?? DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.test_send,
    builder_access:
      capabilities?.builder_access ??
      DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.builder_access,
    metadata_access:
      capabilities?.metadata_access ??
      DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.metadata_access,
    locale_access:
      capabilities?.locale_access ??
      DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES.locale_access,
  };
}

export function createEmptyExternalIntegrationProfile(index = 1): ExternalIntegrationProfile {
  return {
    slug: `external-profile-${index}`,
    name: `External profile ${index}`,
    description: "",
    enabled: false,
    auth_method_name: "",
    resolver_name: "",
    allowed_origins: [],
    allowed_headers: [],
    required_headers: [],
    capabilities: {
      ...DEFAULT_EXTERNAL_INTEGRATION_CAPABILITIES,
    },
  };
}

export function normalizeExternalIntegrationProfile(
  profile?: Partial<ExternalIntegrationProfile> | null,
  fallbackIndex = 1,
): ExternalIntegrationProfile {
  const base = createEmptyExternalIntegrationProfile(fallbackIndex);

  return {
    slug: profile?.slug?.trim() || base.slug,
    name: profile?.name?.trim() || base.name,
    description: profile?.description?.trim() || "",
    enabled: profile?.enabled ?? base.enabled,
    auth_method_name: profile?.auth_method_name?.trim() || "",
    resolver_name: profile?.resolver_name?.trim() || "",
    allowed_origins: normalizeList(profile?.allowed_origins),
    allowed_headers: normalizeList(profile?.allowed_headers),
    required_headers: normalizeList(profile?.required_headers),
    capabilities: normalizeExternalIntegrationCapabilities(profile?.capabilities),
  };
}

export function normalizeExternalIntegrationSettings(
  settings?: Partial<ExternalIntegrationsSettings> | null,
): ExternalIntegrationsSettings {
  return {
    profiles: (settings?.profiles ?? []).map((profile, index) =>
      normalizeExternalIntegrationProfile(profile, index + 1),
    ),
    available_auth_methods: normalizeMethodDescriptors(
      settings?.available_auth_methods,
    ),
    available_resolvers: normalizeMethodDescriptors(settings?.available_resolvers),
  };
}

function normalizeMethodDescriptors(
  descriptors?: readonly ExternalIntegrationMethodDescriptor[] | null,
) {
  return (descriptors ?? []).map((descriptor) => ({
    name: descriptor.name.trim(),
    description: descriptor.description.trim(),
  }));
}

export function findExternalIntegrationMethodDescription(
  descriptors: readonly ExternalIntegrationMethodDescriptor[] | undefined,
  name: string,
) {
  return descriptors?.find((descriptor) => descriptor.name === name)?.description ?? "";
}
