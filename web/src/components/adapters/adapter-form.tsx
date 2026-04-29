"use client";

import { useState } from "react";
import { Check, ChevronsUpDown, ShieldAlert, ShieldCheck, Rocket } from "lucide-react";
import { useScopedPath } from "@/hooks/use-scope";
import { useValidateSES, type SESValidationResult } from "@/hooks/use-adapters";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { FormDialog } from "@/components/shared/form-dialog";
import type {
  Adapter,
  AdapterType,
  CreateAdapterRequest,
  UpdateAdapterRequest,
  SesConfig,
  GmailConfig,
  SmtpConfig,
  SmtpTLSMode,
} from "@/types/adapters";

const SES_REGIONS = [
  { value: "us-east-1", label: "US East (N. Virginia)" },
  { value: "us-east-2", label: "US East (Ohio)" },
  { value: "us-west-1", label: "US West (N. California)" },
  { value: "us-west-2", label: "US West (Oregon)" },
  { value: "eu-west-1", label: "Europe (Ireland)" },
  { value: "eu-west-2", label: "Europe (London)" },
  { value: "eu-west-3", label: "Europe (Paris)" },
  { value: "eu-central-1", label: "Europe (Frankfurt)" },
  { value: "eu-south-1", label: "Europe (Milan)" },
  { value: "eu-north-1", label: "Europe (Stockholm)" },
  { value: "ap-south-1", label: "Asia Pacific (Mumbai)" },
  { value: "ap-southeast-1", label: "Asia Pacific (Singapore)" },
  { value: "ap-southeast-2", label: "Asia Pacific (Sydney)" },
  { value: "ap-northeast-1", label: "Asia Pacific (Tokyo)" },
  { value: "ap-northeast-2", label: "Asia Pacific (Seoul)" },
  { value: "ap-northeast-3", label: "Asia Pacific (Osaka)" },
  { value: "ca-central-1", label: "Canada (Central)" },
  { value: "sa-east-1", label: "South America (S\u00e3o Paulo)" },
  { value: "me-south-1", label: "Middle East (Bahrain)" },
  { value: "af-south-1", label: "Africa (Cape Town)" },
  { value: "il-central-1", label: "Israel (Tel Aviv)" },
];

const ADAPTER_DEFAULTS: Record<AdapterType, { rateLimit: number; region?: string; description: string; docsUrl?: string }> = {
  ses: {
    rateLimit: 14,
    region: "us-east-1",
    description: "AWS SES production default is 14/sec. Sandbox accounts are limited to 1/sec.",
    docsUrl: "https://docs.aws.amazon.com/ses/latest/dg/quotas.html",
  },
  gmail: {
    rateLimit: 2,
    description: "Gmail API with Workspace delegation supports ~2 emails/sec per delegated user.",
    docsUrl: "https://developers.google.com/gmail/api/reference/quota",
  },
  smtp: {
    rateLimit: 10,
    description: "SMTP relay rate limits depend on your provider or internal relay policy.",
  },
};

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
  const scopedPath = useScopedPath();
  const validateSES = useValidateSES(scopedPath);
  const [sesValidation, setSesValidation] = useState<SESValidationResult | null>(null);

  const [name, setName] = useState(defaults?.name ?? "");
  const [adapterType, setAdapterType] = useState<AdapterType>(
    defaults?.adapter_type ?? "ses"
  );
  const [rateLimit, setRateLimit] = useState(
    defaults?.rate_limit_per_second?.toString() ?? ADAPTER_DEFAULTS["ses"].rateLimit.toString()
  );
  const [isDefault, setIsDefault] = useState(defaults?.is_default ?? false);

  // SES fields
  const [sesRegion, setSesRegion] = useState(
    (isEdit && defaults?.config_meta?.region) || ADAPTER_DEFAULTS["ses"].region || ""
  );
  const [sesAccessKey, setSesAccessKey] = useState("");
  const [sesSecretKey, setSesSecretKey] = useState("");

  // Gmail fields (Service Account)
  const [gmailServiceAccountJSON, setGmailServiceAccountJSON] = useState("");
  const [gmailDelegateEmail, setGmailDelegateEmail] = useState(
    (isEdit && defaults?.config_meta?.delegate_email) || ""
  );

  // SMTP fields
  const [smtpHost, setSmtpHost] = useState((isEdit && defaults?.config_meta?.host) || "");
  const [smtpPort, setSmtpPort] = useState((isEdit && defaults?.config_meta?.port) || "587");
  const [smtpTLSMode, setSmtpTLSMode] = useState<SmtpTLSMode>(
    ((isEdit && defaults?.config_meta?.tls_mode) as SmtpTLSMode | undefined) ?? "starttls"
  );
  const [smtpAuthMode, setSmtpAuthMode] = useState<"plain" | "login">(
    ((isEdit && defaults?.config_meta?.auth_mode) as "plain" | "login" | undefined) ?? "plain"
  );
  const [smtpUsername, setSmtpUsername] = useState("");
  const [smtpPassword, setSmtpPassword] = useState("");

  function resetForm() {
    if (!isEdit) {
      setName("");
      setAdapterType("ses");
      setRateLimit(ADAPTER_DEFAULTS["ses"].rateLimit.toString());
      setIsDefault(false);
    }
    setSesRegion(isEdit ? defaults?.config_meta?.region ?? "" : ADAPTER_DEFAULTS["ses"].region || "");
    setSesAccessKey("");
    setSesSecretKey("");
    setGmailServiceAccountJSON("");
    setGmailDelegateEmail(isEdit ? defaults?.config_meta?.delegate_email ?? "" : "");
    setSmtpHost(isEdit ? defaults?.config_meta?.host ?? "" : "");
    setSmtpPort(isEdit ? defaults?.config_meta?.port ?? "587" : "587");
    setSmtpTLSMode(((isEdit ? defaults?.config_meta?.tls_mode : undefined) as SmtpTLSMode | undefined) ?? "starttls");
    setSmtpAuthMode(((isEdit ? defaults?.config_meta?.auth_mode : undefined) as "plain" | "login" | undefined) ?? "plain");
    setSmtpUsername("");
    setSmtpPassword("");
  }

  function buildConfig(): (SesConfig | GmailConfig | SmtpConfig) | undefined {
    if (adapterType === "ses") {
      // In edit mode, only send config if user filled something
      if (isEdit && !sesRegion && !sesAccessKey && !sesSecretKey) return undefined;
      return {
        region: sesRegion,
        access_key_id: sesAccessKey,
        secret_access_key: sesSecretKey,
      };
    }

    if (adapterType === "gmail") {
      if (isEdit && !gmailServiceAccountJSON && !gmailDelegateEmail) return undefined;
      return {
        service_account_json: gmailServiceAccountJSON,
        delegate_email: gmailDelegateEmail,
      };
    }

    const previous = defaults?.config_meta;
    const smtpChanged =
      smtpHost !== (previous?.host ?? "") ||
      smtpPort !== (previous?.port ?? "587") ||
      smtpTLSMode !== ((previous?.tls_mode as SmtpTLSMode | undefined) ?? "starttls") ||
      smtpAuthMode !== ((previous?.auth_mode as "plain" | "login" | undefined) ?? "plain") ||
      !!smtpUsername ||
      !!smtpPassword;

    if (isEdit && !smtpChanged) return undefined;

    return {
      host: smtpHost,
      port: Number(smtpPort),
      tls_mode: smtpTLSMode,
      ...(smtpUsername || smtpPassword ? { auth_mode: smtpAuthMode } : {}),
      ...(smtpUsername ? { username: smtpUsername } : {}),
      ...(smtpPassword ? { password: smtpPassword } : {}),
    };
  }

  // For SES create: submit button first validates, then creates on second click.
  const sesFieldsReady = !!(sesRegion && sesAccessKey && sesSecretKey);
  const sesValidated = sesValidation?.valid === true;
  const needsValidation = !isEdit && adapterType === "ses" && sesFieldsReady && !sesValidated;

  async function handleSubmit(): Promise<boolean | void> {
    // SES create: first click validates, second click creates
    if (needsValidation) {
      await new Promise<void>((resolve, reject) => {
        validateSES.mutate(
          { region: sesRegion, access_key_id: sesAccessKey, secret_access_key: sesSecretKey },
          {
            onSuccess: (r) => { setSesValidation(r); resolve(); },
            onError: (e) => reject(e),
          }
        );
      });
      return true; // keep dialog open — wait for user to click "Create" next
    }

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

  // Compute whether the form is ready to submit.
  const isCreateReady = (() => {
    if (isEdit) return true;
    if (!name.trim()) return false;
    if (adapterType === "ses") {
      return sesFieldsReady; // button enables when fields are filled (validate or create)
    }
    if (adapterType === "smtp") {
      return !!(smtpHost.trim() && smtpPort.trim());
    }
    return !!(gmailServiceAccountJSON.trim() && gmailDelegateEmail.trim());
  })();

  // Dynamic submit button label + icon
  const submitLabel = (() => {
    if (isEdit) return "Update";
    if (adapterType === "ses" && needsValidation) return "Validate Credentials";
    return "Create Adapter";
  })();

  const submitIcon = (() => {
    if (isEdit) return undefined;
    if (adapterType === "ses" && needsValidation) return <ShieldCheck className="h-4 w-4" />;
    return <Rocket className="h-4 w-4" />;
  })();

  const loadingLabel = needsValidation ? "Validating..." : "Creating...";

  return (
    <FormDialog
      trigger={props.trigger}
      title={props.title ?? (isEdit ? "Edit Adapter" : "New Adapter")}
      description={
        isEdit
          ? "Update adapter settings. Leave config fields empty to keep current values."
          : "Configure a new email adapter for sending."
      }
      submitLabel={submitLabel}
      submitIcon={submitIcon}
      loadingLabel={loadingLabel}
      onSubmit={handleSubmit}
      open={isEdit ? props.open : undefined}
      onOpenChange={isEdit ? props.onOpenChange : undefined}
      submitDisabled={!isCreateReady}
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
            onValueChange={(v) => {
              const newType = v as AdapterType;
              setAdapterType(newType);
              if (!isEdit) {
                setRateLimit(ADAPTER_DEFAULTS[newType].rateLimit.toString());
              }
            }}
            disabled={isEdit}
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ses">SES (Amazon)</SelectItem>
              <SelectItem value="gmail">Gmail (Google)</SelectItem>
              <SelectItem value="smtp">SMTP</SelectItem>
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
          <p className="text-xs text-muted-foreground">
            {ADAPTER_DEFAULTS[adapterType].description}
            {ADAPTER_DEFAULTS[adapterType].docsUrl && (
              <>
                {" "}
                <a
                  href={ADAPTER_DEFAULTS[adapterType].docsUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  Docs
                </a>
              </>
            )}
          </p>
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
            {adapterType === "ses"
              ? "AWS SES Configuration"
              : adapterType === "gmail"
                ? "Gmail Configuration"
                : "SMTP Configuration"}
          </p>

          {adapterType === "ses" && (
            <div className="flex flex-col gap-3">
              {!isEdit && (
                <details className="rounded-md border border-blue-500/30 bg-blue-500/5 text-xs group">
                  <summary className="px-3 py-2 cursor-pointer font-medium text-blue-400 select-none list-none flex items-center gap-1.5">
                    <svg className="h-3 w-3 transition-transform group-open:rotate-90" viewBox="0 0 12 12" fill="currentColor"><path d="M4.5 2l4 4-4 4V2z"/></svg>
                    AWS setup &amp; required IAM permissions
                  </summary>
                  <div className="px-3 pb-3 space-y-3">
                    <ul className="list-disc pl-4 text-muted-foreground space-y-1">
                      <li>Use your credentials to send emails via SES</li>
                      <li>Create a <span className="font-mono text-foreground">Configuration Set</span> for event tracking</li>
                      <li>Create an <span className="font-mono text-foreground">SNS Topic</span> for delivery/bounce notifications</li>
                      <li>Subscribe Senda&apos;s webhook to receive events</li>
                    </ul>
                    <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                      <div className="min-w-0">
                        <p className="text-muted-foreground/70 mb-1">Sending</p>
                        <div className="flex flex-col gap-0.5">
                          <code className="text-[10px] font-mono text-muted-foreground break-all">ses:SendEmail</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">ses:SendRawEmail</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">ses:ListEmailIdentities</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">ses:GetAccount</code>
                        </div>
                      </div>
                      <div className="min-w-0">
                        <p className="text-muted-foreground/70 mb-1">Tracking <span className="text-muted-foreground/50">(optional)</span></p>
                        <div className="flex flex-col gap-0.5">
                          <code className="text-[10px] font-mono text-muted-foreground break-all">ses:CreateConfigurationSet</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">ses:CreateConfigurationSetEventDestination</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">ses:ListConfigurationSets</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">sns:CreateTopic</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">sns:Subscribe</code>
                          <code className="text-[10px] font-mono text-muted-foreground break-all">sns:ListTopics</code>
                        </div>
                      </div>
                    </div>
                    <div>
                      <p className="text-muted-foreground/70 mb-1">Cleanup on delete <span className="text-muted-foreground/50">(optional)</span></p>
                      <div className="flex flex-wrap gap-x-4 gap-y-0.5">
                        <code className="text-[10px] font-mono text-muted-foreground">ses:DeleteConfigurationSet</code>
                        <code className="text-[10px] font-mono text-muted-foreground">ses:DeleteConfigurationSetEventDestination</code>
                        <code className="text-[10px] font-mono text-muted-foreground">sns:Unsubscribe</code>
                        <code className="text-[10px] font-mono text-muted-foreground">sns:DeleteTopic</code>
                      </div>
                    </div>
                  </div>
                </details>
              )}
              <div className="flex flex-col gap-2">
                <Label htmlFor="ses-region">Region</Label>
                <RegionCombobox value={sesRegion} onChange={setSesRegion} required={!isEdit} />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="ses-access-key">Access Key ID</Label>
                <Input
                  id="ses-access-key"
                  value={sesAccessKey}
                  onChange={(e) => { setSesAccessKey(e.target.value); setSesValidation(null); }}
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
                  onChange={(e) => { setSesSecretKey(e.target.value); setSesValidation(null); }}
                  placeholder="••••••••"
                  className="font-mono"
                />
              </div>

              {/* Validation results (shown after validate step) */}
              {sesValidation && (
                <div className="rounded-md border p-3 space-y-1.5">
                  {sesValidation.checks.map((check) => (
                    <div key={check.name} className="flex items-center gap-2 text-xs">
                      {check.status === "ok" ? (
                        <Check className="h-3.5 w-3.5 text-green-500 shrink-0" />
                      ) : (
                        <ShieldAlert className="h-3.5 w-3.5 text-destructive shrink-0" />
                      )}
                      <span className={check.status === "ok" ? "text-foreground" : "text-destructive"}>
                        {check.description}
                      </span>
                      <span className="font-mono text-muted-foreground text-[10px]">
                        {check.permission}
                      </span>
                      {!check.required && check.status !== "ok" && (
                        <span className="text-muted-foreground">(optional)</span>
                      )}
                    </div>
                  ))}
                  <p className={cn("text-xs font-medium pt-1", sesValidation.valid ? "text-green-500" : "text-destructive")}>
                    {sesValidation.valid ? "All required permissions verified" : "Missing required permissions"}
                  </p>
                </div>
              )}
            </div>
          )}

          {adapterType === "gmail" && (
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

          {adapterType === "smtp" && (
            <div className="flex flex-col gap-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="smtp-host">Host</Label>
                  <Input
                    id="smtp-host"
                    value={smtpHost}
                    onChange={(e) => setSmtpHost(e.target.value)}
                    placeholder="smtp.example.com"
                    className="font-mono"
                    required={!isEdit}
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="smtp-port">Port</Label>
                  <Input
                    id="smtp-port"
                    type="number"
                    value={smtpPort}
                    onChange={(e) => setSmtpPort(e.target.value)}
                    placeholder="587"
                    className="font-mono"
                    required={!isEdit}
                  />
                </div>
              </div>
              <div className="flex flex-col gap-2">
                <Label>Auth Mode</Label>
                <Select
                  value={smtpAuthMode}
                  onValueChange={(v) => setSmtpAuthMode(v as "plain" | "login")}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="plain">PLAIN</SelectItem>
                    <SelectItem value="login">LOGIN</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-2">
                <Label>TLS Mode</Label>
                <Select
                  value={smtpTLSMode}
                  onValueChange={(v) => setSmtpTLSMode(v as SmtpTLSMode)}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="starttls">STARTTLS</SelectItem>
                    <SelectItem value="implicit_tls">Implicit TLS</SelectItem>
                    <SelectItem value="none">None</SelectItem>
                  </SelectContent>
                </Select>
                {smtpTLSMode === "none" && (
                  <p className="text-xs text-amber-400">
                    Plain SMTP sends without transport encryption. Use only for Mailpit, local testing, or trusted internal relays.
                  </p>
                )}
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="smtp-username">Username</Label>
                  <Input
                    id="smtp-username"
                    value={smtpUsername}
                    onChange={(e) => setSmtpUsername(e.target.value)}
                    placeholder="user or API key"
                    className="font-mono"
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="smtp-password">Password</Label>
                  <Input
                    id="smtp-password"
                    type="password"
                    value={smtpPassword}
                    onChange={(e) => setSmtpPassword(e.target.value)}
                    placeholder={isEdit ? "Leave empty to keep current" : "secret"}
                    className="font-mono"
                  />
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </FormDialog>
  );
}

function RegionCombobox({
  value,
  onChange,
  required,
}: {
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");

  const match = SES_REGIONS.find((r) => r.value === value);
  const displayLabel = match ? `${match.value} — ${match.label}` : value || "Select region...";

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className={cn("w-full justify-between font-mono text-sm", !value && "text-muted-foreground")}
          type="button"
        >
          <span className="truncate">{displayLabel}</span>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder="Search or type custom..."
            value={search}
            onValueChange={setSearch}
          />
          <CommandList>
            <CommandEmpty>
              <button
                type="button"
                className="w-full px-2 py-1.5 text-left text-sm font-mono"
                onClick={() => {
                  onChange(search);
                  setOpen(false);
                  setSearch("");
                }}
              >
                Use &quot;{search}&quot;
              </button>
            </CommandEmpty>
            <CommandGroup>
              {SES_REGIONS.filter(
                (r) =>
                  !search ||
                  r.value.includes(search.toLowerCase()) ||
                  r.label.toLowerCase().includes(search.toLowerCase())
              ).map((r) => (
                <CommandItem
                  key={r.value}
                  value={r.value}
                  onSelect={() => {
                    onChange(r.value);
                    setOpen(false);
                    setSearch("");
                  }}
                  className="font-mono"
                >
                  <Check className={cn("mr-2 h-4 w-4", value === r.value ? "opacity-100" : "opacity-0")} />
                  {r.value} — {r.label}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
      {required && !value && (
        <input type="text" required value="" className="sr-only" tabIndex={-1} onChange={() => {}} />
      )}
    </Popover>
  );
}
