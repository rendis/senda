"use client";

import { useMemo, useRef, useState } from "react";
import { type ColumnDef } from "@tanstack/react-table";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { z } from "zod";
import {
  Plus,
  Search,
  FileType,
  TriangleAlert,
  Pencil,
  Eye,
  Trash2,
  RotateCcw,
} from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  getTemplateCatalogState,
  getTemplateTypeManagementState,
  resolveResourceDisplayScope,
} from "@/lib/workspace-resource-policies";
import { cn } from "@/lib/utils";
import { applyEnvironmentSearchParam } from "@/lib/environment-mode";
import { generateSlug, nameSchema, slugSchema } from "@/lib/validations/slug";
import { useTemplateTypes, useCreateTemplateType, useUpdateTemplateType, useDeleteTemplateType } from "@/hooks/use-template-types";
import { useAdapterList } from "@/hooks/use-adapters";
import { useIdentityList } from "@/hooks/use-identities";
import { useResolvedWorkspacePolicies } from "@/hooks/use-settings";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ResourceStateBadges } from "@/components/shared/resource-state-badges";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { FormDialog } from "@/components/shared/form-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { TemplateType } from "@/types/templates";
import type { Adapter } from "@/types/adapters";
import { toast } from "sonner";
import {
  adapterUsesSenderIdentity,
  requiresExplicitSenderIdentity,
  resolveTemplateTypeSenderIdentityId,
  SENDER_DEFAULT_VALUE,
} from "./sender-identity-policy";

const SENDER_DEFAULT = SENDER_DEFAULT_VALUE;
const SENDER_NONE = "__none__";
const TEST_RECIPIENT_INHERIT = "__inherit__";
const SLUG_WARNING_LINES = [
  "Integraciones que usan el ref actual pueden dejar de resolver.",
  "Links o bookmarks al template type actual pueden quedar obsoletos.",
  "El historial y los filtros por slug pueden quedar divididos entre el valor viejo y el nuevo.",
] as const;
const emailAddressSchema = z.email("Enter a valid email address");

function parseRecipientAddressesInput(value: string): {
  addresses: string[];
  invalid: string[];
} {
  const parts = value
    .split(/[\n,;]+/)
    .map((item) => item.trim())
    .filter(Boolean);

  const invalid = parts.filter(
    (item) => !emailAddressSchema.safeParse(item).success,
  );

  return {
    addresses: Array.from(new Set(parts)),
    invalid,
  };
}

export function TemplateTypesContent() {
  return <TemplateTypesTable />;
}

function TemplateTypesTable() {
  const t = useTranslations("templateTypesPage");
  const tCommon = useTranslations("common");
  const router = useRouter();
  const scope = useScope();
  const scopedPath = useScopedPath();
  const isTestEnvironment = scope.environment === "test";
  const workspacePolicies = useResolvedWorkspacePolicies(scope);
  const templateCatalogState = getTemplateCatalogState(scope, workspacePolicies.data);
  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useTemplateTypes(scopedPath);
  const createMutation = useCreateTemplateType(scopedPath);
  const deleteMutation = useDeleteTemplateType(scopedPath);
  const { data: adapterData } = useAdapterList(scopedPath);
  const adapters = useMemo(
    () => adapterData?.pages.flatMap((p) => p.items) ?? [],
    [adapterData]
  );

  const [search, setSearch] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<TemplateType | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [newSlug, setNewSlug] = useState("");
  const [newName, setNewName] = useState("");
  const [newAdapterId, setNewAdapterId] = useState("");
  const [newSenderIdentityId, setNewSenderIdentityId] = useState("");
  const [newTestRecipientMode, setNewTestRecipientMode] =
    useState<string>(TEST_RECIPIENT_INHERIT);
  const [newTestRecipientAddresses, setNewTestRecipientAddresses] =
    useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [editTarget, setEditTarget] = useState<TemplateType | null>(null);

  const allItems = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data]
  );

  const filtered = useMemo(() => {
    if (!search) return allItems;
    const q = search.toLowerCase();
    return allItems.filter(
      (t) =>
        t.slug.toLowerCase().includes(q) ||
        t.name.toLowerCase().includes(q)
    );
  }, [allItems, search]);

  function buildTypePath(slug: string): string {
    switch (scope.level) {
      case "global":
        return `/global/templates/${slug}`;
      default:
        return applyEnvironmentSearchParam(
          `/t/${scope.tenantCode}/w/${scope.workspaceCode}/templates/${slug}`,
          scope.environment,
        );
    }
  }

  const columns: ColumnDef<TemplateType>[] = [
    {
      accessorKey: "slug",
      header: t("columns.slug"),
      cell: ({ row }) => (
        <button
          className="font-mono text-sm font-medium text-primary hover:underline"
          onClick={() => router.push(buildTypePath(row.original.slug))}
        >
          {row.original.slug}
        </button>
      ),
    },
    {
      accessorKey: "name",
      header: t("columns.name"),
      cell: ({ row }) => (
        <span className="text-sm text-foreground">{row.original.name}</span>
      ),
    },
    {
      accessorKey: "adapter_id",
      header: t("columns.adapter"),
      cell: ({ row }) => {
        const aid = row.original.adapter_id;
        if (!aid) {
          return (
            <span className="inline-flex items-center gap-1 text-status-complained text-xs">
              <TriangleAlert className="h-3.5 w-3.5" />
              {t("noAdapter")}
            </span>
          );
        }
        const adapter = adapters.find((a) => a.id === aid);
        return (
          <span className="text-xs text-foreground">
            {adapter?.name ?? aid.slice(0, 8)}
          </span>
        );
      },
    },
    {
      accessorKey: "scope_level",
      header: t("columns.scope"),
      cell: ({ row }) => (
        <div className="flex flex-wrap items-center gap-2">
          <ScopeIndicator scope={resolveResourceDisplayScope(row.original)} />
          <ResourceStateBadges
            badges={getTemplateTypeManagementState(
              scope,
              row.original,
              workspacePolicies.data,
            ).badges}
          />
        </div>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => {
        const itemState = getTemplateTypeManagementState(
          scope,
          row.original,
          workspacePolicies.data,
        );
        const readOnlyReason =
          row.original.owner_scope === "local"
            ? t("localTemplateTypesDisabled")
            : t("defaultTemplateTypesReadonly");

        return (
          <div className="flex items-center justify-end gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label={t("viewTemplatesAria", { slug: row.original.slug })}
                  onClick={() => router.push(buildTypePath(row.original.slug))}
                >
                  <Eye className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t("viewTemplates")}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label={t("editTemplateTypeAria", { slug: row.original.slug })}
                  onClick={() => itemState.canEdit && setEditTarget(row.original)}
                  disabled={!itemState.canEdit}
                >
                    <Pencil className="h-4 w-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>{itemState.canEdit ? tCommon("edit") : readOnlyReason}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-destructive"
                  aria-label={t("deleteTemplateTypeAria", { slug: row.original.slug })}
                  onClick={() => itemState.canDelete && setDeleteTarget(row.original)}
                  disabled={!itemState.canDelete}
                >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>{itemState.canDelete ? tCommon("delete") : readOnlyReason}</TooltipContent>
            </Tooltip>
          </div>
        );
      },
    },
  ];

  async function handleCreateType() {
    if (!newSlug.trim() || !newName.trim()) return;
    try {
      const selectedAdapter = adapters.find((adapter) => adapter.id === newAdapterId);
      if (requiresExplicitSenderIdentity(selectedAdapter) && !newSenderIdentityId) {
        toast.error(t("sharedSesRequiresIdentity"));
        return;
      }
      const senderIdValue = resolveTemplateTypeSenderIdentityId(newSenderIdentityId);
      const payload: Parameters<typeof createMutation.mutateAsync>[0] = {
        slug: newSlug.trim(),
        name: newName.trim(),
        adapter_id: newAdapterId || undefined,
        sender_identity_id: senderIdValue,
      };

      if (isTestEnvironment && newTestRecipientMode !== TEST_RECIPIENT_INHERIT) {
        const parsed = parseRecipientAddressesInput(newTestRecipientAddresses);
        if (parsed.invalid.length > 0) {
          toast.error(`Invalid addresses: ${parsed.invalid.join(", ")}`);
          return;
        }
        if (parsed.addresses.length === 0) {
          toast.error("Provide at least one safe recipient for the override.");
          return;
        }
        payload.test_recipient_mode = newTestRecipientMode as "replace" | "append";
        payload.test_recipient_addresses = parsed.addresses;
      }

      await createMutation.mutateAsync(payload);
      toast.success(t("templateTypeCreated"));
      setNewSlug("");
      setNewName("");
      setNewAdapterId("");
      setNewSenderIdentityId("");
      setNewTestRecipientMode(TEST_RECIPIENT_INHERIT);
      setNewTestRecipientAddresses("");
      setSlugTouched(false);
    } catch {
      toast.error(t("templateTypeCreateFailed"));
    }
  }

  const createTemplateTypeTrigger = (
    <Button disabled={!templateCatalogState.canCreateTemplateTypes}>
      <Plus className="h-4 w-4 mr-2" />
      {t("newTemplateType")}
    </Button>
  );

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div className="relative w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder={t("searchPlaceholder")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 h-9"
          />
        </div>
        {templateCatalogState.canCreateTemplateTypes ? (
          <FormDialog
            trigger={createTemplateTypeTrigger}
            open={createOpen}
            onOpenChange={setCreateOpen}
            title={t("createDialog.title")}
            description={t("createDialog.description")}
            submitLabel={tCommon("create")}
            onSubmit={handleCreateType}
          >
            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="tt-name">{tCommon("name")}</Label>
                <Input
                  id="tt-name"
                  placeholder={t("createDialog.namePlaceholder")}
                  value={newName}
                  onChange={(e) => {
                    const name = e.target.value;
                    setNewName(name);
                    if (!slugTouched) {
                      setNewSlug(generateSlug(name));
                    }
                  }}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="tt-slug">{t("slugLabel")}</Label>
                <Input
                  id="tt-slug"
                  placeholder={t("createDialog.slugPlaceholder")}
                  value={newSlug}
                  onChange={(e) => {
                    setNewSlug(e.target.value);
                    setSlugTouched(true);
                  }}
                  className="font-mono"
                />
              </div>
              <AdapterSelect
                adapters={adapters}
                value={newAdapterId}
                onChange={setNewAdapterId}
                senderIdentityId={newSenderIdentityId}
                onSenderIdentityChange={setNewSenderIdentityId}
              />
              {isTestEnvironment ? (
                <div className="rounded-md border border-amber-300/60 bg-amber-50 p-3 text-sm dark:border-amber-900/60 dark:bg-amber-950/20">
                  <div className="space-y-3">
                    <div>
                      <p className="font-medium text-amber-950 dark:text-amber-100">
                        Test recipient override
                      </p>
                      <p className="text-xs text-amber-800/90 dark:text-amber-200/90">
                        Leave this unset to inherit the workspace-level safe recipients. Use append with caution.
                      </p>
                    </div>
                    <div className="space-y-2">
                      <Label>Override mode</Label>
                      <Select
                        value={newTestRecipientMode}
                        onValueChange={setNewTestRecipientMode}
                      >
                        <SelectTrigger className="w-full bg-background">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={TEST_RECIPIENT_INHERIT}>
                            Inherit workspace default
                          </SelectItem>
                          <SelectItem value="replace">Replace recipients</SelectItem>
                          <SelectItem value="append">Append safe recipients</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    {newTestRecipientMode !== TEST_RECIPIENT_INHERIT ? (
                      <div className="space-y-2">
                        <Label>Safe recipients</Label>
                        <textarea
                          rows={4}
                          value={newTestRecipientAddresses}
                          onChange={(event) =>
                            setNewTestRecipientAddresses(event.target.value)
                          }
                          placeholder={"qa@example.com\napprover@example.com"}
                          className="flex min-h-[96px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-ring/10 transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-4"
                        />
                        <p className="text-xs text-muted-foreground">
                          Separate addresses with new lines, commas, or semicolons.
                        </p>
                        {newTestRecipientMode === "append" ? (
                          <p className="text-xs text-amber-700 dark:text-amber-300">
                            Warning: append preserves original recipients and adds these addresses.
                          </p>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </div>
          </FormDialog>
        ) : (
          <Tooltip>
            <TooltipTrigger asChild>
              <span>{createTemplateTypeTrigger}</span>
            </TooltipTrigger>
            <TooltipContent>
              {t("localTemplateCreationDisabled")}
            </TooltipContent>
          </Tooltip>
        )}
      </div>

      <DataTable
        columns={columns}
        data={filtered}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={FileType}
            title={t("empty.title")}
            description={t("empty.description")}
            action={
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCreateOpen(true)}
                disabled={!templateCatalogState.canCreateTemplateTypes}
              >
                <Plus className="h-4 w-4 mr-2" />
                {t("createDialog.title")}
              </Button>
            }
          />
        }
      />

      {editTarget && (
        <EditTemplateTypeDialog
          templateType={editTarget}
          adapters={adapters}
          open={!!editTarget}
          onOpenChange={(open) => !open && setEditTarget(null)}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t("deleteDialog.title")}
        description={t("deleteDialog.description", { name: deleteTarget?.name ?? "" })}
        confirmLabel={tCommon("delete")}
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.slug, {
              onSuccess: () => toast.success(t("templateTypeDeleted")),
              onError: () => toast.error(t("templateTypeDeleteFailed")),
            });
            setDeleteTarget(null);
          }
        }}
        loading={deleteMutation.isPending}
      />
    </div>
  );
}

function EditTemplateTypeDialog({
  templateType,
  adapters,
  open,
  onOpenChange,
}: {
  templateType: TemplateType;
  adapters: Adapter[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scope = useScope();
  const scopedPath = useScopedPath();
  const updateMutation = useUpdateTemplateType(scopedPath, templateType.slug);
  const shouldReduceMotion = useReducedMotion();
  const slugInputRef = useRef<HTMLInputElement>(null);
  const [name, setName] = useState(templateType.name);
  const [slug, setSlug] = useState(templateType.slug);
  const [adapterId, setAdapterId] = useState(templateType.adapter_id ?? "");
  const [senderIdentityId, setSenderIdentityId] = useState(templateType.sender_identity_id ?? "");
  const [testRecipientMode, setTestRecipientMode] = useState<string>(
    templateType.test_recipient_mode ?? TEST_RECIPIENT_INHERIT,
  );
  const [testRecipientAddresses, setTestRecipientAddresses] = useState(
    templateType.test_recipient_addresses?.join("\n") ?? "",
  );
  const isTestEnvironment = scope.environment === "test";

  const originalSlug = templateType.slug;
  const slugDirty = slug !== originalSlug;
  const slugValidation = slugSchema.safeParse(slug);
  const nameValidation = nameSchema.safeParse(name);
  const slugError = slugValidation.success ? null : slugValidation.error.issues[0]?.message ?? "Invalid slug";
  const nameError = nameValidation.success ? null : nameValidation.error.issues[0]?.message ?? "Invalid name";
  const hasValidationError = !!slugError || !!nameError;
  const slugWarningId = "template-type-slug-warning";
  const slugErrorId = "template-type-slug-error";
  const slugDescribedBy = [slugError ? slugErrorId : null, slugDirty ? slugWarningId : null]
    .filter(Boolean)
    .join(" ");

  function resetSlug() {
    setSlug(originalSlug);
    requestAnimationFrame(() => slugInputRef.current?.focus());
  }

  async function handleSubmit() {
    if (hasValidationError) {
      return true;
    }

    try {
      const selectedAdapter = adapters.find((adapter) => adapter.id === adapterId);
      if (requiresExplicitSenderIdentity(selectedAdapter) && !senderIdentityId) {
        toast.error("Shared SES/SMTP adapters require an explicit sender identity");
        return true;
      }
      const senderIdValue = resolveTemplateTypeSenderIdentityId(senderIdentityId, {
        clearWithEmptyString: true,
      });
      const payload: Parameters<typeof updateMutation.mutateAsync>[0] = {
        name: name.trim(),
        slug: slug.trim(),
        adapter_id: adapterId || undefined,
        sender_identity_id: senderIdValue,
      };
      if (isTestEnvironment) {
        if (testRecipientMode === TEST_RECIPIENT_INHERIT) {
          payload.test_recipient_mode = "";
          payload.test_recipient_addresses = [];
        } else {
          const parsed = parseRecipientAddressesInput(testRecipientAddresses);
          if (parsed.invalid.length > 0) {
            toast.error(`Invalid addresses: ${parsed.invalid.join(", ")}`);
            return true;
          }
          if (parsed.addresses.length === 0) {
            toast.error("Provide at least one safe recipient for the override.");
            return true;
          }
          payload.test_recipient_mode = testRecipientMode as "replace" | "append";
          payload.test_recipient_addresses = parsed.addresses;
        }
      }
      await updateMutation.mutateAsync(payload);
      toast.success("Template type updated");
    } catch {
      toast.error("Failed to update");
      return true;
    }
  }

  return (
    <FormDialog
      trigger={<span />}
      open={open}
      onOpenChange={onOpenChange}
      title={`Edit — ${templateType.name}`}
      description="Change the name, slug, adapter, and sender assigned to this template type."
      submitLabel="Update"
      onSubmit={handleSubmit}
      submitDisabled={hasValidationError || updateMutation.isPending}
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="edit-tt-name">Name</Label>
          <Input
            id="edit-tt-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            aria-invalid={nameError ? true : undefined}
          />
          {nameError && (
            <p className="text-xs text-destructive">{nameError}</p>
          )}
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="edit-tt-slug">Slug</Label>
          <div className="relative">
            <Input
              ref={slugInputRef}
              id="edit-tt-slug"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              className={cn(
                "font-mono pr-10",
                slugDirty && "border-status-complained focus-visible:ring-status-complained/20"
              )}
              aria-invalid={slugError ? true : undefined}
              aria-describedby={slugDescribedBy || undefined}
            />
            <AnimatePresence initial={false}>
              {slugDirty && (
                <motion.button
                  type="button"
                  aria-label="Restaurar slug original"
                  title="Restaurar slug original"
                  className="absolute right-2 top-1/2 inline-flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
                  onClick={resetSlug}
                  disabled={updateMutation.isPending}
                  initial={shouldReduceMotion ? { opacity: 0 } : { opacity: 0, scale: 0.85 }}
                  animate={shouldReduceMotion ? { opacity: 1 } : { opacity: 1, scale: 1 }}
                  exit={shouldReduceMotion ? { opacity: 0 } : { opacity: 0, scale: 0.85 }}
                  transition={{ duration: 0.18 }}
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                </motion.button>
              )}
            </AnimatePresence>
          </div>

          {slugError && (
            <p id={slugErrorId} className="text-xs text-destructive">
              {slugError}
            </p>
          )}

          <AnimatePresence initial={false}>
            {slugDirty && (
              <motion.div
                id={slugWarningId}
                className="overflow-hidden"
                initial={shouldReduceMotion ? { opacity: 0 } : { opacity: 0, height: 0, y: -4 }}
                animate={shouldReduceMotion ? { opacity: 1 } : { opacity: 1, height: "auto", y: 0 }}
                exit={shouldReduceMotion ? { opacity: 0 } : { opacity: 0, height: 0, y: -4 }}
                transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
              >
                <div className="rounded-md border border-status-complained/40 bg-status-complained-bg px-3 py-2 text-xs text-status-complained">
                  <div className="flex items-start gap-2">
                    <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    <div className="space-y-1.5">
                      <p className="font-medium">
                        Cambiar el slug puede romper referencias existentes.
                      </p>
                      <ul className="list-disc space-y-1 pl-4">
                        {SLUG_WARNING_LINES.map((line) => (
                          <li key={line}>{line}</li>
                        ))}
                      </ul>
                    </div>
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        <AdapterSelect
          adapters={adapters}
          value={adapterId}
          onChange={setAdapterId}
          senderIdentityId={senderIdentityId}
          onSenderIdentityChange={setSenderIdentityId}
        />
        {isTestEnvironment ? (
          <div className="rounded-md border border-amber-300/60 bg-amber-50 p-3 text-sm dark:border-amber-900/60 dark:bg-amber-950/20">
            <div className="space-y-3">
              <div>
                <p className="font-medium text-amber-950 dark:text-amber-100">
                  Test recipient override
                </p>
                <p className="text-xs text-amber-800/90 dark:text-amber-200/90">
                  Choose inherit to use the workspace default safe recipients. Replace is the recommended override mode.
                </p>
              </div>
              <div className="space-y-2">
                <Label>Override mode</Label>
                <Select value={testRecipientMode} onValueChange={setTestRecipientMode}>
                  <SelectTrigger className="w-full bg-background">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={TEST_RECIPIENT_INHERIT}>
                      Inherit workspace default
                    </SelectItem>
                    <SelectItem value="replace">Replace recipients</SelectItem>
                    <SelectItem value="append">Append safe recipients</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {testRecipientMode !== TEST_RECIPIENT_INHERIT ? (
                <div className="space-y-2">
                  <Label>Safe recipients</Label>
                  <textarea
                    rows={4}
                    value={testRecipientAddresses}
                    onChange={(event) =>
                      setTestRecipientAddresses(event.target.value)
                    }
                    placeholder={"qa@example.com\napprover@example.com"}
                    className="flex min-h-[96px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-ring/10 transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-4"
                  />
                  <p className="text-xs text-muted-foreground">
                    Separate addresses with new lines, commas, or semicolons.
                  </p>
                  {testRecipientMode === "append" ? (
                    <p className="text-xs text-amber-700 dark:text-amber-300">
                      Warning: append preserves original recipients and adds these addresses.
                    </p>
                  ) : null}
                </div>
              ) : null}
            </div>
          </div>
        ) : null}
      </div>
    </FormDialog>
  );
}

function AdapterSelect({
  adapters,
  value,
  onChange,
  senderIdentityId,
  onSenderIdentityChange,
}: {
  adapters: Adapter[];
  value: string;
  onChange: (value: string) => void;
  senderIdentityId?: string;
  onSenderIdentityChange?: (value: string) => void;
}) {
  const scopedPath = useScopedPath();
  const selectedAdapter = adapters.find((a) => a.id === value);
  const showIdentitySelect = adapterUsesSenderIdentity(selectedAdapter);
  const requireExplicitSender = requiresExplicitSenderIdentity(selectedAdapter);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-2">
        <Label>Adapter</Label>
        <Select
          value={value}
          onValueChange={(v) => {
            onChange(v);
            onSenderIdentityChange?.("");
          }}
        >
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Select adapter..." />
          </SelectTrigger>
          <SelectContent>
            {adapters.map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.name} ({a.adapter_type}{a.is_shared ? ", shared" : ""})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {showIdentitySelect && onSenderIdentityChange && (
        <SenderIdentitySelect
          scopedPath={scopedPath}
          adapterId={value}
          value={senderIdentityId ?? ""}
          onChange={onSenderIdentityChange}
          requireExplicitSender={requireExplicitSender}
        />
      )}
    </div>
  );
}

function SenderIdentitySelect({
  scopedPath,
  adapterId,
  value,
  onChange,
  requireExplicitSender,
}: {
  scopedPath: string;
  adapterId: string;
  value: string;
  onChange: (value: string) => void;
  requireExplicitSender?: boolean;
}) {
  const { data: identities, isLoading } = useIdentityList(scopedPath, adapterId);
  const emailIdentities = (identities ?? []).filter(
    (i) => i.identity_type === "email" && i.status === "verified"
  );

  return (
    <div className="flex flex-col gap-2">
      <Label>Sender Identity</Label>
      <Select value={value} onValueChange={onChange} disabled={isLoading}>
        <SelectTrigger className="w-full font-mono text-sm">
          <SelectValue placeholder={isLoading ? "Loading..." : requireExplicitSender ? "Select shared sender..." : "Use adapter default"} />
        </SelectTrigger>
        <SelectContent>
          {!requireExplicitSender && (
            <SelectItem value={SENDER_DEFAULT}>
              Use adapter default
            </SelectItem>
          )}
          {emailIdentities.map((i) => (
            <SelectItem key={i.id} value={i.id} className="font-mono">
              {i.identity}
              {i.display_name ? ` (${i.display_name})` : ""}
            </SelectItem>
          ))}
          {emailIdentities.length === 0 && !isLoading && (
            <SelectItem value={SENDER_NONE} disabled>
              No sender emails configured
            </SelectItem>
          )}
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        {requireExplicitSender
          ? "Shared SES/SMTP adapters require an explicit granted sender identity."
          : "Choose which email address to send from. Add senders in the adapter&apos;s identity panel."}
      </p>
    </div>
  );
}
