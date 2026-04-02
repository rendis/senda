"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { AnchorHTMLAttributes } from "react";
import { getHelpBasePath } from "@/lib/help";

interface HelpLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  href?: string;
}

export function HelpLink({ href = "", children, ...props }: HelpLinkProps) {
  const pathname = usePathname();

  const resolvedHref =
    href.startsWith("http") ||
    href.startsWith("/") ||
    href.startsWith("#") ||
    href.startsWith("mailto:")
      ? href
      : `${getHelpBasePath(pathname)}/${href}`.replace(/\/+$/, "");

  return (
    <Link href={resolvedHref} {...props}>
      {children}
    </Link>
  );
}
