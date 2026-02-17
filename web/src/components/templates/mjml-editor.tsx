"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  ArrowLeft,
  Save,
  Send,
  Rocket,
  Monitor,
  Smartphone,
} from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useTemplateVersion,
  useSaveTemplateVersion,
  usePublishVersion,
  usePreviewMjml,
} from "@/hooks/use-template-version";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { TestSendModal } from "@/components/templates/test-send-modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import type { CreateTemplateVersionRequest } from "@/types/templates";

const metadataSchema = z.object({
  subject: z.string().min(1, { error: "Subject is required" }),
  preview_text: z.string().optional(),
  from_name: z.string().min(1, { error: "From name is required" }),
  from_email: z.string().min(1, { error: "From email is required" }),
  reply_to: z.string().optional(),
});

type MetadataForm = z.infer<typeof metadataSchema>;

export function MjmlEditor() {
  const router = useRouter();
  const scope = useScope();
  const scopedPath = useScopedPath();
  const searchParams = useSearchParams();
  const templateId = searchParams.get("templateId") ?? "";
  const versionId = searchParams.get("versionId") ?? "";

  const { data: version, isLoading } = useTemplateVersion(
    scopedPath,
    templateId,
    versionId
  );

  const saveMutation = useSaveTemplateVersion(
    scopedPath,
    templateId,
    versionId
  );
  const publishMutation = usePublishVersion(
    scopedPath,
    templateId,
    versionId
  );
  const previewMutation = usePreviewMjml(scopedPath, templateId);

  const [previewHtml, setPreviewHtml] = useState("");
  const [previewMode, setPreviewMode] = useState<"desktop" | "mobile">(
    "desktop"
  );
  const [showPublishConfirm, setShowPublishConfirm] = useState(false);
  const [showTestSend, setShowTestSend] = useState(false);
  const [activeLocale, setActiveLocale] = useState("default");
  const previewTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Track MJML code edits; null means "not yet edited, use version data"
  const [mjmlOverride, setMjmlOverride] = useState<string | null>(null);
  const mjml = mjmlOverride ?? version?.body_mjml ?? "";

  const {
    register,
    getValues,
    formState: { errors },
  } = useForm<MetadataForm>({
    resolver: zodResolver(metadataSchema),
    values: version
      ? {
          subject: version.subject,
          preview_text: version.preview_text ?? "",
          from_name: version.from_name,
          from_email: version.from_email,
          reply_to: version.reply_to ?? "",
        }
      : {
          subject: "",
          preview_text: "",
          from_name: "",
          from_email: "",
          reply_to: "",
        },
  });

  // Debounced preview update
  const triggerPreview = useCallback(
    (code: string) => {
      if (previewTimeoutRef.current) {
        clearTimeout(previewTimeoutRef.current);
      }
      previewTimeoutRef.current = setTimeout(async () => {
        if (!code.trim()) return;
        try {
          const result = await previewMutation.mutateAsync(code);
          setPreviewHtml(result.html);
        } catch {
          // Silently fail preview, user will see stale content
        }
      }, 800);
    },
    [previewMutation]
  );

  function handleMjmlChange(value: string) {
    setMjmlOverride(value);
    triggerPreview(value);
  }

  async function handleSaveDraft() {
    const formData = getValues();
    const body: CreateTemplateVersionRequest = {
      subject: formData.subject,
      preview_text: formData.preview_text || undefined,
      from_name: formData.from_name,
      from_email: formData.from_email,
      reply_to: formData.reply_to || undefined,
      body_mjml: mjml,
      default_locale: version?.default_locale ?? "en",
    };
    try {
      await saveMutation.mutateAsync(body);
      toast.success("Draft saved");
    } catch {
      toast.error("Failed to save draft");
    }
  }

  async function handlePublish() {
    try {
      await publishMutation.mutateAsync();
      toast.success("Version published");
      setShowPublishConfirm(false);
    } catch {
      toast.error("Failed to publish version");
    }
  }

  function buildBackPath() {
    const slug =
      typeof window !== "undefined"
        ? window.location.pathname.split("/templates/")[1]?.split("/")[0]
        : "";
    switch (scope.level) {
      case "global":
        return `/global/templates/${slug}`;
      case "tenant":
        return `/t/${scope.tenantCode}/templates/${slug}`;
      case "workspace":
        return `/t/${scope.tenantCode}/w/${scope.workspaceCode}/templates/${slug}`;
    }
  }

  if (isLoading) {
    return <MjmlEditorSkeleton />;
  }

  if (!version) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        Version not found
      </div>
    );
  }

  const isDraft = version.status === "draft";

  return (
    <div className="flex flex-col h-full">
      {/* Header bar */}
      <div className="flex items-center justify-between h-14 px-6 border-b bg-card shrink-0">
        <div className="flex items-center gap-3">
          <button
            onClick={() => router.push(buildBackPath())}
            className="text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-5 w-5" />
          </button>
          <div className="flex items-center gap-2 text-sm">
            <span className="text-muted-foreground">Templates</span>
            <span className="text-muted-foreground">/</span>
            <span className="text-muted-foreground">
              {version.subject.split("—")[0]?.trim() ?? "Template"}
            </span>
            <span className="text-muted-foreground">/</span>
            <span className="font-medium">
              Version {version.version_number} (
              {version.status.charAt(0).toUpperCase() +
                version.status.slice(1)}
              )
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2.5">
          {/* Locale tabs */}
          <div className="flex items-center rounded-md border h-8 overflow-hidden">
            <button
              className={`px-2.5 h-full font-mono text-[11px] font-medium ${
                activeLocale === "default"
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
              onClick={() => setActiveLocale("default")}
            >
              {version.default_locale}
            </button>
          </div>

          {isDraft && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleSaveDraft}
              disabled={saveMutation.isPending}
            >
              <Save className="h-4 w-4 mr-1.5" />
              Save Draft
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => setShowTestSend(true)}
          >
            <Send className="h-4 w-4 mr-1.5" />
            Send Test
          </Button>
          {isDraft && (
            <Button size="sm" onClick={() => setShowPublishConfirm(true)}>
              <Rocket className="h-4 w-4 mr-1.5" />
              Publish
            </Button>
          )}
        </div>
      </div>

      {/* Editor body: split panes */}
      <div className="flex flex-1 overflow-hidden">
        {/* Left panel: code + metadata */}
        <div className="flex flex-col flex-1 border-r min-w-0">
          {/* Code editor area */}
          <div className="flex-1 min-h-0">
            <MonacoEditorWrapper
              value={mjml}
              onChange={handleMjmlChange}
              readOnly={!isDraft}
            />
          </div>

          {/* Metadata panel */}
          <div className="border-t bg-card p-4 shrink-0">
            <h4 className="text-sm font-semibold mb-3">Metadata</h4>
            <div className="grid grid-cols-2 gap-3">
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">Subject</Label>
                <Input
                  {...register("subject")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
                {errors.subject && (
                  <span className="text-xs text-destructive">
                    {errors.subject.message}
                  </span>
                )}
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">Preview Text</Label>
                <Input
                  {...register("preview_text")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">From Name</Label>
                <Input
                  {...register("from_name")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
                {errors.from_name && (
                  <span className="text-xs text-destructive">
                    {errors.from_name.message}
                  </span>
                )}
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">Reply-To</Label>
                <Input
                  {...register("reply_to")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
              </div>
            </div>
          </div>
        </div>

        {/* Right panel: preview */}
        <div className="flex flex-col w-[480px] shrink-0">
          <div className="flex items-center justify-between h-10 px-4 border-b bg-card">
            <span className="text-sm font-semibold">Preview</span>
            <div className="flex items-center rounded-md border h-7 overflow-hidden">
              <button
                className={`px-2 h-full ${
                  previewMode === "desktop"
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground"
                }`}
                onClick={() => setPreviewMode("desktop")}
              >
                <Monitor className="h-3.5 w-3.5" />
              </button>
              <button
                className={`px-2 h-full ${
                  previewMode === "mobile"
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground"
                }`}
                onClick={() => setPreviewMode("mobile")}
              >
                <Smartphone className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
          <div className="flex-1 bg-slate-100 p-6 overflow-auto">
            <div
              className={`bg-white rounded-md border mx-auto transition-all ${
                previewMode === "mobile" ? "max-w-[375px]" : "max-w-full"
              }`}
            >
              {previewHtml ? (
                <iframe
                  srcDoc={previewHtml}
                  className="w-full min-h-[400px] border-0"
                  sandbox="allow-same-origin"
                  title="Email Preview"
                />
              ) : (
                <div className="flex items-center justify-center h-[400px] text-sm text-muted-foreground">
                  {previewMutation.isPending
                    ? "Generating preview..."
                    : "Write MJML to see preview"}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Modals */}
      <ConfirmDialog
        open={showPublishConfirm}
        onOpenChange={setShowPublishConfirm}
        title="Publish Version"
        description={`Publishing version ${version.version_number} will make it the active template. The current published version will be archived. This action cannot be undone.`}
        confirmLabel="Publish"
        variant="default"
        onConfirm={handlePublish}
        loading={publishMutation.isPending}
      />

      <TestSendModal
        open={showTestSend}
        onOpenChange={setShowTestSend}
        scopedPath={scopedPath}
        templateId={templateId}
        locale={activeLocale === "default" ? undefined : activeLocale}
      />
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type MonacoEditorComponent = React.ComponentType<any>;

/** Wrapper for lazy-loaded Monaco editor */
function MonacoEditorWrapper({
  value,
  onChange,
  readOnly,
}: {
  value: string;
  onChange: (value: string) => void;
  readOnly: boolean;
}) {
  const [Editor, setEditor] = useState<MonacoEditorComponent | null>(null);

  useEffect(() => {
    import("@monaco-editor/react").then((mod) => {
      setEditor(() => mod.default as MonacoEditorComponent);
    });
  }, []);

  if (!Editor) {
    return (
      <div className="flex items-center justify-center h-full bg-slate-900">
        <span className="text-slate-400 text-sm">Loading editor...</span>
      </div>
    );
  }

  return (
    <Editor
      value={value}
      onChange={(val: string | undefined) => onChange(val ?? "")}
      language="xml"
      theme="vs-dark"
      options={{
        minimap: { enabled: false },
        fontSize: 13,
        fontFamily: "'IBM Plex Mono', monospace",
        lineNumbers: "on",
        scrollBeyondLastLine: false,
        wordWrap: "on",
        readOnly,
        padding: { top: 16 },
      }}
      className="h-full"
    />
  );
}

function MjmlEditorSkeleton() {
  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between h-14 px-6 border-b bg-card">
        <Skeleton className="h-5 w-64" />
        <div className="flex gap-2">
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-8 w-20" />
        </div>
      </div>
      <div className="flex flex-1">
        <div className="flex-1 bg-slate-900" />
        <div className="w-[480px] bg-slate-100 border-l" />
      </div>
    </div>
  );
}
