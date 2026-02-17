"use client";

import { useState } from "react";
import {
  Globe,
  Plus,
  MoreHorizontal,
  ArrowLeft,
  Trash2,
  Eye,
  RefreshCw,
} from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useDomainList,
  useDomainDetail,
  useRegisterDomain,
  useDeleteDomain,
  useVerifyDomain,
} from "@/hooks/use-domains";
import { DataTable } from "@/components/shared/data-table";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { StatusBadge } from "@/components/shared/status-badge";
import { FormDialog } from "@/components/shared/form-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { DomainDetail } from "./domain-detail";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { formatDate } from "@/lib/utils";
import type { ColumnDef } from "@tanstack/react-table";
import type { Domain } from "@/types/domains";

export function DomainsContent() {
  const { level } = useScope();

  if (level === "tenant") {
    return (
      <EmptyState
        icon={Globe}
        title="Select a workspace"
        description="Domains are workspace-scoped. Select a workspace from the sidebar to manage domains."
      />
    );
  }

  return <DomainsTable />;
}

function DomainsTable() {
  const scopedPath = useScopedPath();
  const [search, setSearch] = useState("");
  const [selectedDomainId, setSelectedDomainId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Domain | null>(null);
  const [newDomain, setNewDomain] = useState("");

  const {
    data: listData,
    isLoading,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useDomainList(scopedPath);

  const { data: domainDetail, isLoading: detailLoading } = useDomainDetail(
    scopedPath,
    selectedDomainId ?? ""
  );

  const registerDomain = useRegisterDomain(scopedPath);
  const deleteDomain = useDeleteDomain(scopedPath);
  const verifyDomain = useVerifyDomain(scopedPath);

  const allItems = listData?.pages.flatMap((p) => p.items) ?? [];
  const items = search
    ? allItems.filter((d) =>
        d.domain_name.toLowerCase().includes(search.toLowerCase())
      )
    : allItems;

  async function handleRegister() {
    await registerDomain.mutateAsync({ domain_name: newDomain });
    setNewDomain("");
  }

  // Detail view
  if (selectedDomainId) {
    return (
      <div className="flex flex-col gap-6">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setSelectedDomainId(null)}
          className="w-fit gap-2"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Domains
        </Button>

        {detailLoading ? (
          <div className="flex flex-col gap-4">
            <Skeleton className="h-8 w-64" />
            <Skeleton className="h-32 w-full" />
          </div>
        ) : domainDetail ? (
          <DomainDetail
            domain={domainDetail}
            onVerify={() => verifyDomain.mutate(domainDetail.id)}
            verifying={verifyDomain.isPending}
          />
        ) : null}
      </div>
    );
  }

  const columns: ColumnDef<Domain, unknown>[] = [
    {
      accessorKey: "domain_name",
      header: "DOMAIN",
      cell: ({ row }) => (
        <button
          onClick={() => setSelectedDomainId(row.original.id)}
          className="font-mono text-[13px] font-medium text-foreground hover:text-primary transition-colors text-left"
        >
          {row.original.domain_name}
        </button>
      ),
    },
    {
      accessorKey: "status",
      header: "STATUS",
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: "scope",
      header: "SCOPE",
      cell: () => <ScopeIndicator scope="workspace" />,
    },
    {
      accessorKey: "last_check_at",
      header: "LAST CHECK",
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">
          {row.original.last_check_at
            ? formatDate(row.original.last_check_at)
            : "\u2014"}
        </span>
      ),
    },
    {
      id: "actions",
      enableSorting: false,
      cell: ({ row }) => (
        <DomainActions
          domain={row.original}
          onView={() => setSelectedDomainId(row.original.id)}
          onDelete={setDeleteTarget}
          onVerify={() => verifyDomain.mutate(row.original.id)}
        />
      ),
    },
  ];

  const registerTrigger = (
    <Button className="gap-2">
      <Plus className="h-4 w-4" />
      Add Domain
    </Button>
  );

  return (
    <div className="flex flex-col gap-6">
      {/* Toolbar */}
      <div className="flex items-center justify-between">
        <Input
          placeholder="Search domains..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-[280px]"
        />
        <FormDialog
          trigger={registerTrigger}
          title="Register Domain"
          description="Add a new sending domain. DNS records will be generated automatically."
          submitLabel="Register"
          onSubmit={handleRegister}
        >
          <div className="flex flex-col gap-2">
            <Label htmlFor="domain-name">Domain Name</Label>
            <Input
              id="domain-name"
              value={newDomain}
              onChange={(e) => setNewDomain(e.target.value)}
              placeholder="mail.example.com"
              className="font-mono"
            />
          </div>
        </FormDialog>
      </div>

      {/* Table */}
      <DataTable
        columns={columns}
        data={items}
        loading={isLoading}
        hasMore={hasNextPage}
        onLoadMore={() => fetchNextPage()}
        loadingMore={isFetchingNextPage}
        emptyState={
          <EmptyState
            icon={Globe}
            title="No domains registered"
            description="Register a sending domain to enable DKIM signing and improve email deliverability."
            action={
              <FormDialog
                trigger={registerTrigger}
                title="Register Domain"
                description="Add a new sending domain. DNS records will be generated automatically."
                submitLabel="Register"
                onSubmit={handleRegister}
              >
                <div className="flex flex-col gap-2">
                  <Label htmlFor="domain-name-empty">Domain Name</Label>
                  <Input
                    id="domain-name-empty"
                    value={newDomain}
                    onChange={(e) => setNewDomain(e.target.value)}
                    placeholder="mail.example.com"
                    className="font-mono"
                  />
                </div>
              </FormDialog>
            }
          />
        }
      />

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Remove Domain"
        description={`Are you sure you want to remove "${deleteTarget?.domain_name}"? This will disable DKIM signing for this domain.`}
        confirmLabel="Remove"
        onConfirm={() => {
          if (deleteTarget) {
            deleteDomain.mutate(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        loading={deleteDomain.isPending}
      />
    </div>
  );
}

function DomainActions({
  domain,
  onView,
  onDelete,
  onVerify,
}: {
  domain: Domain;
  onView: () => void;
  onDelete: (d: Domain) => void;
  onVerify: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
          <MoreHorizontal className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onView}>
          <Eye className="mr-2 h-4 w-4" />
          View Details
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onVerify}>
          <RefreshCw className="mr-2 h-4 w-4" />
          Verify Now
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => onDelete(domain)}
          className="text-destructive"
        >
          <Trash2 className="mr-2 h-4 w-4" />
          Remove
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
