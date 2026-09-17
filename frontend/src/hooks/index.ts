/** Shared React hooks for the auditor UI. Domain logic stays on the API. */

export function useApiBaseURL(): string {
  return import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";
}
