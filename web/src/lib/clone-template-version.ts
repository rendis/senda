import type { KyInstance } from "ky";
import type { TemplateVersion } from "@/types/templates";

export function cloneTemplateVersion(
  api: KyInstance,
  scopedPath: string,
  templateId: string,
  versionId: string
) {
  return api
    .post(`${scopedPath}/templates/${templateId}/versions/${versionId}/clone`)
    .json<TemplateVersion>();
}
