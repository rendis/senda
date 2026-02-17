"use client";

import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/shared/status-badge";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { EmptyState } from "@/components/shared/empty-state";
import { DataTable } from "@/components/shared/data-table";
import { PageShell } from "@/components/shared/page-shell";
import { Mail, Plus, LayoutDashboard } from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";

/* ── Mock data for DataTable ─────────────────────────────── */
interface MockEmail {
  id: string;
  recipient: string;
  template: string;
  status: string;
  date: string;
}

const mockEmails: MockEmail[] = [
  { id: "1", recipient: "alice@example.com", template: "welcome-email", status: "delivered", date: "2026-02-17T10:30:00Z" },
  { id: "2", recipient: "bob@example.com", template: "reset-password", status: "sent", date: "2026-02-17T09:15:00Z" },
  { id: "3", recipient: "carol@example.com", template: "invoice", status: "bounced", date: "2026-02-16T14:00:00Z" },
  { id: "4", recipient: "dave@example.com", template: "welcome-email", status: "queued", date: "2026-02-17T11:00:00Z" },
  { id: "5", recipient: "eve@example.com", template: "newsletter", status: "complained", date: "2026-02-15T08:45:00Z" },
];

const mockColumns: ColumnDef<MockEmail>[] = [
  { accessorKey: "recipient", header: "Recipient" },
  { accessorKey: "template", header: "Template" },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => (
      <StatusBadge status={row.getValue("status") as "delivered" | "sent" | "bounced" | "queued" | "complained"} />
    ),
  },
  { accessorKey: "date", header: "Date" },
];

/* ── Page ─────────────────────────────────────────────────── */
export default function DesignSystemPage() {
  return (
    <PageShell
      title="Design System"
      description="Component showcase — compare vs Pencil"
      breadcrumbs={[{ label: "Global" }, { label: "Design System" }]}
    >
      <div className="space-y-12">
        {/* ── Badges ─────────────────────────────────────── */}
        <section>
          <h2 className="text-lg font-semibold mb-4">Status Badges</h2>
          <div className="flex flex-wrap gap-3">
            <StatusBadge status="queued" />
            <StatusBadge status="processing" />
            <StatusBadge status="sent" />
            <StatusBadge status="delivered" />
            <StatusBadge status="opened" />
            <StatusBadge status="bounced" />
            <StatusBadge status="complained" />
            <StatusBadge status="failed" />
            <StatusBadge status="suppressed" />
            <StatusBadge status="draft" />
            <StatusBadge status="published" />
            <StatusBadge status="archived" />
            <StatusBadge status="pending" />
            <StatusBadge status="verified" />
            <StatusBadge status="error" />
            <StatusBadge status="active" />
            <StatusBadge status="disabled" />
            <StatusBadge status="revoked" />
          </div>
        </section>

        {/* ── Scope Indicators ───────────────────────────── */}
        <section>
          <h2 className="text-lg font-semibold mb-4">Scope Indicators</h2>
          <div className="flex flex-wrap gap-3">
            <ScopeIndicator scope="global" />
            <ScopeIndicator scope="system" />
            <ScopeIndicator scope="tenant" label="acme-corp" />
            <ScopeIndicator scope="workspace" label="production" />
          </div>
        </section>

        {/* ── Buttons ────────────────────────────────────── */}
        <section>
          <h2 className="text-lg font-semibold mb-4">Buttons</h2>
          <div className="flex flex-wrap gap-3">
            <Button>Primary</Button>
            <Button variant="secondary">Secondary</Button>
            <Button variant="outline">Outline</Button>
            <Button variant="destructive">Destructive</Button>
            <Button variant="ghost">Ghost</Button>
            <Button size="sm">Small</Button>
            <Button size="lg">Large</Button>
            <Button>
              <Plus className="h-4 w-4" />
              With Icon
            </Button>
          </div>
        </section>

        {/* ── Metric Cards ───────────────────────────────── */}
        <section>
          <h2 className="text-lg font-semibold mb-4">Metric Cards</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="rounded-lg border bg-card p-5">
              <div className="flex items-center justify-between">
                <p className="text-xs font-medium font-mono text-muted-foreground">Total Emails</p>
                <Mail className="h-4 w-4 text-muted-foreground" />
              </div>
              <p className="text-[28px] font-semibold tracking-[-2px] mt-3">12,847</p>
              <div className="flex items-center gap-1.5 mt-3">
                <span className="text-xs font-mono font-medium text-status-delivered">+12.5%</span>
                <span className="text-xs font-mono text-muted-foreground">vs last 7 days</span>
              </div>
            </div>
            <div className="rounded-lg border bg-card p-5">
              <div className="flex items-center justify-between">
                <p className="text-xs font-medium font-mono text-muted-foreground">Delivery Rate</p>
                <Mail className="h-4 w-4 text-muted-foreground" />
              </div>
              <p className="text-[28px] font-semibold tracking-[-2px] mt-3">98.2%</p>
              <div className="flex items-center gap-1.5 mt-3">
                <span className="text-xs font-mono font-medium text-status-delivered">+0.3%</span>
                <span className="text-xs font-mono text-muted-foreground">vs last 7 days</span>
              </div>
            </div>
            <div className="rounded-lg border bg-card p-5">
              <div className="flex items-center justify-between">
                <p className="text-xs font-medium font-mono text-muted-foreground">Bounce Rate</p>
                <Mail className="h-4 w-4 text-muted-foreground" />
              </div>
              <p className="text-[28px] font-semibold tracking-[-2px] mt-3">1.2%</p>
              <div className="flex items-center gap-1.5 mt-3">
                <span className="text-xs font-mono font-medium text-status-bounced">+0.1%</span>
                <span className="text-xs font-mono text-muted-foreground">vs last 7 days</span>
              </div>
            </div>
            <div className="rounded-lg border bg-card p-5">
              <div className="flex items-center justify-between">
                <p className="text-xs font-medium font-mono text-muted-foreground">Complaint Rate</p>
                <Mail className="h-4 w-4 text-muted-foreground" />
              </div>
              <p className="text-[28px] font-semibold tracking-[-2px] mt-3">0.05%</p>
              <div className="flex items-center gap-1.5 mt-3">
                <span className="text-xs font-mono font-medium text-status-delivered">-0.01%</span>
                <span className="text-xs font-mono text-muted-foreground">vs last 7 days</span>
              </div>
            </div>
          </div>
        </section>

        {/* ── Empty State ────────────────────────────────── */}
        <section>
          <h2 className="text-lg font-semibold mb-4">Empty State</h2>
          <div className="rounded-lg border bg-card">
            <EmptyState
              icon={LayoutDashboard}
              title="No emails sent yet"
              description="Configure a template and send your first email via the API."
              action={<Button>Get Started</Button>}
            />
          </div>
        </section>

        {/* ── Data Table ─────────────────────────────────── */}
        <section>
          <h2 className="text-lg font-semibold mb-4">Data Table</h2>
          <DataTable columns={mockColumns} data={mockEmails} />
        </section>
      </div>
    </PageShell>
  );
}
