"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { Save, ShieldAlert } from "lucide-react";
import { toast } from "sonner";
import { useScope } from "@/hooks/use-scope";
import { useSettings, useUpdateSettings } from "@/hooks/use-settings";
import { SettingsSection } from "@/components/settings/settings-section";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import type { UpdateSettingsRequest } from "@/types/settings";

interface SettingsFormValues {
  max_retries: number;
  backoff_base_seconds: number;
  log_retention_days: number;
  bounce_threshold_percent: number;
  complaint_threshold_percent: number;
  recheck_interval_hours: number;
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
  const scope = useScope();
  const { data, isLoading, error } = useSettings();
  const updateSettings = useUpdateSettings();

  const { register, handleSubmit, reset } = useForm<SettingsFormValues>();

  useEffect(() => {
    if (data) {
      reset({
        max_retries: data.email_defaults.max_retries,
        backoff_base_seconds: data.email_defaults.backoff_base_seconds,
        log_retention_days: data.email_defaults.log_retention_days,
        bounce_threshold_percent: data.alerts.bounce_threshold_percent,
        complaint_threshold_percent: data.alerts.complaint_threshold_percent,
        recheck_interval_hours: data.domain.recheck_interval_hours,
      });
    }
  }, [data, reset]);

  // Guard: settings only at global scope
  if (scope.level !== "global") {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="Access denied"
        description="Settings are only available at the global scope for superadmin users."
      />
    );
  }

  if (isLoading) return <SettingsFormSkeleton />;

  if (error && !data) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="Failed to load settings"
        description="Could not retrieve system settings. You may not have superadmin permissions."
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
      domain: {
        recheck_interval_hours: Number(values.recheck_interval_hours),
      },
    };

    try {
      await updateSettings.mutateAsync(payload);
      toast.success("Settings saved successfully");
    } catch {
      toast.error("Failed to save settings");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-8">
      {/* OIDC Section — read only */}
      <SettingsSection title="OIDC Configuration">
        <div className="flex flex-col gap-3">
          <div className="space-y-1.5">
            <Label className="text-[13px] font-medium">Discovery URL</Label>
            <Input
              readOnly
              value={data?.oidc.discovery_url ?? ""}
              className="h-9 text-[13px] bg-muted/50 text-muted-foreground cursor-default"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[13px] font-medium">Client ID</Label>
            <Input
              readOnly
              value={data?.oidc.client_id ?? ""}
              className="h-9 text-[13px] bg-muted/50 text-muted-foreground cursor-default"
            />
          </div>
        </div>
      </SettingsSection>

      {/* Email Defaults Section */}
      <SettingsSection title="Email Defaults">
        <div className="flex gap-4">
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">Max Retries</Label>
            <Input
              type="number"
              {...register("max_retries", { valueAsNumber: true })}
              className="h-9 text-[13px]"
            />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">
              Backoff Base (sec)
            </Label>
            <Input
              type="number"
              {...register("backoff_base_seconds", { valueAsNumber: true })}
              className="h-9 text-[13px]"
            />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">
              Log Retention (days)
            </Label>
            <Input
              type="number"
              {...register("log_retention_days", { valueAsNumber: true })}
              className="h-9 text-[13px]"
            />
          </div>
        </div>
      </SettingsSection>

      {/* Alerts Section */}
      <SettingsSection title="Alerts">
        <div className="flex gap-4">
          <div className="flex-1 space-y-1.5">
            <Label className="text-[13px] font-medium">
              Bounce Threshold %
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
              Complaint Threshold %
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

      {/* Domain Section */}
      <SettingsSection title="Domain">
        <div className="w-[300px] space-y-1.5">
          <Label className="text-[13px] font-medium">
            Recheck Interval (hours)
          </Label>
          <Input
            type="number"
            {...register("recheck_interval_hours", { valueAsNumber: true })}
            className="h-9 text-[13px]"
          />
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
          {updateSettings.isPending ? "Saving..." : "Save Changes"}
        </Button>
      </div>
    </form>
  );
}
