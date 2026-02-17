"use client";

import { useState, useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import type { ColumnDef } from "@tanstack/react-table";
import { Mail } from "lucide-react";
import { toast } from "sonner";
import { useScope } from "@/hooks/use-scope";
import { useEmails } from "@/hooks/use-emails";
import { DataTable } from "@/components/shared/data-table";
import { StatusBadge } from "@/components/shared/status-badge";
import { EmptyState } from "@/components/shared/empty-state";
import { EmailFiltersBar } from "@/components/emails/email-filters";
import { formatDate } from "@/lib/utils";
import type { Email, EmailFilters } from "@/types/emails";

function buildDetailPath(
  scope: ReturnType<typeof useScope>,
  trackingId: string
): string {
  if (scope.level === "workspace") {
    return `/t/${scope.tenantCode}/w/${scope.workspaceCode}/emails/${trackingId}`;
  }
  if (scope.level === "tenant") {
    return `/t/${scope.tenantCode}/emails/${trackingId}`;
  }
  return `/global/emails/${trackingId}`;
}

const columns: ColumnDef<Email, unknown>[] = [
  {
    accessorKey: "recipient_email",
    header: "RECIPIENT",
    cell: ({ row }) => (
      <span className="font-mono text-[13px] text-foreground">
        {row.original.recipient_email}
      </span>
    ),
  },
  {
    accessorKey: "template_type_slug",
    header: "TEMPLATE",
    size: 160,
    cell: ({ row }) => (
      <span className="font-mono text-[13px] text-muted-foreground">
        {row.original.template_type_slug}
      </span>
    ),
  },
  {
    accessorKey: "status",
    header: "STATUS",
    size: 120,
    enableSorting: false,
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  },
  {
    accessorKey: "created_at",
    header: "DATE",
    size: 180,
    cell: ({ row }) => (
      <span className="font-mono text-xs text-muted-foreground">
        {formatDate(row.original.created_at)}
      </span>
    ),
  },
  {
    accessorKey: "tracking_id",
    header: "TRACKING ID",
    size: 200,
    enableSorting: false,
    cell: ({ row }) => (
      <span className="font-mono text-xs text-muted-foreground">
        {row.original.tracking_id}
      </span>
    ),
  },
];

export function EmailsContent() {
  const scope = useScope();

  if (scope.level !== "workspace") {
    return (
      <EmptyState
        icon={Mail}
        title="Select a workspace"
        description="Emails are workspace-scoped. Select a workspace from the sidebar to view emails."
      />
    );
  }

  return <EmailsTable />;
}

function EmailsTable() {
  const [filters, setFilters] = useState<EmailFilters>({});
  const scope = useScope();
  const router = useRouter();
  const {
    data,
    isLoading,
    error,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useEmails(filters);

  useEffect(() => {
    if (error) toast.error("Failed to load emails");
  }, [error]);

  const emails = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data]
  );

  const handleRowClick = (email: Email) => {
    router.push(buildDetailPath(scope, email.tracking_id));
  };

  return (
    <div className="flex flex-col gap-6">
      <EmailFiltersBar filters={filters} onFiltersChange={setFilters} />
      <DataTable
        columns={columns}
        data={emails}
        loading={isLoading}
        hasMore={hasNextPage ?? false}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        onRowClick={handleRowClick}
        emptyState={
          <EmptyState
            icon={Mail}
            title="No emails found"
            description="No emails match your current filters. Try adjusting your search criteria or send your first email."
          />
        }
      />
    </div>
  );
}
