import type { EnvironmentsResponse, HealthResponse } from "../types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  if (!response.ok) {
    throw new Error(`API ${path} failed with ${response.status}`);
  }
  return response.json() as Promise<T>;
}

/** Typed API client — frontend never talks to databases directly. */
export const api = {
  health: () => getJSON<HealthResponse>("/health"),
  ready: () => getJSON<HealthResponse>("/ready"),
  environments: () => getJSON<EnvironmentsResponse>("/api/v1/environments"),
};
