"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Loader2 } from "lucide-react";
import { useCallback, useRef, useState } from "react";
import { shouldRestoreDialogFocus } from "@/components/shared/form-dialog-focus";

interface FormDialogProps {
  trigger?: React.ReactNode;
  title: string;
  description?: string;
  submitLabel?: string;
  loadingLabel?: string;
  submitIcon?: React.ReactNode;
  children: React.ReactNode;
  onSubmit: () => Promise<boolean | void> | boolean | void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  submitDisabled?: boolean;
}

export function FormDialog({
  trigger,
  title,
  description,
  submitLabel = "Save",
  children,
  onSubmit,
  open: controlledOpen,
  onOpenChange: controlledOnOpenChange,
  submitDisabled,
  loadingLabel = "...",
  submitIcon,
}: FormDialogProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const previousFocusedRef = useRef<HTMLElement | null>(null);

  const open = controlledOpen ?? internalOpen;
  const onOpenChange = controlledOnOpenChange ?? setInternalOpen;

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (nextOpen && typeof document !== "undefined") {
        previousFocusedRef.current =
          document.activeElement instanceof HTMLElement
            ? document.activeElement
            : null;
      }

      onOpenChange(nextOpen);
    },
    [onOpenChange],
  );

  const handleCloseAutoFocus = useCallback(
    (event: Event) => {
      event.preventDefault();

      if (typeof document === "undefined") {
        previousFocusedRef.current = null;
        return;
      }

      const previousFocused = previousFocusedRef.current;
      previousFocusedRef.current = null;

      requestAnimationFrame(() => {
        if (
          previousFocused &&
          shouldRestoreDialogFocus({
            isConnected: previousFocused.isConnected,
            disabled: previousFocused.hasAttribute("disabled"),
            ariaHidden: previousFocused.getAttribute("aria-hidden"),
          })
        ) {
          previousFocused.focus();
          return;
        }

        if (document.activeElement instanceof HTMLElement) {
          document.activeElement.blur();
        }
      });
    },
    [],
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const keepOpen = await onSubmit();
      if (keepOpen !== true) onOpenChange(false);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {trigger ? <DialogTrigger asChild>{trigger}</DialogTrigger> : null}
      <DialogContent
        className="max-h-[85vh] flex flex-col"
        onCloseAutoFocus={handleCloseAutoFocus}
      >
        <form onSubmit={handleSubmit} className="flex flex-col min-h-0">
          <DialogHeader className="shrink-0">
            <DialogTitle>{title}</DialogTitle>
            {description && (
              <DialogDescription>{description}</DialogDescription>
            )}
          </DialogHeader>
          <div className="py-4 -mx-6 px-6 overflow-y-auto min-h-0 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-muted-foreground/15 hover:[&::-webkit-scrollbar-thumb]:bg-muted-foreground/30 [&::-webkit-scrollbar-thumb]:rounded-full"><fieldset disabled={loading} className="min-w-0">{children}</fieldset></div>
          <DialogFooter className="shrink-0 pt-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={loading || submitDisabled} className="gap-2">
              {loading ? (
                <><Loader2 className="h-4 w-4 animate-spin" />{loadingLabel}</>
              ) : (
                <>{submitIcon}{submitLabel}</>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
