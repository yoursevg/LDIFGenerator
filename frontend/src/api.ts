import type { GeneratorConfig, Progress, SchemaSummary } from "./types";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(data?.error || response.statusText);
  }
  return data as T;
}

export function defaultConfig() {
  return request<GeneratorConfig>("/api/default-config");
}

export function loadSchema(paths: string[]) {
  return request<SchemaSummary>("/api/schema/load", {
    method: "POST",
    body: JSON.stringify({ paths }),
  });
}

export function loadConfig(path: string) {
  return request<GeneratorConfig>("/api/config/load", {
    method: "POST",
    body: JSON.stringify({ path }),
  });
}

export function saveConfig(path: string, config: GeneratorConfig) {
  return request<{ ok: boolean }>("/api/config/save", {
    method: "POST",
    body: JSON.stringify({ path, config }),
  });
}

export function generate(config: GeneratorConfig) {
  return request<Progress>("/api/generate", {
    method: "POST",
    body: JSON.stringify(config),
  });
}

export function cancelGeneration() {
  return request<{ ok: boolean }>("/api/cancel", { method: "POST" });
}

export function progress() {
  return request<Progress>("/api/progress");
}
