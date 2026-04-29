import type { AdapterType } from "@/types/adapters";

export const SENDER_DEFAULT_VALUE = "__default__";

type SenderIdentityAdapter = {
  adapter_type: AdapterType;
  is_shared?: boolean;
};

export function adapterUsesSenderIdentity(adapter: SenderIdentityAdapter | undefined) {
  return adapter?.adapter_type === "ses" || adapter?.adapter_type === "smtp";
}

export function requiresExplicitSenderIdentity(adapter: SenderIdentityAdapter | undefined) {
  return adapterUsesSenderIdentity(adapter) && adapter?.is_shared === true;
}

export function resolveTemplateTypeSenderIdentityId(
  value: string | undefined,
  options: { clearWithEmptyString?: boolean } = {},
) {
  const normalized = value?.trim() ?? "";
  if (!normalized || normalized === SENDER_DEFAULT_VALUE) {
    return options.clearWithEmptyString ? "" : undefined;
  }
  return normalized;
}
