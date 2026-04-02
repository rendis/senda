import type { MDXComponents } from "mdx/types";
import { HelpLink } from "@/components/help/help-link";

export function getHelpMDXComponents(): MDXComponents {
  return {
    a: HelpLink,
  };
}
