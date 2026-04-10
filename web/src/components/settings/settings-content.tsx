"use client";

import { useEffect, useState } from "react";
import { useMinimumLoading } from "@/hooks/use-minimum-loading";
import { type UseFormRegisterReturn, useForm } from "react-hook-form";
import { Save, ShieldAlert } from "lucide-react";
import { toast } from "sonner";
import { useScope } from "@/hooks/use-scope";
import {
  canManageSystemWorkspacePolicies,
  canShowGlobalSettings,
} from "@/lib/workspace-resource-policies";
import {
  useResolvedWorkspacePolicies,
  useSettings,
  useUpdateSettings,
  useUpdateWorkspacePolicies,
} from "@/hooks/use-settings";
import { ExternalIntegrationsSection } from "@/components/settings/external-integrations-section";
import { SettingsSection } from "@/components/settings/settings-section";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { ThemeSelector } from "@/components/shared/theme-selector";
import type {
  UpdateSettingsRequest,
  UpdateWorkspacePoliciesRequest,
  ExternalIntegrationsSettings,
} from "@/types/settings";
import { normalizeExternalIntegrationSettings } from "@/lib/external-integration-profiles";
import { useTranslations } from "next-intl";

interface SettingsFormValues {
  max_retries: number;
  backoff_base_seconds: number;
  log_retention_days: number;
  bounce_threshold_percent: number;
  complaint_threshold_percent: number;
}

interface WorkspacePolicyFormValues {
  allow_workspace_local_templates: boolean;
  allow_workspace_inherited_template_forks: boolean;
  allow_workspace_local_injectors: boolean;
}

function SettingsFormSkeleton() {
  return (
    <div className="flex flex-col gap-8">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="rounded-lg border bg-card p-6 space-y-4">
          <Skeleton className="h-5 w-40" />
          <div className="flex gap-4">
            <div className="flex-1 space-y-2">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-9 w-full" />
            </div>
            <div className="flex-1 space-y-2">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-9 w-full" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export function SettingsContent() {
  const tSettings = useTranslations("settingsPage");
  const tTheme = useTranslations("theme");
  const tAppearance = useTranslations("appearance");
  const scope = useScope();
  const showGlobalSettings = canShowGlobalSettings(scope);
  const showSystemPolicies = canManageSystemWorkspacePolicies(scope);

  if (!showGlobalSettings && !showSystemPolicies) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title={tSettings("accessDenied.title")}
        description={tSettings("accessDenied.description")}
      />
    );
  }

  if (showGlobalSettings) {
    return (
      <GlobalSettingsContent
        appearanceTitle={tAppearance("title")}
        appearanceDescription={tAppearance("description")}
        themeLabel={tTheme("label")}
      />
    );
  }

  return (
    <WorkspacePolicySettingsContent
      appearanceTitle={tAppearance("title")}
      appearanceDescription={tAppearance("description")}
      themeLabel={tTheme("label")}
    />
  );
}

function GlobalSettingsContent({
  appearanceTitle,
  appearanceDescription,
  themeLabel,
}: {
  appearanceTitle: string;
  appearanceDescription: string;
  themeLabel: string;
}) {
  const tSettings = useTranslations("settingsPage");
  const { data, isLoading: rawLoading, error } = useSettings();
  const isLoading = useMinimumLoading(rawLoading);
  const updateSettings = useUpdateSettings();
  const { register, handleSubmit, reset } = useForm<SettingsFormValues>();
  const [externalIntegrations, setExternalIntegrations] =
    useState<ExternalIntegrationsSettings>(
      normalizeExternalIntegrationSettings(data?.external_integrations),
    );

  useEffect(() => {
    if (data) {
      reset({
        max_retries: data.email_defaults.max_retries,
        backoff_base_seconds: data.email_defaults.backoff_base_seconds,
        log_retention_days: data.email_defaults.log_retention_days,
        bounce_threshold_percent: data.alerts.bounce_threshold_percent,
        complaint_threshold_percent: data.alerts.complaint_threshold_percent,
      });
      // eslint-disable-next-line react-hooks/set-state-in-effect -- hydrate local external profile form state from server data
      setExternalIntegrations(
        normalizeExternalIntegrationSettings(data.external_integrations),
      );
    }
  }, [data, reset]);

  if (isLoading) return <SettingsFormSkeleton />;

  if (error && !data) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title={tSettings("global.loadErrorTitle")}
        description={tSettings("global.loadErrorDescription")}
      />
    );
  }

  const onSubmit = async (values: SettingsFormValues) => {
    const payload: UpdateSettingsRequest = {
      email_defaults: {
        max_retries: Number(values.max_retries),
        backoff_base_seconds: Number(values.backoff_base_seconds),
        log_retention_days: Number(values.log_retention_days),
      },
      alerts: {
        bounce_threshold_percent: Number(values.bounce_threshold_percent),
        complaint_threshold_percent: Number(values.complaint_threshold_percent),
      },
      external_integrations: {
        profiles: externalIntegrations.profiles,
      },
    };

    try {
      await updateSettings.mutateAsync(payload);
      toast.success(tSettings("global.saveSuccess"));
    } catch {
      toast.error(tSettings("global.saveError"));
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-8 animate-in fade-in duration-300">
      {/* OIDC Section — read only */}
      <SettingsSection title={tSettings("global.oidcTitle")}>
        <div className="flex flex-col gap-3">
          <div className="space-y-1.5">
            <Label className="text-[13px] font-medium">{tSettings("global.discoveryUrl")}</Label>
            <Input
              readOnly
              value={data?.oidc.discovery_url ?? ""}
              className="h-9 text-[13px] bg-muted/50 text-muted-foreground cursor-default"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[13px] font-medium">{tSettings("global.clientId")}</Label>
            <Input
              readOnly
              value={data?.oidc.client_id ?? ""}
              className="h-9 text-[13px] bg-muted/50 text-muted-foreground cursor-default"
            />
          </div>
        </div>
      </SettingsSection>

      {/* Email Defaults Section */}
      <SettingsSection title={tSettings("global.emailDefaultsTitle")}>
        <div className="flex gap-4">
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">{tSettings("global.maxRetries")}</Label>
            <Input
              type="number"
              {...register("max_retries", { valueAsNumber: true })}
              className="h-9 text-[13px]"
            />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">
              {tSettings("global.backoffBaseSeconds")}
            </Label>
            <Input
              type="number"
              {...register("backoff_base_seconds", { valueAsNumber: true })}
              className="h-9 text-[13px]"
            />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">
              {tSettings("global.logRetentionDays")}
            </Label>
            <Input
              type="number"
              {...register("log_retention_days", { valueAsNumber: true })}
              className="h-9 text-[13px]"
            />
          </div>
        </div>
      </SettingsSection>

      <ExternalIntegrationsSection
        value={externalIntegrations}
        onChange={setExternalIntegrations}
      />

      {/* Alerts Section */}
      <SettingsSection title={tSettings("global.alertsTitle")}>
        <div className="flex gap-4">
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">
              {tSettings("global.bounceThreshold")}
            </Label>
            <Input
              type="number"
              step="0.1"
              {...register("bounce_threshold_percent", {
                valueAsNumber: true,
              })}
              className="h-9 text-[13px]"
            />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">
              {tSettings("global.complaintThreshold")}
            </Label>
            <Input
              type="number"
              step="0.01"
              {...register("complaint_threshold_percent", {
                valueAsNumber: true,
              })}
              className="h-9 text-[13px]"
            />
          </div>
        </div>
      </SettingsSection>

      <SettingsSection title={appearanceTitle}>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-[13px] font-medium">{themeLabel}</Label>
            <p className="text-sm text-muted-foreground">
              {appearanceDescription}
            </p>
          </div>
          <ThemeSelector variant="inline" />
        </div>
      </SettingsSection>

      {/* Save Button */}
      <div className="flex justify-end">
        <Button
          type="submit"
          className="h-9 gap-2"
          disabled={updateSettings.isPending}
        >
          <Save className="h-4 w-4" />
          {updateSettings.isPending
            ? tSettings("saving")
            : tSettings("saveChanges")}
        </Button>
      </div>
    </form>
  );
}

function WorkspacePolicySettingsContent({
  appearanceTitle,
  appearanceDescription,
  themeLabel,
}: {
  appearanceTitle: string;
  appearanceDescription: string;
  themeLabel: string;
}) {
  const tSettings = useTranslations("settingsPage");
  const scope = useScope();
  const { data, isLoading: rawLoading, error } = useResolvedWorkspacePolicies(scope);
  const isLoading = useMinimumLoading(rawLoading);
  const updatePolicies = useUpdateWorkspacePolicies(scope.tenantCode);
  const { register, getValues, reset } = useForm<WorkspacePolicyFormValues>();

  useEffect(() => {
    if (!data) {
      return;
    }
    reset({
      allow_workspace_local_templates: data.allow_workspace_local_templates,
      allow_workspace_inherited_template_forks:
        data.allow_workspace_inherited_template_forks,
      allow_workspace_local_injectors: data.allow_workspace_local_injectors,
    });
  }, [data, reset]);

  if (isLoading) return <SettingsFormSkeleton />;

  if (error && !data) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title={tSettings("workspace.loadErrorTitle")}
        description={tSettings("workspace.loadErrorDescription")}
      />
    );
  }

  const savePolicies = async (values: WorkspacePolicyFormValues) => {
    const payload: UpdateWorkspacePoliciesRequest = {
      allow_workspace_local_templates: Boolean(
        values.allow_workspace_local_templates,
      ),
      allow_workspace_inherited_template_forks: Boolean(
        values.allow_workspace_inherited_template_forks,
      ),
      allow_workspace_local_injectors: Boolean(
        values.allow_workspace_local_injectors,
      ),
    };

    try {
      await updatePolicies.mutateAsync(payload);
      toast.success(tSettings("workspace.saveSuccess"));
    } catch {
      toast.error(tSettings("workspace.saveError"));
    }
  };

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void savePolicies(getValues());
      }}
      className="flex flex-col gap-8 animate-in fade-in duration-300"
    >
      <SettingsSection title={tSettings("workspace.workspaceDefaultsPolicy")}>
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            {tSettings("workspace.workspaceDefaultsDescription")}
          </p>

          <PolicyToggleRow
            title={tSettings("workspace.allowLocalTemplatesTitle")}
            description={tSettings("workspace.allowLocalTemplatesDescription")}
            inputProps={register("allow_workspace_local_templates")}
          />

          <PolicyToggleRow
            title={tSettings("workspace.allowInheritedForksTitle")}
            description={tSettings("workspace.allowInheritedForksDescription")}
            inputProps={register("allow_workspace_inherited_template_forks")}
          />

          <PolicyToggleRow
            title={tSettings("workspace.allowLocalInjectorsTitle")}
            description={tSettings("workspace.allowLocalInjectorsDescription")}
            inputProps={register("allow_workspace_local_injectors")}
          />
        </div>
      </SettingsSection>

      <SettingsSection title={appearanceTitle}>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-[13px] font-medium">{themeLabel}</Label>
            <p className="text-sm text-muted-foreground">
              {appearanceDescription}
            </p>
          </div>
          <ThemeSelector variant="inline" />
        </div>
      </SettingsSection>

      <div className="flex justify-end">
        <Button
          type="button"
          className="h-9 gap-2"
          disabled={updatePolicies.isPending}
          onClick={() => void savePolicies(getValues())}
        >
          <Save className="h-4 w-4" />
          {updatePolicies.isPending
            ? tSettings("saving")
            : tSettings("saveChanges")}
        </Button>
      </div>
    </form>
  );
}

function PolicyToggleRow({
  title,
  description,
  inputProps,
}: {
  title: string;
  description: string;
  inputProps: UseFormRegisterReturn;
}) {
  return (
    <label className="flex items-start gap-3 rounded-lg border bg-muted/20 p-4">
      <input
        type="checkbox"
        className="mt-0.5 h-4 w-4 rounded border-border"
        {...inputProps}
      />
      <span className="space-y-1">
        <span className="block text-sm font-medium text-foreground">
          {title}
        </span>
        <span className="block text-sm text-muted-foreground">
          {description}
        </span>
      </span>
    </label>
  );
}
