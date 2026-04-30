"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { resubscribe, updatePreferences, type PreferencesView } from "@/lib/unsubscribe-api";
import { toast } from "sonner";

export function PreferencesForm({
  token,
  initialView,
}: {
  token: string;
  initialView: PreferencesView;
}) {
  const t = useTranslations("unsubscribe");
  const [view, setView] = useState<PreferencesView>(initialView);
  const [saving, setSaving] = useState(false);

  function toggle(slug: string, next: boolean) {
    setView((v) => ({
      ...v,
      entries: v.entries.map((e) =>
        e.template_type_slug === slug ? { ...e, subscribed: next } : e,
      ),
    }));
  }

  async function onSave() {
    setSaving(true);
    try {
      const changes = view.entries.map((e) => ({
        template_type_slug: e.template_type_slug,
        subscribed: e.subscribed,
      }));
      await updatePreferences(token, changes);
      toast.success(t("saved"));
    } finally {
      setSaving(false);
    }
  }

  async function onResubscribeAll() {
    setSaving(true);
    try {
      await resubscribe(token);
      setView((v) => ({ ...v, opted_out_of_all: false }));
      toast.success(t("saved"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card data-testid="preferences-card">
      <CardHeader>
        <CardTitle>{t("preferences_title", { email: view.email })}</CardTitle>
        <p className="text-sm text-muted-foreground">
          {t("preferences_subtitle", { workspace_name: view.workspace_name })}
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        {view.opted_out_of_all && (
          <Alert variant="destructive" data-testid="opted-out-all-alert">
            <AlertDescription className="flex items-center justify-between">
              <span>{t("all_warning", { workspace_name: view.workspace_name })}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={onResubscribeAll}
                data-testid="resubscribe-all-button"
              >
                {t("resubscribe_all")}
              </Button>
            </AlertDescription>
          </Alert>
        )}

        <div className="space-y-2">
          {view.entries.map((e) => (
            <div
              key={e.template_type_slug}
              className="flex items-start space-x-3 p-2 rounded hover:bg-muted/40"
              data-testid={`pref-row-${e.template_type_slug}`}
            >
              <Checkbox
                id={`pref-${e.template_type_slug}`}
                checked={e.subscribed}
                onCheckedChange={(v) => toggle(e.template_type_slug, Boolean(v))}
                data-testid={`pref-cb-${e.template_type_slug}`}
              />
              <div className="flex-1">
                <label
                  htmlFor={`pref-${e.template_type_slug}`}
                  className="font-medium cursor-pointer"
                >
                  {e.template_type_name}
                </label>
                {e.description && (
                  <p className="text-sm text-muted-foreground">{e.description}</p>
                )}
                <p className="text-xs text-muted-foreground mt-1">
                  {t("last_received")}: {new Date(e.last_received_at).toLocaleDateString()}
                </p>
              </div>
            </div>
          ))}
        </div>

        <Button
          onClick={onSave}
          disabled={saving}
          data-testid="save-button"
          className="w-full"
        >
          {t("save")}
        </Button>
      </CardContent>
    </Card>
  );
}
