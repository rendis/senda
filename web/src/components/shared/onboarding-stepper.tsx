"use client";

import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

interface OnboardingStepperProps {
  currentStep: 1 | 2 | 3;
}

export function OnboardingStepper({ currentStep }: OnboardingStepperProps) {
  return (
    <div className="flex w-full items-center justify-center gap-2">
      <StepCircle step={1} currentStep={currentStep} />
      <Connector active={currentStep > 1} />
      <StepCircle step={2} currentStep={currentStep} />
      <Connector active={currentStep > 2} />
      <StepCircle step={3} currentStep={currentStep} />
    </div>
  );
}

function StepCircle({
  step,
  currentStep,
}: {
  step: number;
  currentStep: number;
}) {
  const isCompleted = step < currentStep;
  const isActive = step === currentStep;

  return (
    <div
      className={cn(
        "flex h-8 w-8 items-center justify-center rounded-full",
        isCompleted && "bg-primary-light",
        isActive && "bg-primary",
        !isCompleted && !isActive && "border border-border bg-surface",
      )}
    >
      {isCompleted ? (
        <Check className="h-4 w-4 text-primary" />
      ) : (
        <span
          className={cn(
            "text-sm",
            isActive
              ? "font-semibold text-primary-foreground"
              : "font-medium text-muted-foreground",
          )}
        >
          {step}
        </span>
      )}
    </div>
  );
}

function Connector({ active }: { active: boolean }) {
  return (
    <div
      className={cn("h-0.5 w-12", active ? "bg-primary" : "bg-border")}
    />
  );
}
