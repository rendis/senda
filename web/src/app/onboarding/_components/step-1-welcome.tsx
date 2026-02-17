"use client";

import { Send, KeyRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OnboardingStepper } from "@/components/shared/onboarding-stepper";

interface Step1WelcomeProps {
  currentStep: 1 | 2 | 3;
  onConnect: () => void;
}

export function Step1Welcome({ currentStep, onConnect }: Step1WelcomeProps) {
  return (
    <>
      {/* Header */}
      <div className="flex flex-col items-center gap-2">
        <Send className="h-12 w-12 text-primary" />
        <h1 className="text-2xl font-bold text-foreground">
          Bienvenido a Senda
        </h1>
        <p className="w-[360px] text-center text-sm leading-relaxed text-muted-foreground">
          Configura tu plataforma de email en 3 simples pasos.
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
        Conectar con tu proveedor OIDC
      </Button>
    </>
  );
}
