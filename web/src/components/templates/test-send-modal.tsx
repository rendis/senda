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
import { useTestSend } from "@/hooks/use-template-version";
import { toast } from "sonner";

interface TestSendModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  scopedPath: string;
  templateId: string;
  locale?: string;
}

export function TestSendModal({
  open,
  onOpenChange,
  scopedPath,
  templateId,
  locale,
}: TestSendModalProps) {
  const [email, setEmail] = useState("");
  const [variablesJson, setVariablesJson] = useState("{}");
  const testSend = useTestSend(scopedPath, templateId);

  async function handleSend() {
    if (!email.trim()) {
      toast.error("Please enter a recipient email");
      return;
    }
    let variables: Record<string, unknown> = {};
    try {
      variables = JSON.parse(variablesJson);
    } catch {
      toast.error("Invalid JSON for variables");
      return;
    }
    try {
      await testSend.mutateAsync({
        recipient_email: email.trim(),
        variables,
        locale,
      });
      toast.success(`Test email sent to ${email}`);
      onOpenChange(false);
    } catch {
      toast.error("Failed to send test email");
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !testSend.isPending && onOpenChange(v)}>
      <DialogContent className="sm:max-w-md" onInteractOutside={(e) => testSend.isPending && e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>Send Test Email</DialogTitle>
          <DialogDescription>
            Send a test email to verify your template renders correctly.
          </DialogDescription>
        </DialogHeader>
        <fieldset disabled={testSend.isPending} className="flex flex-col gap-4 py-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="test-email">Recipient Email</Label>
            <Input
              id="test-email"
              type="email"
              placeholder="test@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="test-vars">Variables (JSON)</Label>
            <textarea
              id="test-vars"
              className="flex min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              value={variablesJson}
              onChange={(e) => setVariablesJson(e.target.value)}
              placeholder='{"user_name": "Juan", "cta_url": "https://..."}'
            />
          </div>
        </fieldset>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={testSend.isPending}
          >
            Cancel
          </Button>
          <Button onClick={handleSend} disabled={testSend.isPending}>
            {testSend.isPending ? "Sending..." : "Send Test"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
