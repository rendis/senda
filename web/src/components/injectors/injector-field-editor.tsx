"use client";

import { useState } from "react";
import { Save, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ResolutionChainViewer } from "./resolution-chain-viewer";
import type { InjectorField, InjectorFieldResolution } from "@/types/injectors";
import { SYSTEM_WORKSPACE_CODE, type ScopeLevel } from "@/types/api";

interface InjectorFieldEditorProps {
  field: InjectorField;
  resolution: InjectorFieldResolution | null;
  currentScope: ScopeLevel;
  onSave: (fieldName: string, value: unknown) => void;
  onDeleteOverride: (fieldName: string) => void;
  saving?: boolean;
}

const emptyResolution: InjectorFieldResolution = {
  field_name: "",
  global_level: null,
  tenant_level: null,
  workspace_level: null,
  effective_value: null,
};

export function InjectorFieldEditor({
  field,
  resolution: rawResolution,
  currentScope,
  onSave,
  onDeleteOverride,
  saving = false,
}: InjectorFieldEditorProps) {
  const resolution = rawResolution ?? emptyResolution;
  const workspaceValue = resolution.workspace_level;
  const hasOverride = workspaceValue != null;
  const [localValue, setLocalValue] = useState<string>(
    hasOverride ? String(workspaceValue) : ""
  );
  const [editing, setEditing] = useState(false);

  const isEditable = currentScope === "workspace";

  const chainLevels = [
    {
      scope: "global" as const,
      value: resolution.global_level,
    },
    {
      scope: "system" as const,
      value: resolution.tenant_level,
      inherited: resolution.tenant_level == null,
    },
    {
      scope: "workspace" as const,
      value: resolution.workspace_level,
      inherited: resolution.workspace_level == null,
    },
  ];

  const effectiveSource =
    resolution.workspace_level != null
      ? "Workspace"
      : resolution.tenant_level != null
        ? SYSTEM_WORKSPACE_CODE
        : "Global";

  function renderWorkspaceInput() {
    if (!isEditable) return null;

    if (field.field_type === "bool") {
      const checked = localValue === "true";
      return (
        <button
          type="button"
          role="switch"
          aria-checked={checked}
          onClick={() => {
            const next = !checked;
            setLocalValue(String(next));
            setEditing(true);
          }}
          className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
            checked ? "bg-primary" : "bg-muted"
          }`}
        >
          <span
            className={`pointer-events-none block h-5 w-5 rounded-full bg-white shadow-lg transition-transform ${
              checked ? "translate-x-5" : "translate-x-0"
            }`}
          />
        </button>
      );
    }

    const placeholder = getPlaceholder(field.field_type);
    const inputType = field.field_type === "number" ? "number" : "text";

    return (
      <Input
        type={inputType}
        value={localValue}
        onChange={(e) => {
          setLocalValue(e.target.value);
          setEditing(true);
        }}
        placeholder={placeholder}
        className="w-[280px] font-mono text-[13px] h-9"
      />
    );
  }

  return (
    <div className="rounded-lg border bg-card p-5 flex flex-col gap-3">
      {/* Header: field name + type */}
      <div className="flex items-center justify-between">
        <span className="font-mono text-sm font-semibold text-foreground">
          {field.field_name}
        </span>
        <span className="font-mono text-xs text-muted-foreground">
          type: {field.field_type}
        </span>
      </div>

      {/* Resolution chain */}
      <div className="flex flex-col gap-2">
        {chainLevels.slice(0, currentScope === "global" ? 1 : currentScope === "tenant" ? 2 : 3).map((level) => (
          <div key={level.scope} className="flex items-center gap-2">
            <ResolutionChainViewer levels={[level]} />
            {level.scope === "workspace" && isEditable && (
              <div className="flex items-center gap-2">
                {renderWorkspaceInput()}
                {editing && (
                  <Button
                    size="sm"
                    onClick={() => {
                      const val =
                        field.field_type === "number"
                          ? Number(localValue)
                          : field.field_type === "bool"
                            ? localValue === "true"
                            : localValue;
                      onSave(field.field_name, val);
                      setEditing(false);
                    }}
                    disabled={saving}
                    className="gap-1"
                  >
                    <Save className="h-4 w-4" />
                    Save
                  </Button>
                )}
                {hasOverride && !editing && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onDeleteOverride(field.field_name)}
                    disabled={saving}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Effective value */}
      <div className="flex items-center gap-2 border-t pt-2">
        <span className="text-xs font-medium text-muted-foreground">
          Effective value:
        </span>
        <span className="font-mono text-xs text-primary">
          {resolution.effective_value != null
            ? `${String(resolution.effective_value)} (from ${effectiveSource})`
            : "\u2014"}
        </span>
      </div>
    </div>
  );
}

function getPlaceholder(fieldType: string): string {
  switch (fieldType) {
    case "img":
      return "Image URL...";
    case "url":
      return "https://...";
    case "html":
      return "HTML content...";
    case "number":
      return "0";
    default:
      return "Value...";
  }
}
