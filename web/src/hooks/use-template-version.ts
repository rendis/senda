"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi, useApiReady } from "@/hooks/use-api";
import { cloneTemplateVersion } from "@/lib/clone-template-version";
import { parseTemplateVersionMutationResponse } from "@/lib/template-version-response";
import type {
  TemplateVersion,
  TemplateLocale,
  CreateTemplateVersionRequest,
  MjmlPreviewResponse,
  TestSendRequest,
  TemplateBulkSendRequest,
  TemplateBulkSendResponse,
  TemplateBulkSendConfig,
} from "@/types/templates";

const QK_TEMPLATE_VERSIONS = "template-versions";

export function useTemplateVersions(scopedPath: string, templateId: string) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: [QK_TEMPLATE_VERSIONS, scopedPath, templateId],
    queryFn: () =>
      api
        .get(`${scopedPath}/templates/${templateId}/versions`)
        .json<{ items: TemplateVersion[] }>()
        .then((r) => r.items),
    enabled: ready && !!templateId,
  });
}

export function useTemplateVersion(
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["template-version", scopedPath, templateId, versionId],
    queryFn: () =>
      api
        .get(`${scopedPath}/templates/${templateId}/versions/${versionId}`)
        .json<TemplateVersion>(),
    enabled: ready && !!templateId && !!versionId,
  });
}

export function useCreateTemplateVersion(
  scopedPath: string,
  templateId: string
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTemplateVersionRequest) =>
      api
        .post(`${scopedPath}/templates/${templateId}/versions`, { json: data })
        .json<TemplateVersion>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [QK_TEMPLATE_VERSIONS, scopedPath, templateId],
      });
    },
  });
}

export function useCloneVersion(scopedPath: string, templateId: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (versionId: string) =>
      cloneTemplateVersion(api, scopedPath, templateId, versionId),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [QK_TEMPLATE_VERSIONS, scopedPath, templateId],
      });
    },
  });
}

export function useSaveTemplateVersion(
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTemplateVersionRequest) =>
      api
        .put(`${scopedPath}/templates/${templateId}/versions/${versionId}`, {
          json: data,
        })
        .json<TemplateVersion>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["template-version", scopedPath, templateId, versionId],
      });
      queryClient.invalidateQueries({
        queryKey: [QK_TEMPLATE_VERSIONS, scopedPath, templateId],
      });
    },
  });
}

export function usePublishVersion(
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () =>
      api
        .post(
          `${scopedPath}/templates/${templateId}/versions/${versionId}/publish`
        )
        .then(parseTemplateVersionMutationResponse<TemplateVersion>),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [QK_TEMPLATE_VERSIONS, scopedPath, templateId],
      });
      queryClient.invalidateQueries({
        queryKey: ["template-version", scopedPath, templateId, versionId],
      });
    },
  });
}

export function usePreviewMjml(scopedPath: string, templateId: string) {
  const api = useApi();

  return useMutation({
    mutationFn: ({
      bodyMjml,
      signal,
    }: {
      bodyMjml: string;
      signal?: AbortSignal;
    }) =>
      api
        .post(`${scopedPath}/templates/${templateId}/preview-mjml`, {
          json: { mjml: bodyMjml },
          signal,
        })
        .json<MjmlPreviewResponse>(),
  });
}

export function useTestSend(scopedPath: string, templateId: string) {
  const api = useApi();

  return useMutation({
    mutationFn: (data: TestSendRequest) =>
      api
        .post(`${scopedPath}/templates/${templateId}/test-send`, {
          json: data,
        })
        .json<void>(),
  });
}

export function useTemplateBulkSendConfig(
  scopedPath: string,
  templateId: string,
  enabled = true
) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["template-bulk-send-config", scopedPath, templateId],
    queryFn: () =>
      api
        .get(`${scopedPath}/templates/${templateId}/bulk-send-config`)
        .json<TemplateBulkSendConfig>(),
    enabled: ready && enabled && !!templateId,
  });
}

export function useTemplateBulkSend(scopedPath: string, templateId: string) {
  const api = useApi();

  return useMutation({
    mutationFn: (data: TemplateBulkSendRequest) =>
      api
        .post(`${scopedPath}/templates/${templateId}/bulk-send`, {
          json: data,
        })
        .json<TemplateBulkSendResponse>(),
  });
}

export function useDeleteVersion(scopedPath: string, templateId: string) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (versionId: string) =>
      api.delete(`${scopedPath}/templates/${templateId}/versions/${versionId}`).then(() => {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [QK_TEMPLATE_VERSIONS, scopedPath, templateId] });
    },
  });
}

export function useTemplateLocale(
  scopedPath: string,
  templateId: string,
  versionId: string,
  locale: string
) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: [
      "template-locale",
      scopedPath,
      templateId,
      versionId,
      locale,
    ],
    queryFn: () =>
      api
        .get(
          `${scopedPath}/templates/${templateId}/versions/${versionId}/locales/${locale}`
        )
        .json<TemplateLocale>(),
    enabled: ready && !!templateId && !!versionId && !!locale,
  });
}

export function useSaveTemplateLocale(
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: {
      locale: string;
      subject?: string;
      preview_text?: string;
      from_name?: string;
      body_mjml?: string;
      editor_data?: Record<string, unknown>;
    }) =>
      api
        .put(
          `${scopedPath}/templates/${templateId}/versions/${versionId}/locales/${data.locale}`,
          { json: data }
        )
        .json<TemplateLocale>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["template-locale", scopedPath, templateId, versionId],
      });
      queryClient.invalidateQueries({
        queryKey: ["template-locales", scopedPath, templateId, versionId],
      });
    },
  });
}

export function useTemplateVersionLocales(
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  const api = useApi();
  const ready = useApiReady();

  return useQuery({
    queryKey: ["template-locales", scopedPath, templateId, versionId],
    queryFn: () =>
      api
        .get(
          `${scopedPath}/templates/${templateId}/versions/${versionId}/locales`
        )
        .json<{ items: TemplateLocale[] }>()
        .then((r) => r.items),
    enabled: ready && !!templateId && !!versionId,
  });
}

export function useDeleteTemplateLocale(
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  const api = useApi();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (locale: string) => {
      await api.delete(
        `${scopedPath}/templates/${templateId}/versions/${versionId}/locales/${locale}`
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["template-locales", scopedPath, templateId, versionId],
      });
      queryClient.invalidateQueries({
        queryKey: ["template-locale", scopedPath, templateId, versionId],
      });
    },
  });
}
