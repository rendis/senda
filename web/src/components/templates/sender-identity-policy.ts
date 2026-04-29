import type { AdapterType } from "@/types/adapters";

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
