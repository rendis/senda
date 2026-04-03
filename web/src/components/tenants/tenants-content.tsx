"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { zodResolver } from "@hookform/resolvers/zod";
import { type ColumnDef } from "@tanstack/react-table";
import { HTTPError } from "ky";
import { useForm, useWatch, type UseFormSetError } from "react-hook-form";
import { z } from "zod";
import { Building2, Pencil, Plus, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { FormDialog } from "@/components/shared/form-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
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
  type CreateTenantInput,
  useCreateTenant,
  useDeleteTenant,
  useTenants,
  useUpdateTenant,
} from "@/hooks/use-tenants-mgmt";
import { parseApiError } from "@/lib/api";
import { formatDate } from "@/lib/utils";
import {
  generateSlug,
  nameSchema,
  slugSchema,
} from "@/lib/validations/slug";
import type { Tenant } from "@/types/api";

const createTenantSchema = z.object({
  name: nameSchema,
  code: slugSchema,
});

const editTenantSchema = z.object({
  name: nameSchema,
  status: z.enum(["active", "disabled"]),
});

type CreateTenantFormValues = z.infer<typeof createTenantSchema>;
type EditTenantFormValues = z.infer<typeof editTenantSchema>;

function formatCompactDate(value: string): string {
  return formatDate(value).replace(",", "");
}

async function applyCreateTenantErrors(
  error: unknown,
  setError: UseFormSetError<CreateTenantFormValues>,
) {
  if (error instanceof HTTPError && error.response.status >= 500) {
    toast.error("Failed to create tenant");
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

  toast.error(apiError.error.message || "Failed to create tenant");
}

async function applyEditTenantErrors(
  error: unknown,
  setError: UseFormSetError<EditTenantFormValues>,
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

  toast.error(apiError.error.message || "Failed to update tenant");
}

export function TenantsContent() {
  const router = useRouter();
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Tenant | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Tenant | null>(null);

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
  } = useTenants(search);
  const createTenant = useCreateTenant();
  const updateTenant = useUpdateTenant();
  const deleteTenant = useDeleteTenant();

  useEffect(() => {
    if (error) toast.error("Failed to load tenants");
  }, [error]);

  const tenants = useMemo(
    () => data?.pages.flatMap((page) => page.items) ?? [],
    [data],
  );

  const columns: ColumnDef<Tenant>[] = [
    {
      accessorKey: "code",
      header: "Code",
      cell: ({ row }) => (
        <span className="font-mono text-[13px] font-medium">
          {row.original.code}
        </span>
      ),
    },
    {
      accessorKey: "name",
      header: "Name",
      cell: ({ row }) => (
        <span className="text-[13px]">{row.original.name}</span>
      ),
    },
    {
      accessorKey: "is_active",
      header: "Status",
      size: 120,
      cell: ({ row }) => (
        <StatusBadge status={row.original.is_active ? "active" : "disabled"} />
      ),
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
      cell: ({ row }) => (
        <div className="flex items-center justify-end gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={(event) => {
                  event.stopPropagation();
                  setEditTarget(row.original);
                }}
              >
                <Pencil className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Edit tenant</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-destructive"
                onClick={(event) => {
                  event.stopPropagation();
                  setDeleteTarget(row.original);
                }}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Delete tenant</TooltipContent>
          </Tooltip>
        </div>
      ),
    },
  ];

  const handleCreateTenant = async (values: CreateTenantInput) => {
    const created = await createTenant.mutateAsync(values);
    toast.success(`Tenant \"${created.name}\" created`);
  };

  const handleUpdateTenant = async (
    tenantCode: string,
    data: UpdateTenantInput,
  ) => {
    const updated = await updateTenant.mutateAsync({
      tenantCode,
      data,
    });
    if (typeof data.is_active === "boolean") {
      toast.success(
        data.is_active
          ? `Tenant \"${updated.name}\" enabled`
          : `Tenant \"${updated.name}\" disabled`,
      );
      return;
    }

    toast.success(`Tenant \"${updated.name}\" updated`);
  };

  const handleDeleteTenant = async () => {
    if (!deleteTarget) return;
    await deleteTenant.mutateAsync(deleteTarget.code);
    toast.success(`Tenant \"${deleteTarget.name}\" deleted`);
    setDeleteTarget(null);
  };

  return (
    <>
      <div className="mb-6 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="relative w-full md:max-w-sm">
          <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search tenants..."
            value={searchInput}
            onChange={(event) => setSearchInput(event.target.value)}
            className="h-9 pl-9"
          />
        </div>

        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Tenant
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={tenants}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        onRowClick={(tenant) => router.push(`/t/${tenant.code}`)}
        emptyState={
          <EmptyState
            icon={Building2}
            title="No tenants found"
            description={
              search
                ? "No tenant matches the current search."
                : "Create your first tenant to manage tenant dashboards, members, and workspaces."
            }
            action={
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Create Tenant
              </Button>
            }
          />
        }
      />

      <CreateTenantDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreateTenant={handleCreateTenant}
      />

      {editTarget && (
        <EditTenantDialog
          tenant={editTarget}
          open={!!editTarget}
          onOpenChange={(open) => {
            if (!open) setEditTarget(null);
          }}
          onUpdateTenant={handleUpdateTenant}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title="Delete tenant"
        description={
          deleteTarget
            ? `Soft-delete tenant \"${deleteTarget.name}\" (${deleteTarget.code})? Its workspace data remains in the backend, but the tenant will disappear from active lists.`
            : ""
        }
        confirmLabel="Delete tenant"
        onConfirm={handleDeleteTenant}
        loading={deleteTenant.isPending}
      />
    </>
  );
}

function CreateTenantDialog({
  open,
  onOpenChange,
  onCreateTenant,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreateTenant: (values: CreateTenantInput) => Promise<void>;
}) {
  const codeManuallyEdited = useRef(false);
  const form = useForm<CreateTenantFormValues>({
    resolver: zodResolver(createTenantSchema),
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
          await onCreateTenant(values);
          form.reset({ name: "", code: "" });
          codeManuallyEdited.current = false;
        } catch (error) {
          keepOpen = true;
          await applyCreateTenantErrors(error, form.setError);
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
      title="Create Tenant"
      description="Create a new tenant and its default _system workspace."
      submitLabel="Create Tenant"
      loadingLabel="Creating..."
      submitIcon={<Plus className="h-4 w-4" />}
      open={open}
      onOpenChange={onOpenChange}
      onSubmit={handleSubmit}
    >
      <div className="flex flex-col gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="tenant-name" className="text-[13px] font-medium">
            Name
          </Label>
          <Input id="tenant-name" {...form.register("name")} />
          {form.formState.errors.name && (
            <p className="text-xs text-destructive">
              {form.formState.errors.name.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="tenant-code" className="text-[13px] font-medium">
            Code
          </Label>
          <Input
            id="tenant-code"
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

function EditTenantDialog({
  tenant,
  open,
  onOpenChange,
  onUpdateTenant,
}: {
  tenant: Tenant;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUpdateTenant: (tenantCode: string, data: UpdateTenantInput) => Promise<void>;
}) {
  const form = useForm<EditTenantFormValues>({
    resolver: zodResolver(editTenantSchema),
    defaultValues: {
      name: tenant.name,
      status: tenant.is_active ? "active" : "disabled",
    },
  });
  const watchedStatus = useWatch({ control: form.control, name: "status" });

  useEffect(() => {
    form.reset({
      name: tenant.name,
      status: tenant.is_active ? "active" : "disabled",
    });
  }, [form, tenant]);

  const handleSubmit = async () => {
    let keepOpen = false;

    await form.handleSubmit(
      async (values) => {
        try {
          await onUpdateTenant(tenant.code, {
            name: values.name,
            is_active: values.status === "active",
          });
        } catch (error) {
          keepOpen = true;
          await applyEditTenantErrors(error, form.setError);
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
      title="Edit Tenant"
      description="Update the tenant display name and activation status. The tenant code remains immutable."
      submitLabel="Save Changes"
      loadingLabel="Saving..."
      submitIcon={<Pencil className="h-4 w-4" />}
      open={open}
      onOpenChange={onOpenChange}
      onSubmit={handleSubmit}
    >
      <div className="flex flex-col gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="edit-tenant-name" className="text-[13px] font-medium">
            Name
          </Label>
          <Input id="edit-tenant-name" {...form.register("name")} />
          {form.formState.errors.name && (
            <p className="text-xs text-destructive">
              {form.formState.errors.name.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="edit-tenant-code" className="text-[13px] font-medium">
            Code
          </Label>
          <Input id="edit-tenant-code" value={tenant.code} readOnly className="bg-muted/50 font-mono text-[13px] text-muted-foreground" />
        </div>

        <div className="space-y-1.5">
          <Label className="text-[13px] font-medium">Status</Label>
          <Select
            value={watchedStatus}
            onValueChange={(value) =>
              form.setValue("status", value as EditTenantFormValues["status"], {
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
