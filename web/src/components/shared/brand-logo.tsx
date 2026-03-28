import Image from "next/image";
import { cn } from "@/lib/utils";

const sizeMap = {
  xs: 20,
  sm: 28,
  md: 40,
  lg: 64,
} as const;

type BrandLogoSize = keyof typeof sizeMap;

interface BrandLogoProps {
  size?: BrandLogoSize;
  showWordmark?: boolean;
  className?: string;
  imageClassName?: string;
  wordmarkClassName?: string;
  priority?: boolean;
}

export function BrandLogo({
  size = "md",
  showWordmark = false,
  className,
  imageClassName,
  wordmarkClassName,
  priority = false,
}: BrandLogoProps) {
  const dimension = sizeMap[size];

  return (
    <div className={cn("flex items-center gap-2.5", className)}>
      <Image
        src="/senda-logo.svg"
        alt="Senda logo"
        width={dimension}
        height={dimension}
        priority={priority}
        className={cn("shrink-0", imageClassName)}
      />
      {showWordmark && (
        <span
          className={cn(
            "text-base font-semibold tracking-tight text-foreground",
            wordmarkClassName,
          )}
        >
          Senda
        </span>
      )}
    </div>
  );
}
