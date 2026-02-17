interface SettingsSectionProps {
  title: string;
  children: React.ReactNode;
}

export function SettingsSection({ title, children }: SettingsSectionProps) {
  return (
    <div className="rounded-lg border bg-card p-6 space-y-4">
      <h2 className="text-base font-semibold" style={{ letterSpacing: "-0.5px" }}>
        {title}
      </h2>
      {children}
    </div>
  );
}
