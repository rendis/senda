"use client";

import { useProvisioningStatus } from "@/hooks/use-adapters";
import type { ProvisioningStep } from "@/types/adapters";
import { PROVISIONING_STEPS } from "@/types/adapters";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

function StepDot({ status }: { status: string }) {
  return (
    <div
      className={cn(
        "h-2 w-2 rounded-full transition-colors",
        status === "completed" && "bg-emerald-500",
        status === "failed" && "bg-destructive",
        status === "pending" && "bg-muted-foreground/25"
      )}
    />
  );
}

function StepLine({ status }: { status: string }) {
  return (
    <div
      className={cn(
        "h-0.5 w-2 transition-colors",
        status === "completed" && "bg-emerald-500/50",
        status === "failed" && "bg-destructive/50",
        status === "pending" && "bg-muted-foreground/15"
      )}
    />
  );
}

function StatusLabel({ status, steps }: { status: string; steps: ProvisioningStep[] }) {
  if (status === "completed") return <span className="text-[10px] text-emerald-500 font-medium">Ready</span>;
  if (status === "failed") {
    const failed = steps.find((s) => s.status === "failed");
    return <span className="text-[10px] text-destructive font-medium truncate max-w-[80px]">{PROVISIONING_STEPS[failed?.name ?? ""]?.short ?? "Failed"}</span>;
  }
  if (status === "in_progress") return <span className="text-[10px] text-primary font-medium">Setting up...</span>;
  return <span className="text-[10px] text-muted-foreground">Not started</span>;
}

export function TrackingStatus({
  adapterId,
  scopedPath,
  onClick,
}: {
  adapterId: string;
  scopedPath: string;
  onClick: () => void;
}) {
  const { data } = useProvisioningStatus(scopedPath, adapterId, true);
  const status = data?.status ?? "not_started";
  const steps = data?.steps ?? [];

  const tooltipText =
    status === "completed"
      ? "Tracking active — click for details"
      : status === "failed"
        ? "Setup failed — click to retry"
        : status === "in_progress"
          ? "Setting up tracking..."
          : "Click to set up tracking";

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={onClick}
          className="flex items-center gap-1.5 py-1 group cursor-pointer"
        >
          {steps.length > 0 ? (
            <div className="flex items-center">
              {steps.map((step, i) => (
                <div key={step.name} className="flex items-center">
                  {i > 0 && <StepLine status={steps[i - 1].status} />}
                  <StepDot status={step.status} />
                </div>
              ))}
            </div>
          ) : (
            <div className="flex items-center">
              {[0, 1, 2, 3, 4].map((i) => (
                <div key={i} className="flex items-center">
                  {i > 0 && <StepLine status="pending" />}
                  <StepDot status="pending" />
                </div>
              ))}
            </div>
          )}
          <StatusLabel status={status} steps={steps} />
        </button>
      </TooltipTrigger>
      <TooltipContent>{tooltipText}</TooltipContent>
    </Tooltip>
  );
}
