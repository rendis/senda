"use client";

import { useRef, useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { LayoutGrid, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createAuthenticatedApi, parseApiError } from "@/lib/api";
import { OnboardingStepper } from "@/components/shared/onboarding-stepper";
import { slugSchema, nameSchema, generateSlug } from "@/lib/validations/slug";
import { HTTPError } from "ky";

const workspaceSchema = z.object({
  name: nameSchema,
  code: slugSchema,
});

type WorkspaceFormValues = z.infer<typeof workspaceSchema>;

interface Step3CreateWorkspaceProps {
  currentStep: 1 | 2 | 3;
  idToken: string;
  tenantCode: string;
  onSuccess: (workspaceCode: string, tenantCode: string) => void;
}

export function Step3CreateWorkspace({
  currentStep,
  idToken,
  tenantCode,
  onSuccess,
}: Step3CreateWorkspaceProps) {
  const [submitting, setSubmitting] = useState(false);
  const codeManuallyEdited = useRef(false);

  const form = useForm<WorkspaceFormValues>({
    resolver: zodResolver(workspaceSchema),
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

  const handleSubmit = async (values: WorkspaceFormValues) => {
    setSubmitting(true);
    try {
      const api = createAuthenticatedApi(idToken);
      await api
        .post(`manage/tenants/${tenantCode}/workspaces`, {
          json: { code: values.code, name: values.name },
        })
        .json();
      toast.success("Workspace creado exitosamente");
      onSuccess(values.code, tenantCode);
    } catch (error) {
      if (error instanceof HTTPError && error.response.status === 409) {
        onSuccess(values.code, tenantCode);
        return;
      }
      const apiError = await parseApiError(error);
      if (apiError.error.details?.length) {
        for (const fe of apiError.error.details) {
          const field = fe.field === "code" ? "code" : fe.field === "name" ? "name" : null;
          if (field) {
            form.setError(field as keyof WorkspaceFormValues, {
              message: fe.message,
            });
          }
        }
      } else {
        toast.error(apiError.error.message || "Error al crear el workspace");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <div className="flex flex-col items-center gap-2">
        <h1 className="text-[22px] font-bold text-foreground">
          Paso 3: Crear primer Workspace
        </h1>
        <p className="w-[380px] text-center text-sm leading-relaxed text-muted-foreground">
          Un workspace es tu espacio de trabajo donde gestionas templates,
          injectors y envíos.
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
            <Label
              htmlFor="workspace-name"
              className="text-[13px] font-medium"
            >
              Nombre del Workspace
            </Label>
            <Input
              id="workspace-name"
              placeholder="Producción"
              {...form.register("name")}
            />
            {form.formState.errors.name && (
              <p className="text-xs text-destructive">
                {form.formState.errors.name.message}
              </p>
            )}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label
              htmlFor="workspace-code"
              className="text-[13px] font-medium"
            >
              Código (slug)
            </Label>
            <Input
              id="workspace-code"
              placeholder="produccion"
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
            <LayoutGrid className="h-4 w-4" />
          )}
          Crear Workspace
        </Button>
      </form>
    </>
  );
}
