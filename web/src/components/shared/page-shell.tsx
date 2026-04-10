"use client";

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { emitOpenScopeSwitcher } from "@/lib/scope-switcher-events";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { Fragment } from "react";

function escapeRegExp(input: string) {
  return input.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Pattern: /t/{tenantCode}/w/{SYSTEM_WORKSPACE_CODE} */
const TENANT_BREADCRUMB_RE = new RegExp(
  `^/t/([^/]+)/w/${escapeRegExp(SYSTEM_WORKSPACE_CODE)}$`,
);

interface BreadcrumbItemDef {
  label: string;
  href?: string;
}

interface PageShellProps {
  title: string;
  description?: string;
  breadcrumbs?: BreadcrumbItemDef[];
  actions?: React.ReactNode;
  children: React.ReactNode;
}

export function PageShell({
  title,
  description,
  breadcrumbs,
  actions,
  children,
}: PageShellProps) {
  function handleBreadcrumbClick(item: BreadcrumbItemDef, e: React.MouseEvent) {
    // Tenant breadcrumb → open scope switcher in workspaces view
    const match = item.href?.match(TENANT_BREADCRUMB_RE);
    if (match) {
      e.preventDefault();
      emitOpenScopeSwitcher({
        view: "workspaces",
        tenantCode: match[1],
        tenantName: item.label,
      });
      return;
    }

    // Non-scope item without real href → open scope switcher in tenants view
    if (!item.href || item.href === "#") {
      e.preventDefault();
      emitOpenScopeSwitcher({ view: "tenants" });
    }
  }

  return (
    <div className="flex flex-1 flex-col">
      <div className="flex flex-col gap-4 p-8 pb-0">
        {breadcrumbs && breadcrumbs.length > 0 && (
          <Breadcrumb>
            <BreadcrumbList>
              {breadcrumbs.map((item, idx) => (
                <Fragment key={idx}>
                  {idx > 0 && <BreadcrumbSeparator />}
                  <BreadcrumbItem>
                    {idx === breadcrumbs.length - 1 ? (
                      <BreadcrumbPage>{item.label}</BreadcrumbPage>
                    ) : (
                      <BreadcrumbLink
                        href={item.href ?? "#"}
                        onClick={(e) => handleBreadcrumbClick(item, e)}
                      >
                        {item.label}
                      </BreadcrumbLink>
                    )}
                  </BreadcrumbItem>
                </Fragment>
              ))}
            </BreadcrumbList>
          </Breadcrumb>
        )}
        <div className="flex items-center justify-between">
          <div>
            <h1
              className="text-2xl font-semibold tracking-tight"
              style={{ letterSpacing: "-1px" }}
            >
              {title}
            </h1>
            {description && (
              <p className="text-xs font-mono text-muted-foreground mt-1">
                {description}
              </p>
            )}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      </div>
      <div className="flex-1 p-8">{children}</div>
    </div>
  );
}
