"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { zodResolver } from "@hookform/resolvers/zod";
import { type ColumnDef } from "@tanstack/react-table";
import { HTTPError } from "ky";
import { useForm, useWatch, type UseFormSetError } from "react-hook-form";
import { z } from "zod";
import { Layers, Pencil, Plus, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { FormDialog } from "@/components/shared/form-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import {
  SYSTEM_WORKSPACE_LABEL,
  getWorkspaceDisplayName,
  getWorkspaceDisplayCode,
} from "@/lib/system-workspace-display";
import { StatusBadge } from "@/components/shared/status-badge";
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
import {
  type CreateWorkspaceInput,
  type UpdateWorkspaceInput,
  useCreateWorkspace,
  useDeleteWorkspace,
  useUpdateWorkspace,
  useWorkspacesManagement,
} from "@/hooks/use-workspaces-mgmt";
import { useScope } from "@/hooks/use-scope";
import { parseApiError } from "@/lib/api";
import { formatDate } from "@/lib/utils";
import {
  generateSlug,
  nameSchema,
  slugSchema,
} from "@/lib/validations/slug";
import { SYSTEM_WORKSPACE_CODE, type Workspace } from "@/types/api";

const createWorkspaceSchema = z.object({
  name: nameSchema,
  code: slugSchema,
});

const editWorkspaceSchema = z.object({
  name: nameSchema,
  status: z.enum(["active", "disabled"]),
});

type CreateWorkspaceFormValues = z.infer<typeof createWorkspaceSchema>;
type EditWorkspaceFormValues = z.infer<typeof editWorkspaceSchema>;
type WorkspaceToggleTarget = {
  workspace: Workspace;
  nextActive: boolean;
};

function formatCompactDate(value: string): string {
  return formatDate(value).replace(",", "");
}

async function applyCreateWorkspaceErrors(
  error: unknown,
  setError: UseFormSetError<CreateWorkspaceFormValues>,
) {
  if (error instanceof HTTPError && error.response.status >= 500) {
    toast.error("Failed to create workspace");
    return;
  }

  const apiError = await parseApiError(error);
  if (apiError.error.details?.length) {
    for (const detail of apiError.error.details) {
      if (detail.field === "name" || detail.field === "code") {
        setError(detail.field, { message: detail.message });
      }
    }
    return;
  }

  toast.error(apiError.error.message || "Failed to create workspace");
}

async function applyEditWorkspaceErrors(
  error: unknown,
  setError: UseFormSetError<EditWorkspaceFormValues>,
) {
  const apiError = await parseApiError(error);
  if (apiError.error.details?.length) {
    for (const detail of apiError.error.details) {
      if (detail.field === "name") {
        setError("name", { message: detail.message });
      }
    }
    return;
  }

  toast.error(apiError.error.message || "Failed to update workspace");
}

export function WorkspacesContent() {
  const router = useRouter();
  const scope = useScope();
  const tenantCode = scope.tenantCode;
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Workspace | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Workspace | null>(null);
  const [toggleTarget, setToggleTarget] = useState<WorkspaceToggleTarget | null>(null);

  useEffect(() => {
    const timer = setTimeout(() => setSearch(searchInput.trim()), 250);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const {
    data,
    isLoading,
    error,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useWorkspacesManagement(tenantCode ?? "", search);
  const createWorkspace = useCreateWorkspace(tenantCode ?? "");
  const updateWorkspace = useUpdateWorkspace(tenantCode ?? "");
  const deleteWorkspace = useDeleteWorkspace(tenantCode ?? "");

  useEffect(() => {
    if (error) toast.error("Failed to load workspaces");
  }, [error]);

  const workspaces = useMemo(
    () => data?.pages.flatMap((page) => page.items) ?? [],
    [data],
  );

  const columns: ColumnDef<Workspace>[] = [
    {
      accessorKey: "name",
      header: "Name",
      cell: ({ row }) => {
        const displayName = getWorkspaceDisplayName(row.original);

        return (
          <div className="flex items-center gap-2">
            <span className="text-[13px]">{displayName}</span>
            <ScopeIndicator
              scope={row.original.is_system ? "system" : "workspace"}
              label={row.original.is_system ? SYSTEM_WORKSPACE_LABEL : "workspace"}
            />
          </div>
        );
      },
    },
    {
      accessorKey: "code",
      header: "Code",
      cell: ({ row }) => (
        <span className="font-mono text-[13px] font-medium">
          {getWorkspaceDisplayCode(row.original)}
        </span>
      ),
    },
    {
      accessorKey: "is_active",
      header: "Status",
      size: 220,
      cell: ({ row }) => {
        const isProtected = row.original.is_system;
        const displayCode = getWorkspaceDisplayCode(row.original);
        const protectedReason = `The ${SYSTEM_WORKSPACE_LABEL} workspace is always active`;

        return (
          <div className="flex items-center gap-3">
            <StatusBadge status={row.original.is_active ? "active" : "disabled"} />
            <Tooltip>
              <TooltipTrigger asChild>
                <span
                  className="inline-flex"
                  onClick={(event) => event.stopPropagation()}
                >
                  <button
                    type="button"
                    role="switch"
                    aria-checked={row.original.is_active}
                    aria-label={`Toggle workspace ${displayCode} status`}
                    disabled={isProtected || updateWorkspace.isPending}
                    onClick={(event) => {
                      event.stopPropagation();
                      if (isProtected) return;
                      setToggleTarget({
                        workspace: row.original,
                        nextActive: !row.original.is_active,
                      });
                    }}
                    className={`relative inline-flex h-6 w-11 shrink-0 rounded-full border-2 border-transparent transition-colors ${
                      row.original.is_active ? "bg-primary" : "bg-muted"
                    } ${isProtected || updateWorkspace.isPending ? "cursor-not-allowed opacity-60" : "cursor-pointer"}`}
                  >
                    <span
                      className={`pointer-events-none block h-5 w-5 rounded-full bg-white shadow-lg transition-transform ${
                        row.original.is_active ? "translate-x-5" : "translate-x-0"
                      }`}
                    />
                  </button>
                </span>
              </TooltipTrigger>
              <TooltipContent>
                {isProtected
                  ? protectedReason
                  : row.original.is_active
                    ? "Disable workspace"
                    : "Enable workspace"}
              </TooltipContent>
            </Tooltip>
          </div>
        );
      },
    },
    {
      accessorKey: "created_at",
      header: "Created",
      size: 180,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {formatCompactDate(row.original.created_at)}
        </span>
      ),
    },
    {
      accessorKey: "updated_at",
      header: "Updated",
      size: 180,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {formatCompactDate(row.original.updated_at)}
        </span>
      ),
    },
    {
      id: "actions",
      header: "",
      size: 64,
      enableSorting: false,
      cell: ({ row }) => {
        const isProtected = row.original.is_system;
        const displayCode = getWorkspaceDisplayCode(row.original);
        const protectedReason = `The ${SYSTEM_WORKSPACE_LABEL} workspace is protected`;

        return (
          <div className="flex items-center justify-end gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <span
                  className="inline-flex"
                  onClick={(event) => event.stopPropagation()}
                >
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    aria-label={`Edit workspace ${displayCode}`}
                    disabled={isProtected}
                    onClick={(event) => {
                      event.stopPropagation();
                      setEditTarget(row.original);
                    }}
                  >
                    <Pencil className="h-4 w-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>
                {isProtected ? protectedReason : "Edit workspace"}
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <span
                  className="inline-flex"
                  onClick={(event) => event.stopPropagation()}
                >
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-destructive"
                    aria-label={`Delete workspace ${displayCode}`}
                    disabled={isProtected}
                    onClick={(event) => {
                      event.stopPropagation();
                      setDeleteTarget(row.original);
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>
                {isProtected ? protectedReason : "Delete workspace"}
              </TooltipContent>
            </Tooltip>
          </div>
        );
      },
    },
  ];

  if (scope.workspaceCode !== SYSTEM_WORKSPACE_CODE || !tenantCode) {
    return (
      <EmptyState
        icon={Layers}
        title="Not available"
        description={`Workspace management is only available from the ${SYSTEM_WORKSPACE_LABEL} workspace.`}
      />
    );
  }

  const handleCreateWorkspace = async (values: CreateWorkspaceInput) => {
    const created = await createWorkspace.mutateAsync(values);
    toast.success(`Workspace "${created.name}" created`);
  };

  const handleUpdateWorkspace = async (
    workspaceCode: string,
    data: UpdateWorkspaceInput,
  ) => {
    const updated = await updateWorkspace.mutateAsync({
      workspaceCode,
      data,
    });

    if (typeof data.is_active === "boolean") {
      toast.success(
        data.is_active
          ? `Workspace "${updated.name}" enabled`
          : `Workspace "${updated.name}" disabled`,
      );
      return;
    }

    toast.success(`Workspace "${updated.name}" updated`);
  };

  const handleDeleteWorkspace = async () => {
    if (!deleteTarget) return;
    try {
      await deleteWorkspace.mutateAsync(deleteTarget.code);
      toast.success(`Workspace "${deleteTarget.name}" deleted`);
      setDeleteTarget(null);
    } catch (error) {
      const apiError = await parseApiError(error);
      toast.error(apiError.error.message || "Failed to delete workspace");
    }
  };

  const handleToggleWorkspaceStatus = async () => {
    if (!toggleTarget) return;

    try {
      await handleUpdateWorkspace(toggleTarget.workspace.code, {
        is_active: toggleTarget.nextActive,
      });
      setToggleTarget(null);
    } catch (error) {
      const apiError = await parseApiError(error);
      toast.error(apiError.error.message || "Failed to update workspace");
    }
  };

  return (
    <>
      <div className="mb-6 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="relative w-full md:max-w-sm">
          <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search workspaces..."
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            className="h-9 pl-9"
          />
        </div>

        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Workspace
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={workspaces}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        onRowClick={(workspace) => router.push(`/t/${tenantCode}/w/${workspace.code}`)}
        emptyState={
          <EmptyState
            icon={Layers}
            title="No workspaces found"
            description={
              search
                ? "No workspace matches the current search."
                : "Create a workspace to manage scoped emails, templates, and members for this tenant."
            }
            action={
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Create Workspace
              </Button>
            }
          />
        }
      />

      <CreateWorkspaceDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreateWorkspace={handleCreateWorkspace}
      />

      {editTarget && (
        <EditWorkspaceDialog
          workspace={editTarget}
          open={!!editTarget}
          onOpenChange={(open) => {
            if (!open) setEditTarget(null);
          }}
          onUpdateWorkspace={handleUpdateWorkspace}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete workspace"
        description={
          deleteTarget
            ? `Soft-delete workspace "${deleteTarget.name}" (${deleteTarget.code})? Existing records remain in the backend, but the workspace will disappear from active lists.`
            : ""
        }
        confirmLabel="Delete workspace"
        onConfirm={handleDeleteWorkspace}
        loading={deleteWorkspace.isPending}
      />

      <ConfirmDialog
        open={!!toggleTarget}
        onOpenChange={(open) => {
          if (!open) setToggleTarget(null);
        }}
        title={toggleTarget?.nextActive ? "Enable workspace" : "Disable workspace"}
        description={
          toggleTarget
            ? toggleTarget.nextActive
              ? `Enable workspace "${toggleTarget.workspace.name}" (${toggleTarget.workspace.code}) so tenant members can use it again from active lists?`
              : `Disable workspace "${toggleTarget.workspace.name}" (${toggleTarget.workspace.code}) from the list? Existing records remain available in the backend, but the workspace will be marked as inactive.`
            : ""
        }
        confirmLabel={toggleTarget?.nextActive ? "Enable workspace" : "Disable workspace"}
        variant={toggleTarget?.nextActive ? "default" : "destructive"}
        onConfirm={handleToggleWorkspaceStatus}
        loading={updateWorkspace.isPending}
      />
    </>
  );
}

function CreateWorkspaceDialog({
  open,
  onOpenChange,
  onCreateWorkspace,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreateWorkspace: (values: CreateWorkspaceInput) => Promise<void>;
}) {
  const codeManuallyEdited = useRef(false);
  const form = useForm<CreateWorkspaceFormValues>({
    resolver: zodResolver(createWorkspaceSchema),
    defaultValues: { name: "", code: "" },
  });

  const watchedName = useWatch({ control: form.control, name: "name" });

  useEffect(() => {
    if (!open) {
      form.reset({ name: "", code: "" });
      codeManuallyEdited.current = false;
      return;
    }

    if (!codeManuallyEdited.current && watchedName) {
      form.setValue("code", generateSlug(watchedName), {
        shouldValidate: form.formState.isSubmitted,
      });
    }
  }, [form, open, watchedName]);

  const handleSubmit = async () => {
    let keepOpen = false;

    await form.handleSubmit(
      async (values) => {
        try {
          await onCreateWorkspace(values);
          form.reset({ name: "", code: "" });
          codeManuallyEdited.current = false;
        } catch (error) {
          keepOpen = true;
          await applyCreateWorkspaceErrors(error, form.setError);
        }
      },
      () => {
        keepOpen = true;
      },
    )();

    return keepOpen;
  };

  const codeField = form.register("code");

  return (
    <FormDialog
      trigger={<span className="hidden" />}
      title="Create Workspace"
      description={`Create a regular workspace for this tenant. The ${SYSTEM_WORKSPACE_LABEL} workspace remains protected.`}
      submitLabel="Create Workspace"
      loadingLabel="Creating..."
      submitIcon={<Plus className="h-4 w-4" />}
      open={open}
      onOpenChange={onOpenChange}
      onSubmit={handleSubmit}
    >
      <div className="flex flex-col gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="workspace-name" className="text-[13px] font-medium">
            Name
          </Label>
          <Input id="workspace-name" {...form.register("name")} />
          {form.formState.errors.name && (
            <p className="text-xs text-destructive">
              {form.formState.errors.name.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="workspace-code" className="text-[13px] font-medium">
            Code
          </Label>
          <Input
            id="workspace-code"
            {...codeField}
            onChange={(event) => {
              codeManuallyEdited.current = true;
              codeField.onChange(event);
            }}
          />
          {form.formState.errors.code && (
            <p className="text-xs text-destructive">
              {form.formState.errors.code.message}
            </p>
          )}
        </div>
      </div>
    </FormDialog>
  );
}

function EditWorkspaceDialog({
  workspace,
  open,
  onOpenChange,
  onUpdateWorkspace,
}: {
  workspace: Workspace;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUpdateWorkspace: (
    workspaceCode: string,
    data: UpdateWorkspaceInput,
  ) => Promise<void>;
}) {
  const form = useForm<EditWorkspaceFormValues>({
    resolver: zodResolver(editWorkspaceSchema),
    defaultValues: {
      name: workspace.name,
      status: workspace.is_active ? "active" : "disabled",
    },
  });
  const watchedStatus = useWatch({ control: form.control, name: "status" });

  useEffect(() => {
    form.reset({
      name: workspace.name,
      status: workspace.is_active ? "active" : "disabled",
    });
  }, [form, workspace]);

  const handleSubmit = async () => {
    let keepOpen = false;

    await form.handleSubmit(
      async (values) => {
        try {
          await onUpdateWorkspace(workspace.code, {
            name: values.name,
            is_active: values.status === "active",
          });
        } catch (error) {
          keepOpen = true;
          await applyEditWorkspaceErrors(error, form.setError);
        }
      },
      () => {
        keepOpen = true;
      },
    )();

    return keepOpen;
  };

  return (
    <FormDialog
      trigger={<span className="hidden" />}
      title="Edit Workspace"
      description="Update the workspace display name and activation status. The workspace code remains immutable."
      submitLabel="Save Changes"
      loadingLabel="Saving..."
      submitIcon={<Pencil className="h-4 w-4" />}
      open={open}
      onOpenChange={onOpenChange}
      onSubmit={handleSubmit}
    >
      <div className="flex flex-col gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="edit-workspace-name" className="text-[13px] font-medium">
            Name
          </Label>
          <Input id="edit-workspace-name" {...form.register("name")} />
          {form.formState.errors.name && (
            <p className="text-xs text-destructive">
              {form.formState.errors.name.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="edit-workspace-code" className="text-[13px] font-medium">
            Code
          </Label>
          <Input
            id="edit-workspace-code"
            value={workspace.code}
            readOnly
            className="bg-muted/50 font-mono text-[13px] text-muted-foreground"
          />
        </div>

        <div className="space-y-1.5">
          <Label className="text-[13px] font-medium">Status</Label>
          <Select
            value={watchedStatus}
            onValueChange={(value) =>
              form.setValue("status", value as EditWorkspaceFormValues["status"], {
                shouldValidate: true,
              })
            }
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="disabled">Disabled</SelectItem>
            </SelectContent>
          </Select>
          {form.formState.errors.status && (
            <p className="text-xs text-destructive">
              {form.formState.errors.status.message}
            </p>
          )}
        </div>
      </div>
    </FormDialog>
  );
}
