import { z } from "zod";

/**
 * Mirrors backend pkg/slug/slug.go exactly.
 * Pattern: ^[a-z][a-z0-9_-]{1,62}[a-z0-9]$  (3-64 chars)
 */
const SLUG_PATTERN = /^[a-z][a-z0-9_-]{1,62}[a-z0-9]$/;

const RESERVED_WORDS = new Set([
  "system",
  "admin",
  "api",
  "internal",
  "global",
  "null",
  "undefined",
]);

export const slugSchema = z
  .string()
  .min(3, "Mínimo 3 caracteres")
  .max(64, "Máximo 64 caracteres")
  .regex(
    SLUG_PATTERN,
    "Debe iniciar con letra, terminar con letra/dígito, solo minúsculas, dígitos, guiones y guiones bajos",
  )
  .refine((val) => !RESERVED_WORDS.has(val), {
    message: "Esta palabra está reservada y no puede usarse",
  });

export const nameSchema = z
  .string()
  .min(1, "El nombre es requerido")
  .max(255, "Máximo 255 caracteres");

/**
 * Generate a slug from a display name.
 * Lowercases, replaces spaces with hyphens, strips invalid chars,
 * collapses multiple hyphens, trims leading/trailing hyphens, truncates to 64.
 */
export function generateSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/\s+/g, "-")
    .replace(/[^a-z0-9_-]/g, "")
    .replace(/-{2,}/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64);
}
