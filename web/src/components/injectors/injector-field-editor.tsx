"use client";

import { useMemo, useState } from "react";
import { Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { InjectorFieldCard } from "@/components/injectors/injector-field-card";
import {
  DefaultValueInput,
  OverwriteModeToggle,
} from "@/components/injectors/field-runtime-controls";
import type { InjectorField } from "@/types/injectors";

interface InjectorFieldEditorProps {
  field: InjectorField;
  onSave: (
    fieldName: string,
    data: { default_value?: unknown; allow_overwrite: boolean }
  ) => void;
  saving?: boolean;
}

export function InjectorFieldEditor({
  field,
  onSave,
  saving = false,
}: InjectorFieldEditorProps) {
  const initialSerialized = useMemo(
    () => serializeFieldValue(field.field_type, field.default_value),
    [field.default_value, field.field_type]
  );
  const [defaultValue, setDefaultValue] = useState(initialSerialized);
  const [allowOverwrite, setAllowOverwrite] = useState(field.allow_overwrite);

  const dirty =
    defaultValue !== initialSerialized || allowOverwrite !== field.allow_overwrite;

  const saveDisabled =
    saving ||
    !dirty ||
    (!allowOverwrite && isEmptyDefault(defaultValue, field.field_type));

  function handleSave() {
    onSave(field.field_name, {
      allow_overwrite: allowOverwrite,
      default_value: parseFieldValue(field.field_type, defaultValue),
    });
  }

  return (
    <InjectorFieldCard
      testId={`injector-field-editor-row-${field.field_name}`}
      headerTestId={`injector-field-editor-header-${field.field_name}`}
      title={field.field_name}
      typeLabel={formatFieldType(field.field_type)}
      actions={
        <OverwriteModeToggle
          allowOverwrite={allowOverwrite}
          onChange={setAllowOverwrite}
          testIdPrefix={`injector-field-editor-overwrite-${field.field_name}`}
        />
      }
    >
      <div className="flex flex-col gap-4">
        <div className="text-xs text-muted-foreground">
          {field.description || "No description provided for this field yet."}
        </div>

        <div className="flex flex-col gap-4 lg:flex-row lg:items-center">
          <DefaultValueInput
            fieldType={field.field_type}
            value={defaultValue}
            onChange={setDefaultValue}
            hasError={!allowOverwrite && isEmptyDefault(defaultValue, field.field_type)}
            inputTestId={`injector-field-editor-default-${field.field_name}`}
            errorTestId={`injector-field-editor-default-error-${field.field_name}`}
            ariaLabel={`Default value for field ${field.field_name}`}
          />
        </div>

        <div className="flex justify-start lg:justify-end">
          <Button onClick={handleSave} disabled={saveDisabled} className="gap-2 w-fit">
            <Save className="h-4 w-4" />
            Save field
          </Button>
        </div>
      </div>
    </InjectorFieldCard>
  );
}

function serializeFieldValue(fieldType: InjectorField["field_type"], value: unknown): string {
  if (value == null) {
    return fieldType === "bool" ? "false" : "";
  }
  if (fieldType === "bool") {
    return String(Boolean(value));
  }
  return String(value);
}

function parseFieldValue(fieldType: InjectorField["field_type"], value: string): unknown {
  if (fieldType === "bool") {
    return value === "true";
  }
  if (fieldType === "number") {
    return value === "" ? undefined : Number(value);
  }
  return value === "" ? undefined : value;
}

function isEmptyDefault(value: string, fieldType: InjectorField["field_type"]): boolean {
  if (fieldType === "bool") {
    return false;
  }
  return value.trim() === "";
}

function formatFieldType(fieldType: InjectorField["field_type"]): string {
  switch (fieldType) {
    case "bool":
      return "Boolean";
    case "img":
      return "Image URL";
    case "url":
      return "URL";
    case "html":
      return "HTML";
    case "number":
      return "Number";
    default:
      return "Text";
  }
}
