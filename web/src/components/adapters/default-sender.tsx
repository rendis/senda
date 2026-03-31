"use client";

import { useIdentityList } from "@/hooks/use-identities";
import type { Adapter } from "@/types/adapters";

export function DefaultSender({
  adapter,
  scopedPath,
}: {
  adapter: Adapter;
  scopedPath: string;
}) {
  const { data: identities } = useIdentityList(scopedPath, adapter.id);

  // Find default identity
  const defaultIdentity = identities?.find((i) => i.is_default);

  // Fallback to config_meta
  const fallback = adapter.config_meta?.delegate_email;

  const value = defaultIdentity?.identity ?? fallback ?? "\u2014";

  return (
    <span className="font-mono text-[13px] text-muted-foreground truncate max-w-[200px] inline-block">
      {value}
    </span>
  );
}
