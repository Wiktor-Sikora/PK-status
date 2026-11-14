import { afterEach, expect, test } from "bun:test";
import { fetchHistory, fetchStatus } from "./api";

type Call = { url: string; init?: RequestInit };

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function stubFetch(response: unknown, status = 200): Call[] {
  const calls: Call[] = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return {
      ok: status >= 200 && status < 300,
      status,
      json: async () => response,
    } as Response;
  }) as typeof fetch;
  return calls;
}

test("fetchStatus requests /api/status and returns JSON", async () => {
  const payload = [{ target: "https://pk.edu.pl", ok: true, latency_ms: 12 }];
  const calls = stubFetch(payload);

  const result = await fetchStatus();

  expect(result).toEqual(payload);
  expect(calls.length).toBe(1);
  expect(calls[0].url).toBe("/api/status");
  expect(calls[0].init?.headers).toEqual({ Accept: "application/json" });
});

test("fetchHistory encodes target and uses the default limit", async () => {
  const target = "https://a.example/?x=1&y=2";
  const calls = stubFetch([]);

  const result = await fetchHistory(target);

  expect(result).toEqual([]);
  const query = new URLSearchParams({ target, limit: "100" }).toString();
  expect(calls[0].url).toBe(`/api/history?${query}`);
});

test("fetchHistory forwards a custom limit", async () => {
  const calls = stubFetch([]);

  await fetchHistory("https://a.example", 25);

  const query = new URLSearchParams({ target: "https://a.example", limit: "25" }).toString();
  expect(calls[0].url).toBe(`/api/history?${query}`);
});

test("fetchStatus rejects on a non-OK response", async () => {
  stubFetch({ error: "down" }, 503);

  await expect(fetchStatus()).rejects.toThrow("request failed: 503");
});

test("fetchHistory rejects on a non-OK response", async () => {
  stubFetch({ error: "down" }, 500);

  await expect(fetchHistory("https://a.example")).rejects.toThrow("request failed: 500");
});
