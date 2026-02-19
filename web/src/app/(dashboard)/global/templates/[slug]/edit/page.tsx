import { Suspense } from "react";
import { MjmlEditor } from "@/components/templates/mjml-editor";
import { Skeleton } from "@/components/ui/skeleton";

export default function GlobalTemplateEditPage() {
  return (
    <div className="flex flex-col h-[calc(100vh-theme(spacing.16))]">
      <Suspense
        fallback={
          <div className="flex flex-col h-full">
            <div className="flex items-center justify-between h-14 px-6 border-b bg-card">
              <Skeleton className="h-5 w-64" />
              <div className="flex gap-2">
                <Skeleton className="h-8 w-24" />
                <Skeleton className="h-8 w-24" />
              </div>
            </div>
            <div className="flex flex-1">
              <div className="flex-1 bg-background" />
              <div className="w-[480px] bg-slate-100 border-l" />
            </div>
          </div>
        }
      >
        <MjmlEditor />
      </Suspense>
    </div>
  );
}
