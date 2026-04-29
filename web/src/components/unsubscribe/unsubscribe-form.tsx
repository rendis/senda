"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { optOutThisType, optOutAll, type UnsubscribeContext } from "@/lib/unsubscribe-api";
import { toast } from "sonner";

type Choice = "this_event" | "all";

export function UnsubscribeForm({
  token,
  initialContext,
}: {
  token: string;
  initialContext: UnsubscribeContext;
}) {
  const t = useTranslations("unsubscribe");
  const [choice, setChoice] = useState<Choice>("this_event");
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState<Choice | null>(null);

  async function onConfirm() {
    setSubmitting(true);
    try {
      if (choice === "this_event") {
        await optOutThisType(token);
        toast.success(t("success_event", { template_type_name: initialContext.template_type_name }));
      } else {
        await optOutAll(token);
        toast.success(t("success_all", { workspace_name: initialContext.workspace_name }));
      }
      setDone(choice);
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return (
      <Card data-testid="success-card">
        <CardContent className="pt-6">
          <Alert>
            <AlertDescription>
              {done === "this_event"
                ? t("success_event", { template_type_name: initialContext.template_type_name })
                : t("success_all", { workspace_name: initialContext.workspace_name })}
            </AlertDescription>
          </Alert>
          <div className="mt-4">
            <Link href={`/u/${token}/preferences`} className="text-sm underline" data-testid="manage-link">
              {t("manage_link")}
            </Link>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card data-testid="unsubscribe-card">
      <CardHeader>
        <CardTitle>
          {t("title_for_event", { workspace_name: initialContext.workspace_name })}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <RadioGroup
          value={choice}
          onValueChange={(v) => setChoice(v as Choice)}
        >
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="this_event" id="this_event" data-testid="radio-this-event" />
            <Label htmlFor="this_event">
              {t("this_event_label", { template_type_name: initialContext.template_type_name })}
            </Label>
          </div>
          <div className="flex items-center space-x-2">
            <RadioGroupItem value="all" id="all" data-testid="radio-all" />
            <Label htmlFor="all">
              {t("all_label", { workspace_name: initialContext.workspace_name })}
            </Label>
          </div>
        </RadioGroup>

        <Button onClick={onConfirm} disabled={submitting} data-testid="confirm-button" className="w-full">
          {t("confirm")}
        </Button>

        <Link href={`/u/${token}/preferences`} className="block text-sm underline text-center" data-testid="manage-link">
          {t("manage_link")}
        </Link>
      </CardContent>
    </Card>
  );
}
