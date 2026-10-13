// Shared types and fetch helpers for the status API.
// All requests are same-origin; nginx proxies /api/* to the Go backend.

export interface CheckResult {
  target: string;
  checked_at: string;
  status_code: number;
  ok: boolean;
  latency_ms: number;
  error?: string;
  cert_issuer?: string;
  cert_not_after?: string;
  cert_days_left?: number;
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (!res.ok) {
    throw new Error(`request failed: ${res.status}`);
  }
  return (await res.json()) as T;
}

export function fetchStatus(): Promise<CheckResult[]> {
  return getJSON<CheckResult[]>("/api/status");
}

export function fetchHistory(target: string, limit = 100): Promise<CheckResult[]> {
  const params = new URLSearchParams({ target, limit: String(limit) });
  return getJSON<CheckResult[]>(`/api/history?${params.toString()}`);
}
