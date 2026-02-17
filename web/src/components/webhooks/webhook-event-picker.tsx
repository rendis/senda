"use client";

import type { WebhookEventType } from "@/types/webhooks";

const ALL_EVENTS: { value: WebhookEventType; label: string }[] = [
  { value: "email.sent", label: "Sent" },
  { value: "email.delivered", label: "Delivered" },
  { value: "email.bounced", label: "Bounced" },
  { value: "email.complained", label: "Complained" },
  { value: "email.opened", label: "Opened" },
];

interface WebhookEventPickerProps {
  value: WebhookEventType[];
  onChange: (events: WebhookEventType[]) => void;
}

export function WebhookEventPicker({
  value,
  onChange,
}: WebhookEventPickerProps) {
  const toggle = (event: WebhookEventType) => {
    if (value.includes(event)) {
      onChange(value.filter((e) => e !== event));
    } else {
      onChange([...value, event]);
    }
  };

  return (
    <div className="space-y-2">
      <label className="text-sm font-medium">Events</label>
      <div className="space-y-1.5">
        {ALL_EVENTS.map((evt) => (
          <label
            key={evt.value}
            className="flex items-center gap-2 cursor-pointer"
          >
            <input
              type="checkbox"
              checked={value.includes(evt.value)}
              onChange={() => toggle(evt.value)}
              className="h-4 w-4 rounded border-border text-primary accent-primary"
            />
            <span className="text-sm font-mono">{evt.value}</span>
          </label>
        ))}
      </div>
    </div>
  );
}
