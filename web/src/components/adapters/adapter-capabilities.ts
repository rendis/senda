import type { AdapterIdentity, AdapterType } from "@/types/adapters";

export function getAdapterCapabilities(type: AdapterType) {
  return {
    supportsIdentitySync: type === "ses" || type === "gmail",
    supportsProvisioning: type === "ses",
    supportsSenderSharing: type === "ses" || type === "smtp",
    supportsAdapterSharing: type === "gmail",
    usesSenderIdentity: type === "ses" || type === "smtp",
  };
}

export function isVerifiedEmailIdentity(
  identity: Pick<AdapterIdentity, "identity_type" | "status">,
) {
  return identity.identity_type === "email" && identity.status === "verified";
}

export function canTestSendAdapter(
  type: AdapterType,
  identities: Pick<AdapterIdentity, "identity_type" | "status">[],
) {
  return !getAdapterCapabilities(type).usesSenderIdentity ||
    identities.some(isVerifiedEmailIdentity);
}

export function shouldIncludeTestSendFrom(type: AdapterType, from?: string) {
  return getAdapterCapabilities(type).usesSenderIdentity && !!from;
}
