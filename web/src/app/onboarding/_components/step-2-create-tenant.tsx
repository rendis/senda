"use client";

import { useRef, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
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
import { getHttpStatus } from "./api-errors";

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
  const t = useTranslations("onboarding.createTenant");
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
      toast.success(t("successToast"));
      onSuccess(values.code);
    } catch (error) {
      const statusCode = getHttpStatus(error);
      if (statusCode === 401) {
        window.location.replace("/login");
        return;
      }
      if (statusCode === 409) {
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
        toast.error(apiError.error.message || t("errorToast"));
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <div className="flex flex-col items-center gap-2">
        <h1 className="text-[22px] font-bold text-foreground">
          {t("title")}
        </h1>
        <p className="w-full max-w-[380px] text-center text-sm leading-relaxed text-muted-foreground">
          {t("subtitle")}
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
              {t("nameLabel")}
            </Label>
            <Input
              id="tenant-name"
              placeholder={t("namePlaceholder")}
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
              {t("tenantCode")}
            </Label>
            <Input
              id="tenant-code"
              placeholder={t("tenantCodePlaceholder")}
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
          {submitting ? t("creating") : t("create")}
        </Button>
      </form>
    </>
  );
}
