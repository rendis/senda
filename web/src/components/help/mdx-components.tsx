import type { MDXComponents } from "mdx/types";
import { ApiEndpointReference } from "@/components/help/api-endpoint-reference";
import { HelpLink } from "@/components/help/help-link";

export function getHelpMDXComponents(): MDXComponents {
  return {
    a: HelpLink,
    ApiEndpointReference,
  };
}
