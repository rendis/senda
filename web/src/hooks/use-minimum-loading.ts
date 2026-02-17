"use client";

import { useState, useEffect, useRef } from "react";

/**
 * Ensures a loading state stays `true` for at least `minMs` milliseconds.
 * Prevents skeleton "flash" when APIs respond too quickly.
 */
export function useMinimumLoading(isLoading: boolean, minMs = 400): boolean {
  const [showLoading, setShowLoading] = useState(isLoading);
  const startRef = useRef<number | null>(null);

  useEffect(() => {
    if (isLoading) {
      startRef.current = Date.now();
      setShowLoading(true);
    } else if (startRef.current !== null) {
      const elapsed = Date.now() - startRef.current;
      const remaining = minMs - elapsed;

      if (remaining <= 0) {
        setShowLoading(false);
        startRef.current = null;
      } else {
        const timer = setTimeout(() => {
          setShowLoading(false);
          startRef.current = null;
        }, remaining);
        return () => clearTimeout(timer);
      }
    }
  }, [isLoading, minMs]);

  return showLoading;
}
