"use client";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FormDialog } from "@/components/shared/form-dialog";
import type {
  Adapter,
  AdapterType,
  CreateAdapterRequest,
  UpdateAdapterRequest,
  SesConfig,
  GmailConfig,
} from "@/types/adapters";

interface AdapterFormCreateProps {
  mode?: "create";
  trigger: React.ReactNode;
  onSubmit: (data: CreateAdapterRequest) => Promise<void>;
  title?: string;
}

interface AdapterFormEditProps {
  mode: "edit";
  adapter: Adapter;
  trigger: React.ReactNode;
  onSubmit: (data: UpdateAdapterRequest) => Promise<void>;
  title?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

type AdapterFormProps = AdapterFormCreateProps | AdapterFormEditProps;

export function AdapterForm(props: AdapterFormProps) {
  const isEdit = props.mode === "edit";
  const defaults = isEdit ? props.adapter : undefined;

  const [name, setName] = useState(defaults?.name ?? "");
  const [adapterType, setAdapterType] = useState<AdapterType>(
    defaults?.adapter_type ?? "ses"
  );
  const [rateLimit, setRateLimit] = useState(
    defaults?.rate_limit_per_second?.toString() ?? ""
  );
  const [isDefault, setIsDefault] = useState(defaults?.is_default ?? false);

  // SES fields
  const [sesRegion, setSesRegion] = useState(
    (isEdit && defaults?.config_meta?.region) || ""
  );
  const [sesAccessKey, setSesAccessKey] = useState("");
  const [sesSecretKey, setSesSecretKey] = useState("");

  // Gmail fields (Service Account)
  const [gmailServiceAccountJSON, setGmailServiceAccountJSON] = useState("");
  const [gmailDelegateEmail, setGmailDelegateEmail] = useState(
    (isEdit && defaults?.config_meta?.delegate_email) || ""
  );

  function resetForm() {
    if (!isEdit) {
      setName("");
      setAdapterType("ses");
      setRateLimit("");
      setIsDefault(false);
    }
    setSesRegion(isEdit ? defaults?.config_meta?.region ?? "" : "");
    setSesAccessKey("");
    setSesSecretKey("");
    setGmailServiceAccountJSON("");
    setGmailDelegateEmail(isEdit ? defaults?.config_meta?.delegate_email ?? "" : "");
  }

  function buildConfig(): (SesConfig | GmailConfig) | undefined {
    if (adapterType === "ses") {
      // In edit mode, only send config if user filled something
      if (isEdit && !sesRegion && !sesAccessKey && !sesSecretKey) return undefined;
      return {
        region: sesRegion,
        access_key_id: sesAccessKey,
        secret_access_key: sesSecretKey,
      };
    } else {
      if (isEdit && !gmailServiceAccountJSON && !gmailDelegateEmail) return undefined;
      return {
        service_account_json: gmailServiceAccountJSON,
        delegate_email: gmailDelegateEmail,
      };
    }
  }

  async function handleSubmit() {
    if (isEdit) {
      const data: UpdateAdapterRequest = {};
      if (name !== defaults?.name) data.name = name;
      if (isDefault !== defaults?.is_default) data.is_default = isDefault;
      const rl = rateLimit ? Number(rateLimit) : undefined;
      if (rl !== defaults?.rate_limit_per_second) data.rate_limit_per_second = rl;
      const config = buildConfig();
      if (config) data.config = config;
      await props.onSubmit(data);
    } else {
      const config = buildConfig()!;
      await props.onSubmit({
        name,
        adapter_type: adapterType,
        config,
        is_default: isDefault,
        rate_limit_per_second: rateLimit ? Number(rateLimit) : undefined,
      });
    }
    resetForm();
  }

  return (
    <FormDialog
      trigger={props.trigger}
      title={props.title ?? (isEdit ? "Edit Adapter" : "New Adapter")}
      description={
        isEdit
          ? "Update adapter settings. Leave config fields empty to keep current values."
          : "Configure a new email adapter for sending."
      }
      submitLabel={isEdit ? "Update" : "Create"}
      onSubmit={handleSubmit}
      open={isEdit ? props.open : undefined}
      onOpenChange={isEdit ? props.onOpenChange : undefined}
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="adapter-name">Name</Label>
          <Input
            id="adapter-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="ses-production"
            className="font-mono"
            required={!isEdit}
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label>Type</Label>
          <Select
            value={adapterType}
            onValueChange={(v) => setAdapterType(v as AdapterType)}
            disabled={isEdit}
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ses">SES (Amazon)</SelectItem>
              <SelectItem value="gmail">Gmail (Google)</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="rate-limit">Rate Limit (per second)</Label>
          <Input
            id="rate-limit"
            type="number"
            value={rateLimit}
            onChange={(e) => setRateLimit(e.target.value)}
            placeholder="14"
            className="font-mono"
          />
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            role="switch"
            aria-checked={isDefault}
            onClick={() => setIsDefault(!isDefault)}
            className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
              isDefault ? "bg-primary" : "bg-muted"
            }`}
          >
            <span
              className={`pointer-events-none block h-5 w-5 rounded-full bg-white shadow-lg transition-transform ${
                isDefault ? "translate-x-5" : "translate-x-0"
              }`}
            />
          </button>
          <Label>Mark as default</Label>
        </div>

        <div className="border-t pt-4">
          <p className="text-xs font-medium text-muted-foreground mb-3">
            {adapterType === "ses" ? "AWS SES Configuration" : "Gmail Configuration"}
          </p>

          {adapterType === "ses" ? (
            <div className="flex flex-col gap-3">
              <div className="flex flex-col gap-2">
                <Label htmlFor="ses-region">Region</Label>
                <Input
                  id="ses-region"
                  value={sesRegion}
                  onChange={(e) => setSesRegion(e.target.value)}
                  placeholder="us-east-1"
                  className="font-mono"
                  required={!isEdit}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="ses-access-key">Access Key ID</Label>
                <Input
                  id="ses-access-key"
                  value={sesAccessKey}
                  onChange={(e) => setSesAccessKey(e.target.value)}
                  placeholder="AKIAIOSFODNN7EXAMPLE"
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="ses-secret-key">Secret Access Key</Label>
                <Input
                  id="ses-secret-key"
                  type="password"
                  value={sesSecretKey}
                  onChange={(e) => setSesSecretKey(e.target.value)}
                  placeholder="••••••••"
                  className="font-mono"
                />
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              <div className="flex flex-col gap-2">
                <Label htmlFor="gmail-sa-json">Service Account JSON Key</Label>
                {isEdit && !gmailServiceAccountJSON && (
                  <div className="rounded-md border border-dashed border-blue-500/40 bg-blue-500/5 px-3 py-2 text-xs text-blue-400">
                    JSON key is stored encrypted. Leave empty to keep the current key, or paste a new one to replace it.
                  </div>
                )}
                <textarea
                  id="gmail-sa-json"
                  value={gmailServiceAccountJSON}
                  onChange={(e) => setGmailServiceAccountJSON(e.target.value)}
                  placeholder={isEdit ? "Paste new JSON key to replace current..." : "Paste the full JSON key from Google Cloud Console"}
                  rows={isEdit && !gmailServiceAccountJSON ? 2 : 5}
                  required={!isEdit}
                  className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-xs ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono"
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="gmail-delegate">Delegate Email</Label>
                <Input
                  id="gmail-delegate"
                  type="email"
                  value={gmailDelegateEmail}
                  onChange={(e) => setGmailDelegateEmail(e.target.value)}
                  placeholder="send@company.com"
                  className="font-mono"
                  required={!isEdit}
                />
              </div>
            </div>
          )}
        </div>
      </div>
    </FormDialog>
  );
}
