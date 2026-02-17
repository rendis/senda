"use client";

import { useState, useCallback } from "react";
import { useSession, signIn } from "next-auth/react";
import { useRouter } from "next/navigation";
import { Step1Welcome } from "./step-1-welcome";
import { Step2CreateTenant } from "./step-2-create-tenant";
import { Step3CreateWorkspace } from "./step-3-create-workspace";
import { cn } from "@/lib/utils";

const STORAGE_KEY = "senda-onboarding";

interface PersistedState {
  step: 1 | 2 | 3;
  tenantCode?: string;
}

function loadState(): PersistedState {
  if (typeof window === "undefined") return { step: 1 };
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as PersistedState;
  } catch {
    /* ignore */
  }
  return { step: 1 };
}

function saveState(state: PersistedState) {
  sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

function clearState() {
  sessionStorage.removeItem(STORAGE_KEY);
}

export function OnboardingWizard() {
  const { data: session, status } = useSession();
  const router = useRouter();

  // Initialize from sessionStorage (lazy)
  const [wizardStep, setWizardStep] = useState<1 | 2 | 3>(
    () => loadState().step,
  );
  const [tenantCode, setTenantCode] = useState<string>(
    () => loadState().tenantCode ?? "",
  );

  // Derive effective step: auto-advance to 2 when authenticated
  const step: 1 | 2 | 3 =
    status === "authenticated" && wizardStep === 1 ? 2 : wizardStep;

  const handleConnect = useCallback(() => {
    signIn("oidc", { callbackUrl: "/onboarding" });
  }, []);

  const handleTenantCreated = useCallback((code: string) => {
    setTenantCode(code);
    setWizardStep(3);
    saveState({ step: 3, tenantCode: code });
  }, []);

  const handleWorkspaceCreated = useCallback(
    (workspaceCode: string, forTenantCode: string) => {
      clearState();
      router.replace(`/t/${forTenantCode}/w/${workspaceCode}`);
    },
    [router],
  );

  // Show nothing while session is loading
  if (status === "loading") return null;

  // Persist step 2 to sessionStorage when auto-advancing
  if (status === "authenticated" && wizardStep === 1) {
    saveState({ step: 2 });
  }

  const idToken = session?.idToken ?? "";

  return (
    <div className="flex min-h-screen items-center justify-center bg-page">
      <div className="w-[480px] rounded-lg border border-border bg-card px-10 py-12">
        <div
          className={cn(
            "flex flex-col items-center",
            step === 1 ? "gap-8" : "gap-7",
          )}
        >
          {step === 1 && (
            <Step1Welcome currentStep={step} onConnect={handleConnect} />
          )}

          {step === 2 && (
            <Step2CreateTenant
              currentStep={step}
              idToken={idToken}
              onSuccess={handleTenantCreated}
            />
          )}

          {step === 3 && (
            <Step3CreateWorkspace
              currentStep={step}
              idToken={idToken}
              tenantCode={tenantCode}
              onSuccess={handleWorkspaceCreated}
            />
          )}
        </div>
      </div>
    </div>
  );
}
