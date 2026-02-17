"use client";

import { ScopeIndicator } from "@/components/shared/scope-indicator";
import type { ScopeLevel } from "@/types/api";

interface ChainLevel {
  scope: ScopeLevel | "system";
  value: unknown;
  inherited?: boolean;
}

interface ResolutionChainViewerProps {
  levels: ChainLevel[];
}

export function ResolutionChainViewer({ levels }: ResolutionChainViewerProps) {
  return (
    <div className="flex flex-col gap-2">
      {levels.map((level) => (
        <div
          key={level.scope}
          className="flex items-center gap-2"
        >
          <ScopeIndicator scope={level.scope} />
          <span className="font-mono text-[13px]">
            {level.value != null ? (
              <span className="text-foreground">{String(level.value)}</span>
            ) : (
              <span className="text-muted-foreground">
                {level.inherited
                  ? `\u2014 (inherits ${level.scope === "workspace" ? "tenant" : "global"})`
                  : "\u2014"}
              </span>
            )}
          </span>
        </div>
      ))}
    </div>
  );
}
