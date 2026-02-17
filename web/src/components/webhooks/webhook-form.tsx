"use client";

import { useState } from "react";
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
import { Label } from "@/components/ui/label";
import { WebhookEventPicker } from "./webhook-event-picker";
import { SecretRevealField } from "./secret-reveal-field";
import type { Webhook, WebhookEventType } from "@/types/webhooks";

interface WebhookFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  webhook?: Webhook;
  createdSecret?: string;
  onSubmit: (data: { url: string; events: WebhookEventType[] }) => Promise<void>;
}

export function WebhookForm({
  open,
  onOpenChange,
  webhook,
  createdSecret,
  onSubmit,
}: WebhookFormProps) {
  const [url, setUrl] = useState(webhook?.url ?? "");
  const [events, setEvents] = useState<WebhookEventType[]>(
    webhook?.events ?? []
  );
  const [loading, setLoading] = useState(false);
  const [urlError, setUrlError] = useState("");

  const isEdit = !!webhook;

  const validate = (): boolean => {
    if (!url.startsWith("https://")) {
      setUrlError("URL must start with https://");
      return false;
    }
    try {
      new URL(url);
    } catch {
      setUrlError("Invalid URL format");
      return false;
    }
    setUrlError("");
    return true;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    if (events.length === 0) return;
    setLoading(true);
    try {
      await onSubmit({ url, events });
    } finally {
      setLoading(false);
    }
  };

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setUrl(webhook?.url ?? "");
      setEvents(webhook?.events ?? []);
      setUrlError("");
    }
    onOpenChange(next);
  };

  if (createdSecret) {
    return (
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Webhook Created</DialogTitle>
            <DialogDescription>
              Save this secret now. It will not be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <SecretRevealField value={createdSecret} />
          </div>
          <DialogFooter>
            <Button onClick={() => handleOpenChange(false)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>
              {isEdit ? "Edit Webhook" : "New Webhook"}
            </DialogTitle>
            <DialogDescription>
              {isEdit
                ? "Update the webhook URL or subscribed events."
                : "Configure a webhook endpoint to receive event notifications."}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="webhook-url">URL (HTTPS required)</Label>
              <Input
                id="webhook-url"
                placeholder="https://api.example.com/webhooks"
                value={url}
                onChange={(e) => {
                  setUrl(e.target.value);
                  if (urlError) setUrlError("");
                }}
              />
              {urlError && (
                <p className="text-xs text-destructive">{urlError}</p>
              )}
            </div>
            <WebhookEventPicker value={events} onChange={setEvents} />
            {events.length === 0 && (
              <p className="text-xs text-destructive">
                Select at least one event.
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={loading || events.length === 0}
            >
              {loading ? "..." : isEdit ? "Save Changes" : "Create Webhook"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
