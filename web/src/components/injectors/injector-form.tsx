"use client";

import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FormDialog } from "@/components/shared/form-dialog";
import type {
  InjectorFieldType,
  CreateInjectorRequest,
} from "@/types/injectors";

interface FieldEntry {
  field_name: string;
  field_type: InjectorFieldType;
  description: string;
}

const FIELD_TYPES: { value: InjectorFieldType; label: string }[] = [
  { value: "text", label: "Text" },
  { value: "number", label: "Number" },
  { value: "bool", label: "Boolean" },
  { value: "img", label: "Image URL" },
  { value: "url", label: "URL" },
  { value: "html", label: "HTML" },
];

function emptyField(): FieldEntry {
  return { field_name: "", field_type: "text", description: "" };
}

interface InjectorFormProps {
  trigger: React.ReactNode;
  onSubmit: (data: CreateInjectorRequest) => Promise<void>;
  title?: string;
}

export function InjectorForm({
  trigger,
  onSubmit,
  title = "New Injector",
}: InjectorFormProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [fields, setFields] = useState<FieldEntry[]>([emptyField()]);

  function resetForm() {
    setName("");
    setDescription("");
    setFields([emptyField()]);
  }

  function updateField(index: number, patch: Partial<FieldEntry>) {
    setFields((prev) =>
      prev.map((f, i) => (i === index ? { ...f, ...patch } : f))
    );
  }

  function removeField(index: number) {
    setFields((prev) => prev.filter((_, i) => i !== index));
  }

  async function handleSubmit() {
    await onSubmit({
      name,
      description: description || undefined,
      fields: fields.map((f, i) => ({
        field_name: f.field_name,
        field_type: f.field_type,
        description: f.description || undefined,
        position: i,
      })),
    });
    resetForm();
  }

  return (
    <FormDialog
      trigger={trigger}
      title={title}
      description="Define a new injector with its fields."
      submitLabel="Create"
      onSubmit={handleSubmit}
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="injector-name">Name</Label>
          <Input
            id="injector-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="company_info"
            className="font-mono"
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="injector-description">Description</Label>
          <Input
            id="injector-description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description..."
          />
        </div>

        <div className="border-t pt-4 flex flex-col gap-3">
          <p className="text-xs font-medium text-muted-foreground">Fields</p>

          {fields.map((field, index) => (
            <div
              key={index}
              className="flex items-start gap-2 rounded-md border p-3"
            >
              <div className="flex flex-1 flex-col gap-2">
                <div className="flex gap-2">
                  <Input
                    value={field.field_name}
                    onChange={(e) =>
                      updateField(index, { field_name: e.target.value })
                    }
                    placeholder="field_name"
                    className="font-mono flex-1"
                  />
                  <Select
                    value={field.field_type}
                    onValueChange={(v) =>
                      updateField(index, {
                        field_type: v as InjectorFieldType,
                      })
                    }
                  >
                    <SelectTrigger className="w-[130px]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {FIELD_TYPES.map((ft) => (
                        <SelectItem key={ft.value} value={ft.value}>
                          {ft.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <Input
                  value={field.description}
                  onChange={(e) =>
                    updateField(index, { description: e.target.value })
                  }
                  placeholder="Field description (optional)"
                  className="text-xs"
                />
              </div>
              {fields.length > 1 && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => removeField(index)}
                  className="h-8 w-8 p-0 shrink-0 mt-0.5"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              )}
            </div>
          ))}

          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setFields((prev) => [...prev, emptyField()])}
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
