"use client";

import { useTranslations } from "next-intl";
import { KeyRound } from "lucide-react";
import { BrandLogo } from "@/components/shared/brand-logo";
import { Button } from "@/components/ui/button";
import { OnboardingStepper } from "@/components/shared/onboarding-stepper";

interface Step1WelcomeProps {
  currentStep: 1 | 2 | 3;
  onConnect: () => void;
}

export function Step1Welcome({ currentStep, onConnect }: Step1WelcomeProps) {
  const t = useTranslations("onboarding.welcome");

  return (
    <>
      {/* Header */}
      <div className="flex flex-col items-center gap-2">
        <BrandLogo size="lg" priority />
        <h1 className="text-2xl font-bold text-foreground">
          {t("title")}
        </h1>
        <p className="w-full max-w-[360px] text-center text-sm leading-relaxed text-foreground/70">
          {t("subtitle")}
        </p>
      </div>

      {/* Stepper */}
      <OnboardingStepper currentStep={currentStep} />

      {/* Divider */}
      <div className="h-px w-full bg-border" />

      {/* Action */}
      <Button
        onClick={onConnect}
        className="h-9 w-full gap-2 text-[13px] font-medium"
      >
        <KeyRound className="h-4 w-4" />
        {t("connectOidc")}
      </Button>
    </>
  );
}
