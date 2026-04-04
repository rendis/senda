"use client";

import { useMemo, useState } from "react";
import { z } from "zod";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useTemplateBulkSend, useTemplateBulkSendConfig } from "@/hooks/use-template-version";
import type {
  TemplateBulkSendItem,
  TemplateBulkSendRequest,
  TemplateBulkSendResponse,
} from "@/types/templates";
import { toast } from "sonner";

const bulkSendItemSchema = z.object({
  to: z.string().email("to must be a valid email address"),
  cc: z.array(z.string().email("cc entries must be valid emails")).optional(),
  bcc: z.array(z.string().email("bcc entries must be valid emails")).optional(),
  variables: z.record(z.string(), z.unknown()).optional(),
  external_id: z.string().optional(),
  locale: z.string().optional(),
});

const bulkSendPayloadSchema = z.object({
  items: z.array(bulkSendItemSchema).min(1, "items must contain at least 1 entry"),
});

interface BulkSendModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  scopedPath: string;
  templateId: string;
  enabled?: boolean;
}

interface BulkSendModalBodyProps {
  scopedPath: string;
  templateId: string;
  enabled: boolean;
  onOpenChange: (open: boolean) => void;
}

type ParsedBulkSendState = {
  payload: TemplateBulkSendRequest | null;
  errors: string[];
};

function parseBulkSendPayload(raw: string, maxItems: number): ParsedBulkSendState {
  if (!raw.trim()) {
    return { payload: null, errors: [] };
  }

  let decoded: unknown;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return { payload: null, errors: ["The file must contain valid JSON."] };
  }

  const parsed = bulkSendPayloadSchema.safeParse(decoded);
  if (!parsed.success) {
    return {
      payload: null,
      errors: parsed.error.issues.map((issue) => {
        const path = issue.path.length > 0 ? issue.path.join(".") : "items";
        return `${path}: ${issue.message}`;
      }),
    };
  }

  if (parsed.data.items.length > maxItems) {
    return {
      payload: null,
      errors: [`items: must contain at most ${maxItems} entries`],
    };
  }

  return { payload: parsed.data, errors: [] };
}

export function BulkSendModal({
  open,
  onOpenChange,
  scopedPath,
  templateId,
  enabled = true,
}: BulkSendModalProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open ? (
        <BulkSendModalBody
          scopedPath={scopedPath}
          templateId={templateId}
          enabled={enabled}
          onOpenChange={onOpenChange}
        />
      ) : null}
    </Dialog>
  );
}

function BulkSendModalBody({
  scopedPath,
  templateId,
  enabled,
  onOpenChange,
}: BulkSendModalBodyProps) {
  const bulkSend = useTemplateBulkSend(scopedPath, templateId);
  const configQuery = useTemplateBulkSendConfig(scopedPath, templateId, enabled);
  const maxItems = configQuery.data?.max_items ?? 100;

  const [fileName, setFileName] = useState("");
  const [rawJson, setRawJson] = useState("");
  const [result, setResult] = useState<TemplateBulkSendResponse | null>(null);

  const parsed = useMemo(() => parseBulkSendPayload(rawJson, maxItems), [rawJson, maxItems]);

  async function handleFileChange(file: File | null) {
    if (!file) {
      setFileName("");
      setRawJson("");
      setResult(null);
      return;
    }

    const text = await file.text();
    setFileName(file.name);
    setRawJson(text);
    setResult(null);
  }

  async function handleConfirm() {
    if (!parsed.payload || parsed.errors.length > 0) {
      toast.error("Upload a valid JSON file before enqueuing.");
      return;
    }

    try {
      const response = await bulkSend.mutateAsync(parsed.payload);
      setResult(response);
      if (response.status === "accepted") {
        toast.success(`Queued ${response.accepted_count} messages.`);
      } else if (response.status === "partial") {
        toast.warning("Bulk send queued partially. Review the item results.");
      } else {
        toast.error("Bulk send failed for every item.");
      }
    } catch {
      toast.error("Failed to enqueue bulk send.");
    }
  }

  const previewItems: TemplateBulkSendItem[] = parsed.payload?.items.slice(0, 5) ?? [];
  const locales = Array.from(
    new Set(parsed.payload?.items.map((item) => item.locale).filter(Boolean) as string[])
  );
  const externalIDCount =
    parsed.payload?.items.filter((item) => Boolean(item.external_id?.trim())).length ?? 0;

  return (
    <DialogContent
      className="max-h-[90vh] max-w-3xl overflow-hidden"
      onInteractOutside={(event) => bulkSend.isPending && event.preventDefault()}
    >
      <DialogHeader>
        <DialogTitle>Bulk Send</DialogTitle>
        <DialogDescription>
          Uses the current published version. Upload a JSON file containing only{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-xs">items[]</code>, review the
          preview, and confirm to enqueue the messages.
        </DialogDescription>
      </DialogHeader>

      <div className="flex flex-col gap-4 overflow-hidden">
        <Card className="space-y-3 p-4">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">Published version</Badge>
            <Badge variant="outline">Max {maxItems} items</Badge>
            <Badge variant="outline">items[] only</Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            This flow is for real queued sends, not template verification. Use <strong>Send
            Test</strong> for one-off rendering checks.
          </p>
          <pre className="overflow-x-auto rounded-md bg-muted p-3 text-xs">
{`{
  "items": [
    {
      "to": "ana@example.com",
      "variables": { "name": "Ana" },
      "external_id": "msg-1",
      "locale": "es"
    }
  ]
}`}
          </pre>
        </Card>

        <div className="space-y-2">
          <Input
            type="file"
            accept="application/json,.json"
            onChange={(event) => void handleFileChange(event.target.files?.[0] ?? null)}
            disabled={bulkSend.isPending || !enabled}
          />
          {fileName ? (
            <p className="text-xs text-muted-foreground">Loaded file: {fileName}</p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Upload one JSON file. The server will enforce the same batch limit used by the API.
            </p>
          )}
        </div>

        {configQuery.isLoading && (
          <p className="text-xs text-muted-foreground">Loading server-side bulk send limits…</p>
        )}

        {parsed.errors.length > 0 && (
          <Card className="border-destructive/40 p-4">
            <div className="space-y-2">
              <p className="text-sm font-medium text-destructive">
                Fix these issues before continuing:
              </p>
              <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
                {parsed.errors.map((error) => (
                  <li key={error}>{error}</li>
                ))}
              </ul>
            </div>
          </Card>
        )}

        {parsed.payload && parsed.errors.length === 0 && (
          <Card className="space-y-4 p-4">
            <div className="flex flex-wrap items-center gap-3">
              <Badge>{parsed.payload.items.length} items</Badge>
              <Badge variant="secondary">{externalIDCount} external IDs</Badge>
              <Badge variant="outline">
                {locales.length > 0 ? `Locales: ${locales.join(", ")}` : "Locales: default"}
              </Badge>
            </div>
            <Separator />
            <div className="space-y-2">
              <p className="text-sm font-medium">Preview</p>
              <ScrollArea className="max-h-56 rounded-md border">
                <div className="divide-y">
                  {previewItems.map((item, index) => (
                    <div key={`${item.to}-${index}`} className="space-y-1 p-3 text-sm">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium">{item.to}</span>
                        {item.locale ? <Badge variant="outline">{item.locale}</Badge> : null}
                        {item.external_id ? (
                          <Badge variant="secondary">{item.external_id}</Badge>
                        ) : null}
                      </div>
                      <p className="text-xs text-muted-foreground">
                        Variables: {Object.keys(item.variables ?? {}).length}
                        {item.cc?.length ? ` · CC ${item.cc.length}` : ""}
                        {item.bcc?.length ? ` · BCC ${item.bcc.length}` : ""}
                      </p>
                    </div>
                  ))}
                </div>
              </ScrollArea>
              {parsed.payload.items.length > previewItems.length && (
                <p className="text-xs text-muted-foreground">
                  Showing {previewItems.length} of {parsed.payload.items.length} items.
                </p>
              )}
            </div>
          </Card>
        )}

        {result && (
          <Card className="space-y-4 p-4">
            <div className="flex flex-wrap items-center gap-2">
              <Badge>{result.status}</Badge>
              <Badge variant="secondary">{result.accepted_count} accepted</Badge>
              <Badge variant="outline">{result.suppressed_count} suppressed</Badge>
              <Badge variant="destructive">{result.failed_count} failed</Badge>
            </div>
            <ScrollArea className="max-h-56 rounded-md border">
              <div className="divide-y">
                {result.items.map((item) => (
                  <div key={`${item.index}-${item.to}`} className="space-y-1 p-3 text-sm">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{item.to}</span>
                      <Badge variant={item.status === "failed" ? "destructive" : "outline"}>
                        {item.status}
                      </Badge>
                      {item.tracking_id ? (
                        <code className="rounded bg-muted px-1 py-0.5 text-xs">
                          {item.tracking_id}
                        </code>
                      ) : null}
                    </div>
                    {item.error ? (
                      <p className="text-xs text-muted-foreground">{item.error}</p>
                    ) : null}
                  </div>
                ))}
              </div>
            </ScrollArea>
          </Card>
        )}

        {!result && parsed.payload && parsed.errors.length === 0 && (
          <p className="rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-900 dark:text-amber-200">
            This will queue real emails using the current published version.
          </p>
        )}
      </div>

      <DialogFooter>
        <Button
          variant="outline"
          onClick={() => onOpenChange(false)}
          disabled={bulkSend.isPending}
        >
          {result ? "Close" : "Cancel"}
        </Button>
        {!result ? (
          <Button
            onClick={() => void handleConfirm()}
            disabled={
              bulkSend.isPending ||
              parsed.payload == null ||
              parsed.errors.length > 0 ||
              configQuery.isLoading
            }
          >
            {bulkSend.isPending ? "Queueing..." : "Confirm & Queue"}
          </Button>
        ) : null}
      </DialogFooter>
    </DialogContent>
  );
}
