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
  AdapterType,
  CreateAdapterRequest,
  SesConfig,
  GmailConfig,
} from "@/types/adapters";

interface AdapterFormProps {
  trigger: React.ReactNode;
  onSubmit: (data: CreateAdapterRequest) => Promise<void>;
  title?: string;
}

export function AdapterForm({
  trigger,
  onSubmit,
  title = "New Adapter",
}: AdapterFormProps) {
  const [name, setName] = useState("");
  const [adapterType, setAdapterType] = useState<AdapterType>("ses");
  const [rateLimit, setRateLimit] = useState("");
  const [isDefault, setIsDefault] = useState(false);

  // SES fields
  const [sesRegion, setSesRegion] = useState("");
  const [sesAccessKey, setSesAccessKey] = useState("");
  const [sesSecretKey, setSesSecretKey] = useState("");

  // Gmail fields
  const [gmailClientId, setGmailClientId] = useState("");
  const [gmailClientSecret, setGmailClientSecret] = useState("");
  const [gmailRefreshToken, setGmailRefreshToken] = useState("");
  const [gmailDelegateEmail, setGmailDelegateEmail] = useState("");

  function resetForm() {
    setName("");
    setAdapterType("ses");
    setRateLimit("");
    setIsDefault(false);
    setSesRegion("");
    setSesAccessKey("");
    setSesSecretKey("");
    setGmailClientId("");
    setGmailClientSecret("");
    setGmailRefreshToken("");
    setGmailDelegateEmail("");
  }

  async function handleSubmit() {
    let config: SesConfig | GmailConfig;

    if (adapterType === "ses") {
      config = {
        region: sesRegion,
        access_key_id: sesAccessKey,
        secret_access_key: sesSecretKey,
      };
    } else {
      config = {
        oauth_client_id: gmailClientId,
        oauth_client_secret: gmailClientSecret,
        refresh_token: gmailRefreshToken,
        delegate_email: gmailDelegateEmail,
      };
    }

    await onSubmit({
      name,
      adapter_type: adapterType,
      config,
      is_default: isDefault,
      rate_limit_per_second: rateLimit ? Number(rateLimit) : undefined,
    });
    resetForm();
  }

  return (
    <FormDialog
      trigger={trigger}
      title={title}
      description="Configure a new email adapter for sending."
      submitLabel="Create"
      onSubmit={handleSubmit}
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
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label>Type</Label>
          <Select
            value={adapterType}
            onValueChange={(v) => setAdapterType(v as AdapterType)}
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
                <Label htmlFor="gmail-client-id">OAuth Client ID</Label>
                <Input
                  id="gmail-client-id"
                  value={gmailClientId}
                  onChange={(e) => setGmailClientId(e.target.value)}
                  placeholder="123456789.apps.googleusercontent.com"
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="gmail-client-secret">Client Secret</Label>
                <Input
                  id="gmail-client-secret"
                  type="password"
                  value={gmailClientSecret}
                  onChange={(e) => setGmailClientSecret(e.target.value)}
                  placeholder="••••••••"
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="gmail-refresh-token">Refresh Token</Label>
                <Input
                  id="gmail-refresh-token"
                  type="password"
                  value={gmailRefreshToken}
                  onChange={(e) => setGmailRefreshToken(e.target.value)}
                  placeholder="••••••••"
                  className="font-mono"
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
                />
              </div>
            </div>
          )}
        </div>
      </div>
    </FormDialog>
  );
}
