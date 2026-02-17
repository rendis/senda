"use client";

import { signOut, useSession } from "next-auth/react";
import { ShieldX, LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function AccessDeniedPage() {
  const { data: session } = useSession();

  return (
    <div className="flex min-h-screen items-center justify-center bg-page">
      <div className="w-[440px] rounded-lg border border-border bg-card px-10 py-12">
        <div className="flex flex-col items-center gap-6">
          {/* Icon */}
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
            <ShieldX className="h-8 w-8 text-destructive" />
          </div>

          {/* Title */}
          <h1 className="text-2xl font-bold text-foreground">
            Acceso Denegado
          </h1>

          {/* Message */}
          <p className="w-[360px] text-center text-sm leading-relaxed text-muted-foreground">
            Tu email ({session?.user?.email ?? "unknown"}) no está registrado
            como miembro de ningún workspace. Contacta a tu administrador para
            solicitar acceso.
          </p>

          {/* Divider */}
          <div className="h-px w-full bg-border" />

          {/* Sign Out Button */}
          <Button
            variant="destructive"
            onClick={() => signOut({ callbackUrl: "/login" })}
            className="h-9 w-full gap-2 rounded-md text-[13px] font-medium"
          >
            <LogOut className="h-4 w-4" />
            Cerrar Sesión
          </Button>
        </div>
      </div>
    </div>
  );
}
