"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useApi } from "@/hooks/use-api";
import type {
  TemplateVersion,
  TemplateLocale,
  CreateTemplateVersionRequest,
  MjmlPreviewResponse,
  TestSendRequest,
} from "@/types/templates";

export function useTemplateVersions(scopedPath: string, templateId: string) {
  const api = useApi();

  return useQuery({
    queryKey: ["template-versions", scopedPath, templateId],
    queryFn: () =>
      api
        .get(`${scopedPath}/templates/${templateId}/versions`)
        .json<TemplateVersion[]>(),
    enabled: !!templateId,
  });
}

export function useTemplateVersion(
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  const api = useApi();

  return useQuery({
    queryKey: ["template-version", scopedPath, templateId, versionId],
    queryFn: () =>
      api
        .get(`${scopedPath}/templates/${templateId}/versions/${versionId}`)
        .json<TemplateVersion>(),
    enabled: !!templateId && !!versionId,
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
        queryKey: ["template-versions", scopedPath, templateId],
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
        queryKey: ["template-versions", scopedPath, templateId],
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
        .json<TemplateVersion>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["template-versions", scopedPath, templateId],
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
    mutationFn: (bodyMjml: string) =>
      api
        .post(`${scopedPath}/templates/${templateId}/preview-mjml`, {
          json: { body_mjml: bodyMjml },
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

export function useTemplateLocale(
  scopedPath: string,
  templateId: string,
  versionId: string,
  locale: string
) {
  const api = useApi();

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
    enabled: !!templateId && !!versionId && !!locale,
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
    }) =>
      api
        .put(
          `${scopedPath}/templates/${templateId}/versions/${versionId}/locales`,
          { json: data }
        )
        .json<TemplateLocale>(),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["template-locale", scopedPath, templateId, versionId],
      });
    },
  });
}
