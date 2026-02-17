"use client";

import { signIn, useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { LogIn, Send } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function LoginPage() {
  const { status } = useSession();
  const router = useRouter();

  useEffect(() => {
    if (status === "authenticated") {
      router.replace("/global");
    }
  }, [status, router]);

  if (status === "loading" || status === "authenticated") {
    return null;
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-page">
      <div className="w-[400px] rounded-lg border border-border bg-card px-10 py-12">
        <div className="flex flex-col items-center gap-8">
          {/* Logo */}
          <div className="flex flex-col items-center gap-2">
            <Send className="h-12 w-12 text-primary" />
            <h1 className="text-[28px] font-bold text-foreground">Senda</h1>
            <p className="text-sm text-muted-foreground">
              Email Orchestration Platform
            </p>
          </div>

          {/* Divider */}
          <div className="h-px w-full bg-border" />

          {/* OIDC Button */}
          <Button
            onClick={() => signIn("oidc", { callbackUrl: "/global" })}
            className="h-11 w-full gap-2.5 rounded-md text-sm font-medium"
          >
            <LogIn className="h-[18px] w-[18px]" />
            Iniciar sesión con OIDC
          </Button>

          {/* Footer */}
          <p className="w-[280px] text-center text-xs text-muted-foreground">
            Al continuar, aceptas los Términos de Servicio
          </p>
        </div>
      </div>
    </div>
  );
}
