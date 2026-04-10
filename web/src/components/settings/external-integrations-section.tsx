"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  ExternalIntegrationMethodDescriptor,
  ExternalIntegrationProfile,
  ExternalIntegrationsSettings,
} from "@/types/settings";
import {
  EXTERNAL_INTEGRATION_CAPABILITY_KEYS,
  createEmptyExternalIntegrationProfile,
  findExternalIntegrationMethodDescription,
  normalizeExternalIntegrationProfile,
  parseDelimitedEntries,
  stringifyDelimitedEntries,
} from "@/lib/external-integration-profiles";

interface ExternalIntegrationsSectionProps {
  value: ExternalIntegrationsSettings;
  onChange: (next: ExternalIntegrationsSettings) => void;
}

export function ExternalIntegrationsSection({
  value,
  onChange,
}: ExternalIntegrationsSectionProps) {
  const t = useTranslations("settingsPage.externalIntegrations");

  const profiles = useMemo(
    () =>
      (value.profiles ?? []).map((profile, index) =>
        normalizeExternalIntegrationProfile(profile, index + 1),
      ),
    [value.profiles],
  );

  const availableAuthMethods = value.available_auth_methods ?? [];
  const availableResolvers = value.available_resolvers ?? [];

  const updateProfiles = (nextProfiles: ExternalIntegrationProfile[]) => {
    onChange({
      ...value,
      profiles: nextProfiles,
    });
  };

  const addProfile = () => {
    updateProfiles([...profiles, createEmptyExternalIntegrationProfile(profiles.length + 1)]);
  };

  const removeProfile = (index: number) => {
    updateProfiles(profiles.filter((_, currentIndex) => currentIndex !== index));
  };

  const updateProfile = (
    index: number,
    updater: (profile: ExternalIntegrationProfile) => ExternalIntegrationProfile,
  ) => {
    updateProfiles(
      profiles.map((profile, currentIndex) =>
        currentIndex === index ? updater(profile) : profile,
      ),
    );
  };

  return (
    <section className="rounded-lg border bg-card p-6 space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1.5">
          <h2 className="text-base font-semibold tracking-[-0.02em]">
            {t("title")}
          </h2>
          <p className="text-sm text-muted-foreground max-w-3xl">
            {t("description")}
          </p>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={addProfile}>
          <Plus className="h-4 w-4" />
          {t("addProfile")}
        </Button>
      </div>

      {profiles.length > 0 ? (
        <div className="space-y-4">
          {profiles.map((profile, index) => (
            <ProfileCard
              key={`${profile.slug}-${index}`}
              profile={profile}
              index={index}
              authMethods={availableAuthMethods}
              resolvers={availableResolvers}
              onRemove={() => removeProfile(index)}
              onChange={(next) => updateProfile(index, () => next)}
            />
          ))}
        </div>
      ) : (
        <div className="rounded-lg border border-dashed bg-background px-4 py-8 text-center">
          <h3 className="text-sm font-medium">{t("empty.title")}</h3>
          <p className="mt-1 text-sm text-muted-foreground">{t("empty.description")}</p>
          <Button type="button" variant="outline" size="sm" className="mt-4" onClick={addProfile}>
            <Plus className="h-4 w-4" />
            {t("addProfile")}
          </Button>
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <DescriptorList
          title={t("registeredAuthMethods")}
          emptyLabel={t("noAuthMethods")}
          items={availableAuthMethods}
        />
        <DescriptorList
          title={t("registeredResolvers")}
          emptyLabel={t("noResolvers")}
          items={availableResolvers}
        />
      </div>
    </section>
  );
}

function ProfileCard({
  profile,
  index,
  authMethods,
  resolvers,
  onRemove,
  onChange,
}: {
  profile: ExternalIntegrationProfile;
  index: number;
  authMethods: ExternalIntegrationMethodDescriptor[];
  resolvers: ExternalIntegrationMethodDescriptor[];
  onRemove: () => void;
  onChange: (next: ExternalIntegrationProfile) => void;
}) {
  const t = useTranslations("settingsPage.externalIntegrations");
  const selectedAuthMethodDescription = findExternalIntegrationMethodDescription(
    authMethods,
    profile.auth_method_name,
  );
  const selectedResolverDescription = findExternalIntegrationMethodDescription(
    resolvers,
    profile.resolver_name,
  );

  return (
    <article className="rounded-lg border bg-background p-4 shadow-sm space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold">{t("profileTitle", { index: index + 1 })}</h3>
            <Badge variant={profile.enabled ? "default" : "outline"}>
              {profile.enabled ? t("enabled") : t("disabled")}
            </Badge>
          </div>
          <p className="text-xs text-muted-foreground">{t("profileDescription")}</p>
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={onRemove}>
          <Trash2 className="h-4 w-4" />
          {t("removeProfile")}
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Field
          label={t("slugLabel")}
          value={profile.slug}
          onChange={(slug) => onChange({ ...profile, slug })}
          placeholder="external-builder"
        />
        <Field
          label={t("nameLabel")}
          value={profile.name}
          onChange={(name) => onChange({ ...profile, name })}
          placeholder="External builder"
        />
      </div>

      <div className="space-y-1.5">
        <Label className="text-sm font-medium">{t("descriptionLabel")}</Label>
        <textarea
          value={profile.description}
          onChange={(event) => onChange({ ...profile, description: event.target.value })}
          rows={3}
          className="min-h-20 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
          placeholder={t("descriptionPlaceholder")}
        />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <MethodSelector
          label={t("authMethodLabel")}
          placeholder={t("authMethodPlaceholder")}
          value={profile.auth_method_name}
          options={authMethods}
          onChange={(auth_method_name) => onChange({ ...profile, auth_method_name })}
          description={
            selectedAuthMethodDescription || t("methodFallbackDescription")
          }
        />
        <MethodSelector
          label={t("resolverLabel")}
          placeholder={t("resolverPlaceholder")}
          value={profile.resolver_name}
          options={resolvers}
          onChange={(resolver_name) => onChange({ ...profile, resolver_name })}
          description={
            selectedResolverDescription || t("methodFallbackDescription")
          }
        />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <ListField
          label={t("allowedOriginsLabel")}
          value={profile.allowed_origins}
          placeholder={t("allowedOriginsPlaceholder")}
          onChange={(next) =>
            onChange({ ...profile, allowed_origins: parseDelimitedEntries(next) })
          }
        />
        <ListField
          label={t("allowedHeadersLabel")}
          value={profile.allowed_headers}
          placeholder={t("allowedHeadersPlaceholder")}
          onChange={(next) =>
            onChange({ ...profile, allowed_headers: parseDelimitedEntries(next) })
          }
        />
        <ListField
          label={t("requiredHeadersLabel")}
          value={profile.required_headers}
          placeholder={t("requiredHeadersPlaceholder")}
          onChange={(next) =>
            onChange({ ...profile, required_headers: parseDelimitedEntries(next) })
          }
        />
      </div>

      <div className="space-y-3">
        <Label className="text-sm font-medium">{t("capabilitiesLabel")}</Label>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {EXTERNAL_INTEGRATION_CAPABILITY_KEYS.map((key) => (
            <CapabilityToggle
              key={key}
              capability={key}
              checked={profile.capabilities[key]}
              onChange={(checked) =>
                onChange({
                  ...profile,
                  capabilities: {
                    ...profile.capabilities,
                    [key]: checked,
                  },
                })
              }
            />
          ))}
        </div>
      </div>
    </article>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="text-sm font-medium">{label}</Label>
      <Input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-9 text-sm"
      />
    </div>
  );
}

function ListField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string[];
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="text-sm font-medium">{label}</Label>
      <textarea
        value={stringifyDelimitedEntries(value)}
        onChange={(event) => onChange(event.target.value)}
        rows={5}
        className="min-h-32 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
        placeholder={placeholder}
      />
      <p className="text-xs text-muted-foreground">
        {value.length} {value.length === 1 ? "entry" : "entries"}
      </p>
    </div>
  );
}

function MethodSelector({
  label,
  placeholder,
  value,
  options,
  description,
  onChange,
}: {
  label: string;
  placeholder: string;
  value: string;
  options: ExternalIntegrationMethodDescriptor[];
  description: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="text-sm font-medium">{label}</Label>
      {options.length > 0 ? (
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger className="h-9 w-full">
            <SelectValue placeholder={placeholder} />
          </SelectTrigger>
          <SelectContent>
            {options.map((option) => (
              <SelectItem key={option.name} value={option.name}>
                {option.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : (
        <Input
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder={placeholder}
          className="h-9 text-sm"
        />
      )}
      <p className="text-xs text-muted-foreground">{description}</p>
    </div>
  );
}

function CapabilityToggle({
  capability,
  checked,
  onChange,
}: {
  capability: (typeof EXTERNAL_INTEGRATION_CAPABILITY_KEYS)[number];
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  const t = useTranslations("settingsPage.externalIntegrations");

  return (
    <label className="flex items-start gap-3 rounded-lg border bg-muted/20 p-3">
      <input
        type="checkbox"
        className="mt-0.5 h-4 w-4 rounded border-border"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className="space-y-1">
        <span className="block text-sm font-medium text-foreground">
          {t(`capabilityLabels.${capability}`)}
        </span>
        <span className="block text-xs text-muted-foreground">
          {t(`capabilityDescriptions.${capability}`)}
        </span>
      </span>
    </label>
  );
}

function DescriptorList({
  title,
  emptyLabel,
  items,
}: {
  title: string;
  emptyLabel: string;
  items: ExternalIntegrationMethodDescriptor[];
}) {
  return (
    <div className="rounded-lg border bg-background p-4 space-y-3">
      <h3 className="text-sm font-semibold">{title}</h3>
      {items.length > 0 ? (
        <div className="space-y-2">
          {items.map((item) => (
            <div key={item.name} className="rounded-md border bg-card p-3">
              <p className="text-sm font-medium">{item.name}</p>
              <p className="text-xs text-muted-foreground">{item.description}</p>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">{emptyLabel}</p>
      )}
    </div>
  );
}
