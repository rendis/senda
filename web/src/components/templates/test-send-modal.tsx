"use client";

import { useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTestSend } from "@/hooks/use-template-version";
import { useInjectorList } from "@/hooks/use-injectors";
import type { InjectorDefinition, InjectorField, InjectorFieldType } from "@/types/injectors";
import { toast } from "sonner";
import { getTestSendScrollableBodyClassName } from "./test-send-modal-layout";
import {
  type TemplateInjectorUsage,
} from "./test-send-injector-usage";
import {
  resolveTestSendInjectorCatalogRequest,
  resolveVisibleTestSendInjectors,
} from "./test-send-modal-catalog";

interface TestSendModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  scopedPath: string;
  templateId: string;
  locale?: string;
  sendEnabled?: boolean;
  sendDisabledReason?: string;
  allowedInjectorUsage?: TemplateInjectorUsage;
}

type InjectorFieldState = {
  value: string;
  touched: boolean;
};

type InjectorState = Record<string, Record<string, InjectorFieldState>>;

export function TestSendModal({
  open,
  onOpenChange,
  scopedPath,
  templateId,
  locale,
  sendEnabled = true,
  sendDisabledReason,
  allowedInjectorUsage = {},
}: TestSendModalProps) {
  const [email, setEmail] = useState("");
  const [variablesJson, setVariablesJson] = useState("{}");
  const [injectorState, setInjectorState] = useState<InjectorState | null>(null);

  const testSend = useTestSend(scopedPath, templateId);
  const injectorCatalogRequest = resolveTestSendInjectorCatalogRequest(allowedInjectorUsage);
  const injectorList = useInjectorList(scopedPath, {
    enabled: open && injectorCatalogRequest.enabled,
    includeInherited: injectorCatalogRequest.includeInherited,
  });
  const injectorItems = useMemo(
    () => resolveVisibleTestSendInjectors(injectorList.data?.items ?? [], allowedInjectorUsage),
    [allowedInjectorUsage, injectorList.data],
  );
  const activeInjectorState = injectorState ?? buildInitialInjectorState(injectorItems);

  const hasInjectors = injectorItems.length > 0;
  const templateUsesInjectors = Object.keys(allowedInjectorUsage).length > 0;

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      setInjectorState(null);
    }
    onOpenChange(nextOpen);
  }

  function updateField(injectorName: string, fieldName: string, value: string) {
    setInjectorState((prev) => ({
      ...(prev ?? buildInitialInjectorState(injectorItems)),
      [injectorName]: {
        ...((prev ?? buildInitialInjectorState(injectorItems))[injectorName] ?? {}),
        [fieldName]: {
          value,
          touched: true,
        },
      },
    }));
  }

  async function handleSend() {
    if (!sendEnabled) {
      toast.error(
        sendDisabledReason ??
          "Assign an adapter to this template type before sending test emails.",
      );
      return;
    }

    if (!email.trim()) {
      toast.error("Please enter a recipient email");
      return;
    }

    let variables: Record<string, unknown> = {};
    try {
      variables = JSON.parse(variablesJson);
    } catch {
      toast.error("Invalid JSON for variables");
      return;
    }

    try {
      await testSend.mutateAsync({
        recipient_email: email.trim(),
        variables,
        injectors: buildInjectorPayload(injectorItems, activeInjectorState),
        locale,
      });
      toast.success(`Test email sent to ${email}`);
      handleOpenChange(false);
    } catch {
      toast.error("Failed to send test email");
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !testSend.isPending && handleOpenChange(v)}>
      <DialogContent
        className="sm:max-w-2xl max-h-[85vh] flex flex-col overflow-hidden"
        onInteractOutside={(e) => testSend.isPending && e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>Send Test Email</DialogTitle>
          <DialogDescription>
            Send a test email to verify your template renders correctly.
          </DialogDescription>
        </DialogHeader>
        <div className={getTestSendScrollableBodyClassName()}>
          <fieldset
            disabled={testSend.isPending}
            className="flex min-h-0 flex-col gap-6 py-4"
          >
            {!sendEnabled && (
              <div className="rounded-md border border-status-complained/40 bg-status-complained-bg px-3 py-2 text-sm text-status-complained">
                {sendDisabledReason ??
                  "Assign an adapter to this template type before sending test emails."}
              </div>
            )}
            <div className="flex flex-col gap-2">
              <Label htmlFor="test-email">Recipient Email</Label>
              <Input
                id="test-email"
                data-testid="test-send-email"
                type="email"
                placeholder="test@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="test-vars">Variables (JSON)</Label>
              <textarea
                id="test-vars"
                data-testid="test-send-variables-json"
                className="flex min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                value={variablesJson}
                onChange={(e) => setVariablesJson(e.target.value)}
                placeholder='{"user_name": "Juan", "cta_url": "https://..."}'
              />
            </div>

            <div className="flex min-h-0 flex-col gap-3">
              <div className="flex items-center justify-between gap-2">
                <div className="flex flex-col gap-1">
                  <Label>Injectors</Label>
                  <p className="text-xs text-muted-foreground">
                    Leave overwriteable fields untouched to keep the runtime fallback. Locked fields
                    always use their default value.
                  </p>
                </div>
              </div>

              {!templateUsesInjectors ? (
                <p className="text-sm text-muted-foreground">
                  This template does not reference injector variables.
                </p>
              ) : injectorList.isLoading ? (
                <p className="text-sm text-muted-foreground">Loading injectors…</p>
              ) : hasInjectors ? (
                <div className="min-w-0 space-y-5">
                  {injectorItems.map((injector) => (
                    <section
                      key={injector.name}
                      data-testid={`test-send-injector-${injector.name}`}
                      className="min-w-0 space-y-3"
                    >
                      <div className="space-y-1">
                        <p className="font-mono text-sm font-medium">{injector.name}</p>
                        {injector.description ? (
                          <p className="text-xs text-muted-foreground">{injector.description}</p>
                        ) : null}
                      </div>

                      <div className="divide-y divide-border">
                        {(injector.fields ?? [])
                          .slice()
                          .sort((a, b) => a.position - b.position)
                          .map((field) => {
                            const current = activeInjectorState[injector.name]?.[field.field_name] ?? {
                              value: serializeFieldValue(field.field_type, field.default_value),
                              touched: false,
                            };

                            return (
                              <div
                                key={`${injector.name}-${field.field_name}`}
                                className="min-w-0 space-y-3 py-3 first:pt-0 last:pb-0"
                              >
                                <div className="space-y-1">
                                  <Label
                                    htmlFor={`injector-${injector.name}-${field.field_name}`}
                                    className="font-mono text-xs"
                                  >
                                    {field.field_name}
                                  </Label>
                                  {field.description ? (
                                    <p className="text-xs text-muted-foreground">
                                      {field.description}
                                    </p>
                                  ) : null}
                                </div>

                                <InjectorFieldInput
                                  injectorName={injector.name}
                                  field={field}
                                  state={current}
                                  onChange={updateField}
                                />

                                <div className="grid gap-x-4 gap-y-1 text-xs text-muted-foreground sm:grid-cols-2">
                                  <p className="min-w-0">Type: {field.field_type}</p>
                                  <p className="min-w-0">
                                    Mode:{" "}
                                    <span className="font-medium text-foreground">
                                      {field.allow_overwrite ? "overwrite enabled" : "locked"}
                                    </span>
                                  </p>
                                  <p className="min-w-0">
                                    Default:{" "}
                                    <span className="font-mono text-foreground">
                                      {serializeFieldValue(field.field_type, field.default_value) || "∅"}
                                    </span>
                                  </p>
                                  <p className="min-w-0">
                                    Payload:{" "}
                                    <span className="font-medium text-foreground">
                                      {current.touched && field.allow_overwrite ? "override" : "fallback"}
                                    </span>
                                  </p>
                                </div>
                              </div>
                            );
                          })}
                      </div>
                    </section>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  This template references injectors that are not available in this scope.
                </p>
              )}
            </div>
          </fieldset>
        </div>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={testSend.isPending}
          >
            Cancel
          </Button>
          <Button
            data-testid="test-send-submit"
            onClick={handleSend}
            disabled={testSend.isPending || !sendEnabled}
            title={!sendEnabled ? sendDisabledReason : undefined}
          >
            {testSend.isPending ? "Sending..." : "Send Test"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function InjectorFieldInput({
  injectorName,
  field,
  state,
  onChange,
}: {
  injectorName: string;
  field: InjectorField;
  state: InjectorFieldState;
  onChange: (injectorName: string, fieldName: string, value: string) => void;
}) {
  const id = `injector-${injectorName}-${field.field_name}`;
  const disabled = !field.allow_overwrite;

  if (field.field_type === "bool") {
    return (
      <label className="flex items-center gap-2 text-sm text-foreground">
        <input
          id={id}
          data-testid={`test-send-field-${injectorName}-${field.field_name}`}
          type="checkbox"
          checked={state.value === "true"}
          disabled={disabled}
          onChange={(e) => onChange(injectorName, field.field_name, String(e.target.checked))}
          className="h-4 w-4"
        />
        {state.value === "true" ? "True" : "False"}
      </label>
    );
  }

  return (
    <Input
      id={id}
      data-testid={`test-send-field-${injectorName}-${field.field_name}`}
      type={field.field_type === "number" ? "number" : "text"}
      value={state.value}
      disabled={disabled}
      onChange={(e) => onChange(injectorName, field.field_name, e.target.value)}
      placeholder={getPlaceholder(field.field_type)}
      className="min-w-0 font-mono text-[13px]"
    />
  );
}

function buildInitialInjectorState(injectors: InjectorDefinition[]): InjectorState {
  return injectors.reduce<InjectorState>((acc, injector) => {
    acc[injector.name] = (injector.fields ?? []).reduce<Record<string, InjectorFieldState>>(
      (fieldAcc, field) => {
        fieldAcc[field.field_name] = {
          value: serializeFieldValue(field.field_type, field.default_value),
          touched: false,
        };
        return fieldAcc;
      },
      {}
    );
    return acc;
  }, {});
}

function buildInjectorPayload(
  injectors: InjectorDefinition[],
  state: InjectorState
): Record<string, Record<string, unknown>> | undefined {
  const payload = injectors.reduce<Record<string, Record<string, unknown>>>((acc, injector) => {
    const fieldPayload = (injector.fields ?? []).reduce<Record<string, unknown>>((fieldAcc, field) => {
      if (!field.allow_overwrite) {
        return fieldAcc;
      }

      const current = state[injector.name]?.[field.field_name];
      if (!current?.touched) {
        return fieldAcc;
      }

      fieldAcc[field.field_name] = parseFieldValueForPayload(field.field_type, current.value);
      return fieldAcc;
    }, {});

    if (Object.keys(fieldPayload).length > 0) {
      acc[injector.name] = fieldPayload;
    }
    return acc;
  }, {});

  return Object.keys(payload).length > 0 ? payload : undefined;
}

function serializeFieldValue(fieldType: InjectorFieldType, value: unknown): string {
  if (value == null) {
    return fieldType === "bool" ? "false" : "";
  }
  if (fieldType === "bool") {
    return String(Boolean(value));
  }
  return String(value);
}

function parseFieldValueForPayload(fieldType: InjectorFieldType, value: string): unknown {
  if (fieldType === "bool") {
    return value === "true";
  }
  if (fieldType === "number") {
    return value === "" ? "" : Number(value);
  }
  return value;
}

function getPlaceholder(fieldType: InjectorFieldType): string {
  switch (fieldType) {
    case "img":
      return "https://cdn.example.com/logo.png";
    case "url":
      return "https://example.com";
    case "html":
      return "<p>Footer</p>";
    case "number":
      return "0";
    case "bool":
      return "false";
    default:
      return "Override value...";
  }
}
