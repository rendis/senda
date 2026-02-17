"use client";

export default function DashboardTemplate({
  children,
}: {
  children: React.ReactNode;
}) {
  return <div className="animate-in fade-in slide-in-from-bottom-2 duration-200">{children}</div>;
}
