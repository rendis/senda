"use client";

import { Check, AlertCircle, Loader2, Cloud } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import type { AutoSaveStatus } from "@/hooks/use-auto-save";

export interface SaveStatusIndicatorProps {
  status: AutoSaveStatus;
  lastSavedAt: Date | null;
  error: Error | null;
  onRetry?: () => void;
  className?: string;
}

const easeOutCubic = [0.4, 0, 0.2, 1] as const;
const easeInCubic = [0.4, 0, 1, 1] as const;

const textVariants = {
  initial: { opacity: 0, x: 30, filter: "blur(4px)" },
  animate: {
    opacity: 1,
    x: 0,
    filter: "blur(0px)",
    transition: { duration: 0.5, ease: easeOutCubic },
  },
  exit: {
    opacity: 0,
    x: 30,
    filter: "blur(4px)",
    transition: { duration: 0.4, ease: easeInCubic },
  },
};

const iconMorphTransition = {
  type: "spring" as const,
  stiffness: 150,
  damping: 20,
  duration: 0.6,
};

const easeBackOut = [0.34, 1.56, 0.64, 1] as const;
const easeIn = [0.4, 0, 1, 1] as const;

const iconVariants = {
  initial: { scale: 0.5, opacity: 0 },
  animate: {
    scale: 1,
    opacity: 1,
    transition: { duration: 0.4, ease: easeBackOut },
  },
  exit: {
    scale: 0.5,
    opacity: 0,
    transition: { duration: 0.25, ease: easeIn },
  },
};

function StatusIcon({ status }: { status: AutoSaveStatus }) {
  const iconMap = {
    idle: <Cloud className="h-4 w-4 text-muted-foreground" />,
    pending: <Cloud className="h-4 w-4 text-muted-foreground" />,
    saving: <Loader2 className="h-4 w-4 text-primary animate-spin" />,
    saved: <Check className="h-4 w-4 text-green-600 dark:text-green-500" />,
    error: <AlertCircle className="h-4 w-4 text-destructive" />,
  };

  return (
    <motion.div
      layoutId="save-status-icon"
      className="flex items-center justify-center w-5 h-5"
      transition={iconMorphTransition}
    >
      <AnimatePresence mode="wait">
        <motion.div
          key={status}
          variants={iconVariants}
          initial="initial"
          animate="animate"
          exit="exit"
        >
          {iconMap[status]}
        </motion.div>
      </AnimatePresence>
    </motion.div>
  );
}

export function SaveStatusIndicator({
  status,
  lastSavedAt,
  onRetry,
  className,
}: SaveStatusIndicatorProps) {
  const t = useTranslations("editor.saveStatus");
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!lastSavedAt || status !== "idle") {
      return;
    }

    const interval = setInterval(() => {
      setNow(Date.now());
    }, 1000);

    return () => clearInterval(interval);
  }, [lastSavedAt, status]);

  const formatLastSaved = (date: Date): string => {
    const diffMs = now - date.getTime();
    const diffSeconds = Math.floor(diffMs / 1000);
    const diffMinutes = Math.floor(diffSeconds / 60);

    if (diffSeconds < 5) return t("justNow");
    if (diffSeconds < 60) return t("secondsAgo", { n: diffSeconds });
    if (diffMinutes < 60) return t("minutesAgo", { n: diffMinutes });
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  };

  const getTextContent = () => {
    switch (status) {
      case "idle":
        return lastSavedAt ? t("savedAgo", { time: formatLastSaved(lastSavedAt) }) : null;
      case "pending":
        return t("unsaved");
      case "saving":
        return t("saving");
      case "saved":
        return t("saved");
      case "error":
        return t("error");
      default:
        return null;
    }
  };

  const getTextClass = () => {
    switch (status) {
      case "idle":
      case "pending":
        return "text-muted-foreground";
      case "saving":
        return "text-primary";
      case "saved":
        return "text-green-600 dark:text-green-500";
      case "error":
        return "text-destructive";
      default:
        return "text-muted-foreground";
    }
  };

  const textContent = getTextContent();

  return (
    <motion.div
      layout
      className={cn(
        "flex items-center gap-2 text-xs h-5 min-w-[120px] justify-end overflow-hidden",
        status === "idle" && "opacity-60",
        className
      )}
    >
      <AnimatePresence mode="wait">
        {textContent && (
          <motion.span
            key={`${status}-${textContent}`}
            className={cn("whitespace-nowrap", getTextClass())}
            variants={textVariants}
            initial="initial"
            animate="animate"
            exit="exit"
          >
            {textContent}
          </motion.span>
        )}
      </AnimatePresence>

      <StatusIcon status={status} />

      <AnimatePresence>
        {status === "error" && onRetry && (
          <motion.div
            initial={{ opacity: 0, scale: 0.8, x: 20 }}
            animate={{ opacity: 1, scale: 1, x: 0 }}
            exit={{ opacity: 0, scale: 0.8, x: 20 }}
            transition={{ duration: 0.3, ease: "easeOut" }}
          >
            <Button
              variant="ghost"
              size="sm"
              className="h-5 px-1.5 text-xs text-destructive hover:text-destructive"
              onClick={onRetry}
            >
              {t("retry")}
            </Button>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
