import { HTTPError } from "ky";

export function getHttpStatus(error: unknown): number | null {
  return error instanceof HTTPError ? error.response.status : null;
}
