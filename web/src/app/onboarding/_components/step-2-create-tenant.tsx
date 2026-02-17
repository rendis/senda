"use client";

import { useRef, useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Building, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createAuthenticatedApi, parseApiError } from "@/lib/api";
import { OnboardingStepper } from "@/components/shared/onboarding-stepper";
import { slugSchema, nameSchema, generateSlug } from "@/lib/validations/slug";
import type { OnboardingSetupResponse } from "@/types/api";
import { HTTPError } from "ky";

const tenantSchema = z.object({
  name: nameSchema,
  code: slugSchema,
});

type TenantFormValues = z.infer<typeof tenantSchema>;

interface Step2CreateTenantProps {
  currentStep: 1 | 2 | 3;
  idToken: string;
  onSuccess: (tenantCode: string) => void;
}

export function Step2CreateTenant({
  currentStep,
  idToken,
  onSuccess,
}: Step2CreateTenantProps) {
  const [submitting, setSubmitting] = useState(false);
  const codeManuallyEdited = useRef(false);

  const form = useForm<TenantFormValues>({
    resolver: zodResolver(tenantSchema),
    defaultValues: { name: "", code: "" },
  });

  const watchedName = form.watch("name");

  useEffect(() => {
    if (!codeManuallyEdited.current && watchedName) {
      form.setValue("code", generateSlug(watchedName), {
        shouldValidate: form.formState.isSubmitted,
      });
    }
  }, [watchedName, form]);

  const handleSubmit = async (values: TenantFormValues) => {
    setSubmitting(true);
    try {
      const api = createAuthenticatedApi(idToken);
      await api
        .post("onboarding/setup", {
          json: { tenant_code: values.code, tenant_name: values.name },
        })
        .json<OnboardingSetupResponse>();
      toast.success("Tenant creado exitosamente");
      onSuccess(values.code);
    } catch (error) {
      if (error instanceof HTTPError && error.response.status === 409) {
        onSuccess(values.code);
        return;
      }
      const apiError = await parseApiError(error);
      if (apiError.error.details?.length) {
        for (const fe of apiError.error.details) {
          const field = fe.field === "tenant_code" ? "code" : fe.field === "tenant_name" ? "name" : null;
          if (field) {
            form.setError(field as keyof TenantFormValues, {
              message: fe.message,
            });
          }
        }
      } else {
        toast.error(apiError.error.message || "Error al crear el tenant");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <div className="flex flex-col items-center gap-2">
        <h1 className="text-[22px] font-bold text-foreground">
          Paso 2: Crear primer Tenant
        </h1>
        <p className="w-[380px] text-center text-sm leading-relaxed text-muted-foreground">
          Un tenant agrupa workspaces y configuraciones de tu organización.
        </p>
      </div>

      {/* Stepper */}
      <OnboardingStepper currentStep={currentStep} />

      <form
        onSubmit={form.handleSubmit(handleSubmit)}
        className="flex w-full flex-col gap-7"
      >
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="tenant-name" className="text-[13px] font-medium">
              Nombre del Tenant
            </Label>
            <Input
              id="tenant-name"
              placeholder="Mi Organización"
              {...form.register("name")}
            />
            {form.formState.errors.name && (
              <p className="text-xs text-destructive">
                {form.formState.errors.name.message}
              </p>
            )}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="tenant-code" className="text-[13px] font-medium">
              Código (slug)
            </Label>
            <Input
              id="tenant-code"
              placeholder="mi-organizacion"
              {...form.register("code", {
                onChange: () => {
                  codeManuallyEdited.current = true;
                },
              })}
            />
            {form.formState.errors.code && (
              <p className="text-xs text-destructive">
                {form.formState.errors.code.message}
              </p>
            )}
          </div>
        </div>

        <Button
          type="submit"
          disabled={submitting}
          className="h-9 w-full gap-2 text-[13px] font-medium"
        >
          {submitting ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Building className="h-4 w-4" />
          )}
          Crear Tenant
        </Button>
      </form>
    </>
  );
}
