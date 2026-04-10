"use client";

import { useState, useRef, useCallback } from "react";
import {
  RefreshCw,
  Star,
  Trash2,
  Plus,
  Mail,
  Globe,
  Loader2,
  Share2,
  Lock,
} from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import {
  useIdentityList,
  useSyncIdentities,
  useCreateIdentity,
  useDeleteIdentity,
  useSetDefaultIdentity,
  useIdentityWorkspaceAccess,
  useUpdateIdentityWorkspaceAccess,
} from "@/hooks/use-identities";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { cn } from "@/lib/utils";
import { SYSTEM_WORKSPACE_SCOPE_LABEL } from "@/lib/system-workspace-display";
import type { Adapter, AdapterIdentity } from "@/types/adapters";
import { useWorkspacesManagement } from "@/hooks/use-workspaces-mgmt";

const STATUS_STYLES: Record<string, { className: string; label: string }> = {
  verified: { className: "bg-emerald-500/15 text-emerald-500 border-emerald-500/30", label: "Verified" },
  pending: { className: "bg-yellow-500/15 text-yellow-500 border-yellow-500/30", label: "Pending" },
  failed: { className: "bg-destructive/15 text-destructive border-destructive/30", label: "Failed" },
};

function StatusBadge({ status }: { status: string }) {
  const style = STATUS_STYLES[status] ?? STATUS_STYLES.failed;
  return (
    <Badge variant="outline" className={cn("text-[10px] px-1.5 py-0 shrink-0", style.className)}>
      {style.label}
    </Badge>
  );
}

function MarqueeText({ text }: { text: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const textRef = useRef<HTMLSpanElement>(null);

  const handleMouseEnter = useCallback(() => {
    const container = containerRef.current;
    const textEl = textRef.current;
    if (!container || !textEl) return;
    const overflow = textEl.scrollWidth - container.clientWidth;
    if (overflow > 0) {
      textEl.style.setProperty("--marquee-offset", `-${overflow}px`);
      textEl.classList.add("animate-marquee");
    }
  }, []);

  const handleMouseLeave = useCallback(() => {
    const textEl = textRef.current;
    if (!textEl) return;
    textEl.classList.remove("animate-marquee");
    textEl.style.transform = "";
  }, []);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div
          ref={containerRef}
          className="min-w-0 overflow-hidden cursor-default"
          onMouseEnter={handleMouseEnter}
          onMouseLeave={handleMouseLeave}
        >
          <span
            ref={textRef}
            className="text-sm font-mono whitespace-nowrap inline-block"
          >
            {text}
          </span>
        </div>
      </TooltipTrigger>
      <TooltipContent side="top" className="font-mono text-xs max-w-none">
        {text}
      </TooltipContent>
    </Tooltip>
  );
}

function EmailRow({
  identity,
  onSetDefault,
  onDelete,
  onManageAccess,
  isSettingDefault,
  canEdit,
  isSystemWorkspace,
}: {
  identity: AdapterIdentity;
  onSetDefault: () => void;
  onDelete: () => void;
  onManageAccess?: () => void;
  isSettingDefault: boolean;
  canEdit: boolean;
  isSystemWorkspace: boolean;
}) {
  return (
    <div className="flex items-center gap-2 py-1.5 pl-8 pr-2 rounded-md hover:bg-muted/50 group">
      <Mail className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />

      <div className="flex items-center gap-2 min-w-0 flex-1">
        <MarqueeText text={identity.identity} />
        {identity.is_default && (
          <Badge variant="outline" className="text-[10px] px-1.5 py-0 bg-primary/10 text-primary border-primary/30 shrink-0">
            Default
          </Badge>
        )}
        {isSystemWorkspace && identity.granted_workspace_count > 0 && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-scope-system-bg text-[10px] font-medium text-scope-system px-1 shrink-0">
                {identity.granted_workspace_count}
              </span>
            </TooltipTrigger>
            <TooltipContent>
              Shared with {identity.granted_workspace_count} workspace{identity.granted_workspace_count !== 1 ? "s" : ""}
            </TooltipContent>
          </Tooltip>
        )}
      </div>

      {identity.display_name && (
        <span className="text-[10px] text-muted-foreground truncate max-w-[80px] shrink-0">
          {identity.display_name}
        </span>
      )}

      <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
        {isSystemWorkspace && onManageAccess && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={onManageAccess}
              >
                <Share2 className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Workspace access</TooltipContent>
          </Tooltip>
        )}
        {canEdit && identity.status === "verified" && !identity.is_default && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={onSetDefault}
                disabled={isSettingDefault}
              >
                <Star className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Set as default sender</TooltipContent>
          </Tooltip>
        )}
        {!canEdit && (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex items-center text-muted-foreground">
                <Lock className="h-3 w-3" />
              </span>
            </TooltipTrigger>
            <TooltipContent>Shared adapter — read only in this workspace</TooltipContent>
          </Tooltip>
        )}
        {canEdit && identity.source === "manual" && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6 text-destructive"
                onClick={onDelete}
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Remove</TooltipContent>
          </Tooltip>
        )}
      </div>
    </div>
  );
}

function DomainAddInput({
  domain,
  adapterId,
  scopedPath,
  disabled,
}: {
  domain: string;
  adapterId: string;
  scopedPath: string;
  disabled?: boolean;
}) {
  const [localPart, setLocalPart] = useState("");
  const create = useCreateIdentity(scopedPath, adapterId);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const cleaned = localPart.trim().replace(/@.*$/, "");
    if (!cleaned) return;
    create.mutate({ identity: `${cleaned}@${domain}` }, {
      onSuccess: () => setLocalPart(""),
    });
  }

  return (
    <form onSubmit={handleSubmit} className="flex items-center gap-1.5 pl-8 pr-2 py-1">
      <fieldset disabled={create.isPending} className="flex items-center gap-1.5 min-w-0 flex-1">
        <div className="flex items-center min-w-0 flex-1 rounded-md border border-input bg-transparent h-7 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-1">
          <input
            value={localPart}
            onChange={(e) => setLocalPart(e.target.value.replace(/@/g, ""))}
            placeholder="user"
            className="w-full bg-transparent px-2 text-xs font-mono outline-none placeholder:text-muted-foreground disabled:opacity-50"
          />
          <span className="text-xs font-mono text-muted-foreground pr-2 shrink-0 select-none">
            @{domain}
          </span>
        </div>
        <Button
          type="submit"
          size="sm"
          variant="ghost"
          className="shrink-0 h-7 px-2 text-xs gap-1 text-muted-foreground hover:text-foreground"
          disabled={disabled || !localPart.trim()}
        >
          {create.isPending ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            <><Plus className="h-3 w-3" /> Add</>
          )}
        </Button>
      </fieldset>
    </form>
  );
}

function DomainTree({
  domainIdentity,
  childEmails,
  adapterId,
  scopedPath,
  onSetDefault,
  onDelete,
  isSettingDefault,
  canEdit,
  isSystemWorkspace,
  onManageAccess,
}: {
  domainIdentity: AdapterIdentity;
  childEmails: AdapterIdentity[];
  adapterId: string;
  scopedPath: string;
  onSetDefault: (id: string) => void;
  onDelete: (identity: AdapterIdentity) => void;
  isSettingDefault: boolean;
  canEdit: boolean;
  isSystemWorkspace: boolean;
  onManageAccess: (identity: AdapterIdentity) => void;
}) {
  return (
    <div className="py-1">
      {/* Domain header */}
      <div className="flex items-center gap-2 py-2 px-2">
        <Globe className="h-4 w-4 text-primary shrink-0" />
        <span className="text-sm font-mono font-medium">{domainIdentity.identity}</span>
        <StatusBadge status={domainIdentity.status} />
        {isSystemWorkspace && (
          <span className="text-[10px] text-muted-foreground">
            Share specific emails, not the domain
          </span>
        )}
      </div>

      {/* Child emails */}
      {childEmails.length > 0 && (
        <div className="border-l border-border/50 ml-4">
          {childEmails.map((email) => (
            <EmailRow
              key={email.id}
              identity={email}
              onSetDefault={() => onSetDefault(email.id)}
              onDelete={() => onDelete(email)}
              onManageAccess={() => onManageAccess(email)}
              isSettingDefault={isSettingDefault}
              canEdit={canEdit}
              isSystemWorkspace={isSystemWorkspace}
            />
          ))}
        </div>
      )}

      {/* Add email under this domain */}
      {canEdit && domainIdentity.status === "verified" && (
        <div className="border-l border-border/50 ml-4">
          <DomainAddInput
            domain={domainIdentity.identity}
            adapterId={adapterId}
            scopedPath={scopedPath}
            disabled={!canEdit}
          />
        </div>
      )}
    </div>
  );
}

export function IdentityPanel({
  adapter,
  open,
  onOpenChange,
}: {
  adapter: Adapter;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scopedPath = useScopedPath();
  const scope = useScope();
  const isSystemWorkspace = scope.workspaceCode === SYSTEM_WORKSPACE_CODE;
  const isReadOnly = !adapter.is_editable;
  const [deleteTarget, setDeleteTarget] = useState<AdapterIdentity | null>(null);
  const [shareTarget, setShareTarget] = useState<AdapterIdentity | null>(null);

  const { data: identities, isLoading } = useIdentityList(scopedPath, adapter.id);
  const sync = useSyncIdentities(scopedPath, adapter.id);
  const setDefault = useSetDefaultIdentity(scopedPath, adapter.id);
  const remove = useDeleteIdentity(scopedPath, adapter.id);

  const isBusy = sync.isPending || setDefault.isPending || remove.isPending;

  // Group emails under their parent domain
  const allIdentities = identities ?? [];
  const domains = allIdentities.filter((i) => i.identity_type === "domain");
  const emails = allIdentities.filter((i) => i.identity_type === "email");

  function emailsForDomain(domain: string) {
    return emails.filter((e) => e.identity.endsWith(`@${domain}`));
  }

  // Emails not belonging to any domain (orphans — shouldn't happen with new sync, but defensive)
  const domainNames = new Set(domains.map((d) => d.identity));
  const orphanEmails = emails.filter((e) => {
    const parts = e.identity.split("@");
    return parts.length !== 2 || !domainNames.has(parts[1]);
  });

  return (
    <>
      <Dialog open={open} onOpenChange={(v) => !isBusy && onOpenChange(v)}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] flex flex-col" onInteractOutside={(e) => isBusy && e.preventDefault()}>
          <DialogHeader className="shrink-0">
            <DialogTitle>Sender Identities — {adapter.name}</DialogTitle>
            <DialogDescription>
              Verified domains and sender addresses. Add emails under each domain.
            </DialogDescription>
            <div className="pt-1">
              {adapter.is_editable ? (
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-1.5"
                  onClick={() => sync.mutate()}
                  disabled={sync.isPending}
                >
                  <RefreshCw className={cn("h-3.5 w-3.5", sync.isPending && "animate-spin")} />
                  Sync from provider
                </Button>
              ) : (
                <div className="inline-flex items-center gap-2 rounded-md bg-scope-system-bg px-2.5 py-1 text-xs text-scope-system">
                  <Lock className="h-3.5 w-3.5" />
                  {`Shared from ${SYSTEM_WORKSPACE_SCOPE_LABEL} — read only`}
                </div>
              )}
            </div>
          </DialogHeader>

          <div className="overflow-y-auto min-h-0 -mx-6 px-6 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-muted-foreground/15 hover:[&::-webkit-scrollbar-thumb]:bg-muted-foreground/30 [&::-webkit-scrollbar-thumb]:rounded-full">
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            ) : domains.length === 0 && orphanEmails.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-10 text-center">
                <Globe className="h-8 w-8 text-muted-foreground/30" />
                <div>
                  <p className="text-sm font-medium">No domains found</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    Click &quot;Sync from provider&quot; to import verified domains from SES.
                  </p>
                </div>
              </div>
            ) : (
              <div className="flex flex-col divide-y">
                {domains.map((domain) => (
                  <DomainTree
                    key={domain.id}
                    domainIdentity={domain}
                    childEmails={emailsForDomain(domain.identity)}
                    adapterId={adapter.id}
                    scopedPath={scopedPath}
                    onSetDefault={(id) => setDefault.mutate(id)}
                    onDelete={setDeleteTarget}
                    isSettingDefault={setDefault.isPending}
                    canEdit={!isReadOnly}
                    isSystemWorkspace={isSystemWorkspace}
                    onManageAccess={setShareTarget}
                  />
                ))}
                {orphanEmails.map((email) => (
                  <EmailRow
                    key={email.id}
                    identity={email}
                    onSetDefault={() => setDefault.mutate(email.id)}
                    onDelete={() => setDeleteTarget(email)}
                    onManageAccess={() => setShareTarget(email)}
                    isSettingDefault={setDefault.isPending}
                    canEdit={!isReadOnly}
                    isSystemWorkspace={isSystemWorkspace}
                  />
                ))}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Remove Identity"
        description={`Remove "${deleteTarget?.identity}" from this adapter? This only removes the local reference — the identity remains in your AWS account.`}
        confirmLabel="Remove"
        onConfirm={() => {
          if (deleteTarget) {
            remove.mutate(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        loading={remove.isPending}
      />

      {shareTarget && isSystemWorkspace && (
        <IdentityWorkspaceAccessDialog
          adapter={adapter}
          identity={shareTarget}
          open={!!shareTarget}
          onOpenChange={(next) => !next && setShareTarget(null)}
        />
      )}
    </>
  );
}

function IdentityWorkspaceAccessDialog({
  adapter,
  identity,
  open,
  onOpenChange,
}: {
  adapter: Adapter;
  identity: AdapterIdentity;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const scope = useScope();
  const scopedPath = useScopedPath();
  const tenantCode = scope.tenantCode ?? "";
  const { data: access, isLoading } = useIdentityWorkspaceAccess(scopedPath, adapter.id, identity.id);
  const updateAccess = useUpdateIdentityWorkspaceAccess(scopedPath, adapter.id, identity.id);
  const { data: workspacePages } = useWorkspacesManagement(tenantCode, "");
  const [selected, setSelected] = useState<string[] | null>(null);

  const allWorkspaces = workspacePages?.pages.flatMap((page) => page.items).filter((workspace) => !workspace.is_system) ?? [];
  const effectiveSelection = selected
    ? selected
    : access?.items.filter((item) => item.is_granted).map((item) => item.workspace_id) ?? [];
  const items = access?.items.length
    ? access.items
    : allWorkspaces.map((workspace) => ({
        workspace_id: workspace.id,
        code: workspace.code,
        name: workspace.name,
        is_granted: false,
      }));

  function toggle(workspaceId: string) {
    setSelected((current) =>
      (current ?? effectiveSelection).includes(workspaceId)
        ? (current ?? effectiveSelection).filter((id) => id !== workspaceId)
        : [...(current ?? effectiveSelection), workspaceId]
    );
  }

  async function handleSave() {
    await updateAccess.mutateAsync(effectiveSelection);
    setSelected(null);
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !updateAccess.isPending && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Workspace access — {identity.identity}</DialogTitle>
          <DialogDescription>
            Choose which workspaces can use this SES sender identity.
          </DialogDescription>
        </DialogHeader>
        <div className="flex max-h-[50vh] flex-col gap-3 overflow-y-auto py-2">
          {identity.identity_type === "domain" ? (
            <p className="text-sm text-muted-foreground">
              Domain identities are not shareable. Share specific email senders instead.
            </p>
          ) : isLoading ? (
            <p className="text-sm text-muted-foreground">Loading workspaces...</p>
          ) : items.length === 0 ? (
            <p className="text-sm text-muted-foreground">No child workspaces available.</p>
          ) : (
            items.map((item) => {
              const checked = effectiveSelection.includes(item.workspace_id);
              return (
                <label key={item.workspace_id} className="flex items-center justify-between rounded-md border px-3 py-2">
                  <div className="flex flex-col">
                    <span className="text-sm font-medium">{item.name}</span>
                    <span className="text-xs text-muted-foreground">{item.code}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggle(item.workspace_id)}
                    className="h-4 w-4"
                  />
                </label>
              );
            })
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={updateAccess.isPending}>
            Cancel
          </Button>
          <Button
            onClick={handleSave}
            disabled={updateAccess.isPending || identity.identity_type === "domain"}
          >
            Save access
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
