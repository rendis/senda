"use client";

import { useState } from "react";
import {
  CircleAlert,
  Eye,
  Image as ImageIcon,
  LockKeyhole,
  PencilLine,
  CodeXml,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { InjectorFieldType } from "@/types/injectors";

export const LOCKED_DEFAULT_ERROR =
  "Locked fields require a non-empty default value.";
const ACTIVE_SEGMENT_CLASSES =
  "border border-primary/30 bg-primary text-primary-foreground shadow-sm";
const INACTIVE_SEGMENT_CLASSES =
  "text-muted-foreground hover:bg-background hover:text-foreground";
const PREVIEW_BUTTON_CLASSES =
  "shrink-0 rounded-lg border-transparent text-muted-foreground hover:border-border/70 hover:bg-background hover:text-foreground";

interface DefaultValueInputProps {
  fieldType: InjectorFieldType;
  value: string;
  onChange: (value: string) => void;
  hasError?: boolean;
  inputTestId: string;
  errorTestId?: string;
  ariaLabel?: string;
}

export function DefaultValueInput({
  fieldType,
  value,
  onChange,
  hasError = false,
  inputTestId,
  errorTestId,
  ariaLabel = "Default value",
}: DefaultValueInputProps) {
  if (fieldType === "bool") {
    return (
      <BoolValueToggle
        checked={value === "true"}
        onChange={(next) => onChange(String(next))}
        testId={inputTestId}
        ariaLabel={ariaLabel}
      />
    );
  }

  if (fieldType === "html") {
    return (
      <HtmlValueInput
        value={value}
        onChange={onChange}
        hasError={hasError}
        inputTestId={inputTestId}
        errorTestId={errorTestId}
        ariaLabel={ariaLabel}
      />
    );
  }

  if (fieldType === "img") {
    return (
      <ImageUrlValueInput
        value={value}
        onChange={onChange}
        hasError={hasError}
        inputTestId={inputTestId}
        errorTestId={errorTestId}
        ariaLabel={ariaLabel}
      />
    );
  }

  return (
    <div className="relative">
      <Input
        data-testid={inputTestId}
        type={fieldType === "number" ? "number" : "text"}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={getPlaceholder(fieldType)}
        className={cn(
          "font-mono text-[13px]",
          hasError && "border-destructive pr-10 focus-visible:ring-destructive/20"
        )}
        aria-invalid={hasError}
        aria-label={ariaLabel}
      />
      {hasError && errorTestId ? (
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                data-testid={errorTestId}
                tabIndex={-1}
                aria-label={LOCKED_DEFAULT_ERROR}
                className="absolute right-2 top-2 inline-flex size-5 items-center justify-center rounded-full text-destructive"
              >
                <CircleAlert className="size-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="left">{LOCKED_DEFAULT_ERROR}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      ) : null}
    </div>
  );
}

function BoolValueToggle({
  checked,
  onChange,
  testId,
  ariaLabel,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  testId: string;
  ariaLabel: string;
}) {
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      data-testid={testId}
      className="grid h-9 grid-cols-2 rounded-xl border bg-muted/50 p-1 shadow-xs"
    >
      <button
        type="button"
        aria-pressed={checked}
        onClick={() => onChange(true)}
        className={cn(
          "inline-flex h-7 min-w-18 items-center justify-center rounded-lg border border-transparent px-3 text-sm font-medium transition-colors",
          checked
            ? ACTIVE_SEGMENT_CLASSES
            : INACTIVE_SEGMENT_CLASSES
        )}
      >
        True
      </button>
      <button
        type="button"
        aria-pressed={!checked}
        onClick={() => onChange(false)}
        className={cn(
          "inline-flex h-7 min-w-18 items-center justify-center rounded-lg border border-transparent px-3 text-sm font-medium transition-colors",
          !checked
            ? ACTIVE_SEGMENT_CLASSES
            : INACTIVE_SEGMENT_CLASSES
        )}
      >
        False
      </button>
    </div>
  );
}

function HtmlValueInput({
  value,
  onChange,
  hasError,
  inputTestId,
  errorTestId,
  ariaLabel,
}: Omit<DefaultValueInputProps, "fieldType">) {
  const [open, setOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<"source" | "preview">("preview");

  return (
    <>
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Input
            data-testid={inputTestId}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder={getPlaceholder("html")}
            className={cn(
              "font-mono text-[13px]",
              hasError && "border-destructive pr-10 focus-visible:ring-destructive/20"
            )}
            aria-invalid={hasError}
            aria-label={ariaLabel}
          />
          {hasError && errorTestId ? (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    data-testid={errorTestId}
                    tabIndex={-1}
                    aria-label={LOCKED_DEFAULT_ERROR}
                    className="absolute right-2 top-2 inline-flex size-5 items-center justify-center rounded-full text-destructive"
                  >
                    <CircleAlert className="size-4" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="left">{LOCKED_DEFAULT_ERROR}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          ) : null}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid={`${inputTestId}-preview`}
          className={PREVIEW_BUTTON_CLASSES}
          onClick={() => {
            setActiveTab("preview");
            setOpen(true);
          }}
        >
          <CodeXml className="size-4" />
          Preview
        </Button>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-5xl">
          <DialogHeader>
            <DialogTitle>HTML preview</DialogTitle>
            <DialogDescription>
              Switch between source and preview so each view can use the full modal width.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div
              className="inline-flex w-fit items-center gap-1 rounded-lg border border-border/70 bg-muted/20 p-1"
              role="tablist"
              aria-label="HTML preview tabs"
            >
              <button
                type="button"
                role="tab"
                aria-selected={activeTab === "source"}
                data-testid={`${inputTestId}-tab-source`}
                className={cn(
                  "inline-flex h-8 items-center justify-center rounded-md px-3 text-sm font-medium transition-colors",
                  activeTab === "source"
                    ? ACTIVE_SEGMENT_CLASSES
                    : INACTIVE_SEGMENT_CLASSES
                )}
                onClick={() => setActiveTab("source")}
              >
                HTML
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeTab === "preview"}
                data-testid={`${inputTestId}-tab-preview`}
                className={cn(
                  "inline-flex h-8 items-center justify-center rounded-md px-3 text-sm font-medium transition-colors",
                  activeTab === "preview"
                    ? ACTIVE_SEGMENT_CLASSES
                    : INACTIVE_SEGMENT_CLASSES
                )}
                onClick={() => setActiveTab("preview")}
              >
                Preview
              </button>
            </div>

            {activeTab === "source" ? (
              <div className="flex flex-col gap-2">
                <p className="text-xs font-medium text-muted-foreground">HTML source</p>
                <textarea
                  value={value}
                  onChange={(e) => onChange(e.target.value)}
                  className="min-h-[420px] w-full rounded-md border bg-background px-3 py-2 font-mono text-sm outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
                  spellCheck={false}
                />
              </div>
            ) : (
              <div className="flex flex-col gap-2">
                <p className="text-xs font-medium text-muted-foreground">Rendered preview</p>
                <iframe
                  title="HTML preview"
                  srcDoc={value || "<div></div>"}
                  className="min-h-[420px] w-full rounded-md border bg-white"
                />
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

function ImageUrlValueInput({
  value,
  onChange,
  hasError,
  inputTestId,
  errorTestId,
  ariaLabel,
}: Omit<DefaultValueInputProps, "fieldType">) {
  const [open, setOpen] = useState(false);
  const [imageError, setImageError] = useState(false);
  const [draftValue, setDraftValue] = useState(value);

  return (
    <>
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Input
            data-testid={inputTestId}
            value={value}
            onChange={(e) => {
              setImageError(false);
              onChange(e.target.value);
            }}
            placeholder={getPlaceholder("img")}
            className={cn(
              "font-mono text-[13px]",
              hasError && "border-destructive pr-10 focus-visible:ring-destructive/20"
            )}
            aria-invalid={hasError}
            aria-label={ariaLabel}
          />
          {hasError && errorTestId ? (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    data-testid={errorTestId}
                    tabIndex={-1}
                    aria-label={LOCKED_DEFAULT_ERROR}
                    className="absolute right-2 top-2 inline-flex size-5 items-center justify-center rounded-full text-destructive"
                  >
                    <CircleAlert className="size-4" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="left">{LOCKED_DEFAULT_ERROR}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          ) : null}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid={`${inputTestId}-preview`}
          className={PREVIEW_BUTTON_CLASSES}
          onClick={() => {
            setDraftValue(value);
            setImageError(false);
            setOpen(true);
          }}
        >
          <Eye className="size-4" />
          Preview
        </Button>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>Image preview</DialogTitle>
            <DialogDescription>
              Edit the image URL here and review the preview before applying it to the field.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <Input
              data-testid={`${inputTestId}-preview-url`}
              value={draftValue}
              onChange={(e) => {
                setImageError(false);
                setDraftValue(e.target.value);
              }}
              placeholder={getPlaceholder("img")}
              className="font-mono text-[13px]"
              aria-label={`${ariaLabel} preview URL`}
            />
            <div className="flex min-h-[320px] items-center justify-center rounded-md border bg-muted/20 p-4">
              {draftValue && !imageError ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={draftValue}
                  alt="Injector default preview"
                  className="max-h-[420px] max-w-full rounded-md object-contain"
                  onError={() => setImageError(true)}
                />
              ) : (
                <div className="flex flex-col items-center gap-2 text-sm text-muted-foreground">
                  <ImageIcon className="size-8" />
                  <span>
                    {draftValue
                      ? "We couldn't load this image URL."
                      : "Add an image URL to preview it here."}
                  </span>
                </div>
              )}
            </div>
          </div>
          <DialogFooter showCloseButton>
            <Button
              type="button"
              onClick={() => {
                onChange(draftValue);
                setOpen(false);
              }}
            >
              Use this URL
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

interface OverwriteModeToggleProps {
  allowOverwrite: boolean;
  onChange: (value: boolean) => void;
  testIdPrefix: string;
}

export function OverwriteModeToggle({
  allowOverwrite,
  onChange,
  testIdPrefix,
}: OverwriteModeToggleProps) {
  return (
    <TooltipProvider>
      <div
        className="inline-flex items-center gap-0.5 rounded-lg border border-border/70 bg-background/80 p-0.5"
        role="group"
        aria-label="Runtime overwrite mode"
      >
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              data-testid={`${testIdPrefix}-allow`}
              aria-pressed={allowOverwrite}
              aria-label="Allow runtime overwrite"
              className={cn(
                "rounded-md border border-transparent transition-colors",
                allowOverwrite
                  ? "border-primary/25 bg-primary text-primary-foreground hover:bg-primary/95"
                  : "text-muted-foreground hover:bg-background hover:text-foreground"
              )}
              onClick={() => onChange(true)}
            >
              <PencilLine className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">
            Runtime uses reqBody, then code, then default.
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              data-testid={`${testIdPrefix}-locked`}
              aria-pressed={!allowOverwrite}
              aria-label="Always use default value"
              className={cn(
                "rounded-md border border-transparent transition-colors",
                !allowOverwrite
                  ? "border-amber-300/80 bg-amber-100 text-amber-950 hover:bg-amber-100 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-200"
                  : "text-muted-foreground hover:bg-background hover:text-foreground"
              )}
              onClick={() => onChange(false)}
            >
              <LockKeyhole className="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">
            Always uses the default value.
          </TooltipContent>
        </Tooltip>
      </div>
    </TooltipProvider>
  );
}

function getPlaceholder(fieldType: InjectorFieldType): string {
  switch (fieldType) {
    case "img":
      return "https://cdn.example.com/logo.png";
    case "url":
      return "https://example.com";
    case "html":
      return "<p>Footer</p>";
    case "number":
      return "0";
    case "bool":
      return "false";
    default:
      return "Default value...";
  }
}
