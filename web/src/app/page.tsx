import { redirect } from "next/navigation";
import { authWithoutRefresh } from "@/auth";
import { decideRootRedirectPath } from "@/app/root-page-redirect";

export default async function RootPage() {
  const session = await authWithoutRefresh();
  redirect(decideRootRedirectPath(session));
}
