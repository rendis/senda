import ky from "ky";

const api = ky.create({ prefix: "/api/v1" });

export type UnsubscribeContext = {
  workspace_name: string;
  template_type_slug: string;
  template_type_name: string;
  email: string;
  opted_out_of_type: boolean;
  opted_out_of_all: boolean;
};

export type PreferencesEntry = {
  template_type_slug: string;
  template_type_name: string;
  description?: string;
  subscribed: boolean;
  last_received_at: string;
};

export type PreferencesView = {
  workspace_name: string;
  email: string;
  opted_out_of_all: boolean;
  entries: PreferencesEntry[];
};

export async function getContext(token: string): Promise<UnsubscribeContext> {
  return api.get(`u/${encodeURIComponent(token)}`).json<UnsubscribeContext>();
}

export async function optOutThisType(token: string): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}`);
}

export async function optOutAll(token: string): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}/all`);
}

export async function resubscribe(token: string): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}/resubscribe`);
}

export async function getPreferences(token: string): Promise<PreferencesView> {
  return api.get(`u/${encodeURIComponent(token)}/preferences`).json<PreferencesView>();
}

export async function updatePreferences(
  token: string,
  changes: { template_type_slug: string; subscribed: boolean }[],
): Promise<void> {
  await api.post(`u/${encodeURIComponent(token)}/preferences`, { json: { changes } });
}
