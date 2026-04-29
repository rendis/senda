"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export type AutoSaveStatus = "idle" | "pending" | "saving" | "saved" | "error";

export interface UseAutoSaveOptions<T> {
  getPayload: () => T;
  saveFn: (data: T) => Promise<unknown>;
  enabled: boolean;
  debounceMs?: number;
}

export interface UseAutoSaveReturn {
  status: AutoSaveStatus;
  lastSavedAt: Date | null;
  error: Error | null;
  isDirty: boolean;
  scheduleSave: () => void;
  save: () => Promise<void>;
  resetError: () => void;
}

const DEFAULT_DEBOUNCE_MS = 2000;
const MAX_RETRIES = 2;
const SAVED_DISPLAY_MS = 3000;

export function useAutoSave<T>({
  getPayload,
  saveFn,
  enabled,
  debounceMs = DEFAULT_DEBOUNCE_MS,
}: UseAutoSaveOptions<T>): UseAutoSaveReturn {
  const [status, setStatus] = useState<AutoSaveStatus>("idle");
  const [lastSavedAt, setLastSavedAt] = useState<Date | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isDirty, setIsDirty] = useState(false);

  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const savedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isSavingRef = useRef(false);

  const getPayloadRef = useRef(getPayload);
  const saveFnRef = useRef(saveFn);

  useEffect(() => {
    getPayloadRef.current = getPayload;
  }, [getPayload]);

  useEffect(() => {
    saveFnRef.current = saveFn;
  }, [saveFn]);

  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current);
      if (savedTimerRef.current) clearTimeout(savedTimerRef.current);
    };
  }, []);

  const performSave = useCallback(async () => {
    if (!enabled || isSavingRef.current) return;

    isSavingRef.current = true;
    setStatus("saving");
    setError(null);

    try {
      for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
        try {
          const payload = getPayloadRef.current();
          await saveFnRef.current(payload);

          setStatus("saved");
          setLastSavedAt(new Date());
          setIsDirty(false);

          if (savedTimerRef.current) clearTimeout(savedTimerRef.current);
          savedTimerRef.current = setTimeout(() => {
            setStatus("idle");
          }, SAVED_DISPLAY_MS);
          return;
        } catch (err) {
          if (attempt < MAX_RETRIES) {
            await new Promise((resolve) => setTimeout(resolve, 1000));
            continue;
          }

          setStatus("error");
          setError(err instanceof Error ? err : new Error("Save failed"));
        }
      }
    } finally {
      isSavingRef.current = false;
    }
  }, [enabled]);

  const save = useCallback(async () => {
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
      debounceTimerRef.current = null;
    }
    await performSave();
  }, [performSave]);

  const resetError = useCallback(() => {
    setError(null);
    setStatus(isDirty ? "pending" : "idle");
  }, [isDirty]);

  const scheduleSave = useCallback(() => {
    if (!enabled) return;

    setIsDirty(true);
    setStatus("pending");

    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }

    debounceTimerRef.current = setTimeout(() => {
      debounceTimerRef.current = null;
      performSave();
    }, debounceMs);
  }, [enabled, debounceMs, performSave]);

  return {
    status,
    lastSavedAt,
    error,
    isDirty,
    scheduleSave,
    save,
    resetError,
  };
}
