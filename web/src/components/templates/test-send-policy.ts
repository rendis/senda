export interface TestSendAvailability {
  enabled: boolean;
  reason?: string;
}

export function getTestSendAvailability(params: {
  adapterId?: string | null;
}): TestSendAvailability {
  if (!params.adapterId) {
    return {
      enabled: false,
      reason: "Assign an adapter to this template type before sending test emails.",
    };
  }

  return { enabled: true };
}
