"use client";

import { useQuery } from "@tanstack/react-query";
import { HTTPError } from "ky";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/shared/empty-state";
import { ScopeIndicator } from "@/components/shared/scope-indicator";
import { MjmlEditor } from "@/components/templates/mjml-editor";
import { ExternalEmbedTokenBridge } from "@/components/templates/external-embed-token-bridge";
import type { ScopeContext } from "@/types/api";
import { ShieldAlert, LoaderCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import {
  resolveExternalTemplateBuilderViewState,
  type ExternalEmbedSessionState,
} from "@/lib/external-template-builder";
import { useApi, useApiReady } from "@/hooks/use-api";

interface ExternalTemplateBuilderShellProps {
  profileSlug: string;
  templateSlug: string;
  scope: ScopeContext;
  fallbackToSystem?: boolean;
}

export function ExternalTemplateBuilderShell({
  profileSlug,
  templateSlug,
  scope,
  fallbackToSystem,
}: ExternalTemplateBuilderShellProps) {
  const t = useTranslations("externalBuilderPage");
  const api = useApi();
  const apiReady = useApiReady();
  const sessionPath =
    scope.tenantCode && scope.workspaceCode
      ? `external/${profileSlug}/tenants/${scope.tenantCode}/workspaces/${scope.workspaceCode}/session`
      : null;
  const sessionQuery = useQuery({
    queryKey: ["external-builder-session", sessionPath, scope.environment],
    queryFn: () => api.get(sessionPath!).json<ExternalEmbedSessionState>(),
    enabled: apiReady && !!sessionPath,
    retry: false,
  });
  const accessDenied =
    sessionQuery.error instanceof HTTPError &&
    (sessionQuery.error.response.status === 401 ||
      sessionQuery.error.response.status === 403);
  const viewState = resolveExternalTemplateBuilderViewState({
    profileSlug,
    scope,
    fallbackToSystem,
    session: sessionQuery.data,
    accessDenied,
  });

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <ExternalEmbedTokenBridge />
      <header className="border-b bg-card/70 px-6 py-4 backdrop-blur">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-lg font-semibold tracking-[-0.02em]">
                {t("title")}
              </h1>
              <Badge variant="outline">{profileSlug}</Badge>
              <Badge variant="secondary">{templateSlug}</Badge>
              <Badge
                variant="outline"
                className={
                  scope.environment === "test"
                    ? "border-amber-500/60 bg-amber-500/10 text-amber-900 dark:text-amber-100"
                    : "border-emerald-500/60 bg-emerald-500/10 text-emerald-900 dark:text-emerald-100"
                }
              >
                {(scope.environment ?? "prod").toUpperCase()}
              </Badge>
              <ScopeIndicator
                scope={viewState.readOnly ? "system" : "workspace"}
                label={viewState.workspaceLabel}
              />
              {viewState.readOnly ? (
                <Badge variant="secondary">{t("readOnlyBadge")}</Badge>
              ) : null}
            </div>
            <p className="text-sm text-muted-foreground">{t("description")}</p>
          </div>
          <div className="text-right text-xs text-muted-foreground">
            <p className="font-medium text-foreground">{t("profileLabel")}</p>
            <p>{profileSlug}</p>
          </div>
        </div>
      </header>

      {viewState.readOnly && !viewState.accessDenied ? (
        <div className="border-b border-sky-500/30 bg-sky-500/10 px-6 py-3 text-sm text-sky-900 dark:text-sky-100">
          <p className="font-medium">{t("readOnlyFallbackTitle")}</p>
          <p>{t("readOnlyFallbackDescription")}</p>
        </div>
      ) : null}

      <div className="flex-1 min-h-0">
        {sessionQuery.isLoading || (apiReady && sessionPath && !sessionQuery.data && !sessionQuery.error) ? (
          <div className="flex h-full items-center justify-center px-6 py-10">
            <div className="flex items-center gap-3 text-sm text-muted-foreground">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              <span>{t("loadingAccessState")}</span>
            </div>
          </div>
        ) : null}
        {viewState.accessDenied ? (
          <EmptyState
            icon={ShieldAlert}
            title={t("accessDenied.title")}
            description={t("accessDenied.description")}
            className="py-20"
          />
        ) : null}
        {!viewState.accessDenied && !(sessionQuery.isLoading || (apiReady && sessionPath && !sessionQuery.data && !sessionQuery.error)) ? (
        <MjmlEditor
          embedded
          forceReadOnly={viewState.readOnly}
          lockEditing={!viewState.canEdit}
          canPublish={viewState.canPublish}
          canSendTest={viewState.canTestSend}
          showBulkSend={false}
        />
        ) : null}
      </div>
    </div>
  );
}
