"use client";

import { useState } from "react";
import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FormDialog } from "@/components/shared/form-dialog";
import { Label } from "@/components/ui/label";
import { InjectorFieldCard } from "@/components/injectors/injector-field-card";
import {
  DefaultValueInput,
  OverwriteModeToggle,
} from "@/components/injectors/field-runtime-controls";
import {
  buildInjectorRequest,
  emptyInjectorFormField,
  injectorDefinitionToFormValues,
  isEmptyInjectorFieldDefault,
  type InjectorFormFieldEntry,
} from "@/components/injectors/injector-form-model";
import type {
  InjectorDefinition,
  InjectorFieldType,
  CreateInjectorRequest,
} from "@/types/injectors";

const FIELD_TYPES: { value: InjectorFieldType; label: string }[] = [
  { value: "text", label: "Text" },
  { value: "number", label: "Number" },
  { value: "bool", label: "Boolean" },
  { value: "img", label: "Image URL" },
  { value: "url", label: "URL" },
  { value: "html", label: "HTML" },
];

interface InjectorFormProps {
  trigger?: React.ReactNode;
  onSubmit: (data: CreateInjectorRequest) => Promise<void>;
  title?: string;
  mode?: "create" | "edit";
  injector?: InjectorDefinition;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export function InjectorForm({
  trigger,
  onSubmit,
  title,
  mode = "create",
  injector,
  open,
  onOpenChange,
}: InjectorFormProps) {
  const initialValues = injectorDefinitionToFormValues(
    mode === "edit" ? injector : undefined,
  );
  const [name, setName] = useState(initialValues.name);
  const [description, setDescription] = useState(initialValues.description);
  const [fields, setFields] = useState<InjectorFormFieldEntry[]>(initialValues.fields);

  function resetForm() {
    const values = injectorDefinitionToFormValues(mode === "edit" ? injector : undefined);
    setName(values.name);
    setDescription(values.description);
    setFields(values.fields);
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange?.(nextOpen);
  }

  function updateField(index: number, patch: Partial<InjectorFormFieldEntry>) {
    setFields((prev) =>
      prev.map((field, currentIndex) =>
        currentIndex === index ? { ...field, ...patch } : field,
      ),
    );
  }

  function removeField(index: number) {
    setFields((prev) => {
      if (prev.length === 1) {
        return prev;
      }

      return prev.filter((_, currentIndex) => currentIndex !== index);
    });
  }

  function moveField(index: number, direction: "up" | "down") {
    setFields((prev) => {
      const nextIndex = direction === "up" ? index - 1 : index + 1;
      if (nextIndex < 0 || nextIndex >= prev.length) {
        return prev;
      }

      const next = [...prev];
      [next[index], next[nextIndex]] = [next[nextIndex], next[index]];
      return next;
    });
  }

  async function handleSubmit() {
    await onSubmit(
      buildInjectorRequest({
        name,
        description,
        fields,
      }),
    );
    resetForm();
  }

  const hasLockedFieldWithoutDefault = fields.some(
    (field) => !field.allow_overwrite && isEmptyInjectorFieldDefault(field),
  );

  const submitDisabled =
    !name.trim() ||
    fields.length === 0 ||
    fields.some((field) => !field.field_name.trim()) ||
    hasLockedFieldWithoutDefault;

  const dialogTitle =
    title ?? (mode === "edit" ? "Edit Injector" : "New Injector");

  return (
    <FormDialog
      trigger={trigger}
      title={dialogTitle}
      description={
        mode === "edit"
          ? "Update the injector schema, field order, and runtime defaults."
          : "Define the injectors your template builders can use."
      }
      submitLabel={mode === "edit" ? "Update" : "Create"}
      onSubmit={handleSubmit}
      submitDisabled={submitDisabled}
      open={open}
      onOpenChange={handleOpenChange}
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="injector-name">Name</Label>
          <Input
            id="injector-name"
            data-testid="injector-name-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="student"
            className="font-mono"
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="injector-description">Description</Label>
          <Input
            id="injector-description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description…"
          />
        </div>

        <div className="border-t pt-4 flex flex-col gap-3">
          <p className="text-xs font-medium text-muted-foreground">Fields</p>

          {fields.map((field, index) => (
            <InjectorFieldCard
              key={`${field.field_name || "field"}-${index}`}
              testId={`injector-field-row-${index}`}
              headerTestId={`injector-field-header-${index}`}
              title={field.field_name.trim() || `Field ${index + 1}`}
              typeLabel={fieldTypeLabel(field.field_type)}
              description={field.description.trim() || undefined}
              actions={
                <>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={`Move field ${field.field_name || index + 1} up`}
                    disabled={index === 0}
                    onClick={() => moveField(index, "up")}
                    className="rounded-md text-muted-foreground hover:text-foreground"
                  >
                    <ArrowUp className="size-3.5" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={`Move field ${field.field_name || index + 1} down`}
                    disabled={index === fields.length - 1}
                    onClick={() => moveField(index, "down")}
                    className="rounded-md text-muted-foreground hover:text-foreground"
                  >
                    <ArrowDown className="size-3.5" />
                  </Button>
                  <OverwriteModeToggle
                    allowOverwrite={field.allow_overwrite}
                    onChange={(value) =>
                      updateField(index, { allow_overwrite: value })
                    }
                    testIdPrefix={`injector-field-overwrite-${index}`}
                  />
                  {fields.length > 1 ? (
                    <Button
                      data-testid={`injector-field-remove-${index}`}
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      aria-label={`Remove field ${field.field_name || index + 1}`}
                      onClick={() => removeField(index)}
                      className="rounded-md text-muted-foreground hover:text-foreground"
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  ) : null}
                </>
              }
            >
              <div className="flex gap-2">
                <Input
                  data-testid={`injector-field-name-${index}`}
                  value={field.field_name}
                  onChange={(e) =>
                    updateField(index, { field_name: e.target.value })
                  }
                  placeholder="name"
                  aria-label={`Field ${index + 1} name`}
                  className="font-mono flex-1"
                />
                <Select
                  value={field.field_type}
                  onValueChange={(value) =>
                    updateField(index, {
                      field_type: value as InjectorFieldType,
                      default_value:
                        value === "bool"
                          ? field.default_value || "false"
                          : field.default_value,
                    })
                  }
                >
                  <SelectTrigger
                    className="w-[140px]"
                    aria-label={`Field ${index + 1} type`}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FIELD_TYPES.map((fieldType) => (
                      <SelectItem key={fieldType.value} value={fieldType.value}>
                        {fieldType.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <Input
                data-testid={`injector-field-description-${index}`}
                value={field.description}
                onChange={(e) =>
                  updateField(index, { description: e.target.value })
                }
                placeholder="Field description (optional)…"
                aria-label={`Field ${index + 1} description`}
                className="text-sm"
              />

              <DefaultValueInput
                fieldType={field.field_type}
                value={field.default_value}
                onChange={(value) =>
                  updateField(index, { default_value: value })
                }
                hasError={!field.allow_overwrite && isEmptyInjectorFieldDefault(field)}
                inputTestId={`injector-field-default-${index}`}
                errorTestId={`injector-field-default-error-${index}`}
                ariaLabel={`Default value for field ${field.field_name || index + 1}`}
              />
            </InjectorFieldCard>
          ))}

          <Button
            data-testid="injector-add-field"
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setFields((prev) => [...prev, emptyInjectorFormField()])}
            className="gap-2 w-fit"
          >
            <Plus className="h-4 w-4" />
            Add Field
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function fieldTypeLabel(fieldType: InjectorFieldType): string {
  return FIELD_TYPES.find((item) => item.value === fieldType)?.label ?? fieldType;
}
