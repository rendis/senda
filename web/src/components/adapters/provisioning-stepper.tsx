"use client";

import { useEffect, useRef } from "react";
import {
  CheckCircle,
  XCircle,
  Circle,
  Loader2,
  Copy,
  RotateCcw,
  Info,
  ExternalLink,
} from "lucide-react";
import Link from "next/link";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useProvisioningStatus,
  useAutoProvision,
} from "@/hooks/use-adapters";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import type { Adapter, ProvisioningStep } from "@/types/adapters";
import { PROVISIONING_STEPS } from "@/types/adapters";

function StepIcon({
  status,
  isProvisioning,
  isNext,
}: {
  status: string;
  isProvisioning: boolean;
  isNext: boolean;
}) {
  if (status === "completed") {
    return <CheckCircle className="h-5 w-5 text-emerald-500 shrink-0" />;
  }
  if (status === "failed") {
    return <XCircle className="h-5 w-5 text-destructive shrink-0" />;
  }
  if (isProvisioning && isNext) {
    return <Loader2 className="h-5 w-5 text-primary animate-spin shrink-0" />;
  }
  return <Circle className="h-5 w-5 text-muted-foreground/40 shrink-0" />;
}

function ResourcePanel({ steps }: { steps: ProvisioningStep[] }) {
  const resources = steps
    .filter((s) => s.status === "completed" && (s.resource_name || s.resource_arn))
    .map((s) => ({
      label: PROVISIONING_STEPS[s.name]?.label ?? s.name,
      name: s.resource_name,
      arn: s.resource_arn,
    }));

  if (resources.length === 0) return null;

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
    toast.success("Copied to clipboard");
  }

  return (
    <div className="mt-4 rounded-lg border bg-muted/30 p-4">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">
        AWS Resources Created
      </p>
      <div className="flex flex-col gap-2">
        {resources.map((r) => (
          <div key={r.label} className="flex flex-col gap-0.5">
            <span className="text-xs text-muted-foreground">{r.label}</span>
            {r.name && (
              <div className="flex items-center gap-1.5">
                <code className="text-xs font-mono bg-background px-1.5 py-0.5 rounded border truncate">
                  {r.name}
                </code>
                <button
                  onClick={() => copyToClipboard(r.name!)}
                  className="text-muted-foreground hover:text-foreground"
                >
                  <Copy className="h-3 w-3" />
                </button>
              </div>
            )}
            {r.arn && (
              <div className="flex items-center gap-1.5">
                <code className="text-xs font-mono bg-background px-1.5 py-0.5 rounded border truncate">
                  {r.arn}
                </code>
                <button
                  onClick={() => copyToClipboard(r.arn!)}
                  className="text-muted-foreground hover:text-foreground"
                >
                  <Copy className="h-3 w-3" />
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export function ProvisioningStepper({
  adapter,
  open,
  onOpenChange,
}: {
  adapter: Adapter;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scopedPath = useScopedPath();
  const scope = useScope();
  const autoStarted = useRef(false);

  // Build frontend navigation path for help links.
  const navBase =
    scope.level === "workspace"
      ? `/t/${scope.tenantCode}/w/${scope.workspaceCode}`
      : scope.level === "tenant"
        ? `/t/${scope.tenantCode}`
        : "/global";

  const { data: statusData, isLoading } = useProvisioningStatus(
    scopedPath,
    adapter.id,
    open
  );

  const provision = useAutoProvision(scopedPath, adapter.id);

  const status = statusData?.status ?? "not_started";
  const steps = statusData?.steps ?? [];
  const isProvisioning = provision.isPending || status === "in_progress";

  // Find first non-completed step index for spinner.
  const nextStepIndex = steps.findIndex((s) => s.status !== "completed");

  // Auto-start provisioning when dialog opens and status is not_started
  useEffect(() => {
    if (open && status === "not_started" && !isLoading && !autoStarted.current && !provision.isPending) {
      autoStarted.current = true;
      provision.mutate();
    }
  }, [open, status, isLoading, provision]);

  function handleStart() {
    provision.mutate();
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !isProvisioning && onOpenChange(v)}>
      <DialogContent
        className="sm:max-w-lg max-h-[85vh] flex flex-col"
        onInteractOutside={(e) => isProvisioning && e.preventDefault()}
      >
        <DialogHeader className="shrink-0">
          <DialogTitle>Setup Tracking — {adapter.name}</DialogTitle>
          <DialogDescription>
            Auto-provision AWS resources for email event tracking (delivery,
            bounce, complaint).
          </DialogDescription>
        </DialogHeader>

        <div className="py-4 overflow-y-auto min-h-0 -mx-6 px-6 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-muted-foreground/15 hover:[&::-webkit-scrollbar-thumb]:bg-muted-foreground/30 [&::-webkit-scrollbar-thumb]:rounded-full">
          {isLoading || (steps.length === 0 && !status) ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {steps.map((step, i) => (
                <div key={step.name} className="flex items-start gap-3">
                  <div className="mt-0.5">
                    <StepIcon
                      status={step.status}
                      isProvisioning={isProvisioning}
                      isNext={i === nextStepIndex}
                    />
                  </div>
                  <div className="flex flex-col gap-0.5 min-w-0">
                    <span className="text-sm font-medium">
                      {PROVISIONING_STEPS[step.name]?.label ?? step.name}
                    </span>
                    {step.status === "failed" && step.error_message && (
                      <p className="text-xs text-destructive font-mono break-all">
                        {step.error_message}
                      </p>
                    )}
                  </div>
                </div>
              ))}

              {status === "completed" && (
                <>
                  <ResourcePanel steps={steps} />
                  <div className="mt-4 rounded-lg border border-blue-200 bg-blue-50/50 dark:border-blue-900 dark:bg-blue-950/30 p-3 flex gap-2.5">
                    <Info className="h-4 w-4 text-blue-500 shrink-0 mt-0.5" />
                    <div className="flex flex-col gap-1">
                      <p className="text-xs text-blue-700 dark:text-blue-300">
                        SNS subscription confirmation may take a few seconds after provisioning.
                        If email events don&apos;t appear shortly, the webhook will auto-confirm — no action needed.
                      </p>
                      <Link
                        href={`${navBase}/help/ses-tracking`}
                        className="text-xs text-blue-600 dark:text-blue-400 hover:underline inline-flex items-center gap-1"
                      >
                        Learn more about SES provisioning
                        <ExternalLink className="h-3 w-3" />
                      </Link>
                    </div>
                  </div>
                </>
              )}
            </div>
          )}
        </div>

        <DialogFooter className="shrink-0 pt-4">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={isProvisioning}
          >
            {status === "completed" ? "Close" : "Cancel"}
          </Button>

          {status === "failed" && (
            <Button
              onClick={handleStart}
              disabled={isProvisioning}
              className="gap-2"
            >
              {isProvisioning ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Retrying...
                </>
              ) : (
                <>
                  <RotateCcw className="h-4 w-4" />
                  Retry
                </>
              )}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
