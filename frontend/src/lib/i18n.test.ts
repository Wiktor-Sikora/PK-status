import { afterEach, expect, test } from "bun:test";
import {
  DEFAULT_LANG,
  LANGUAGES,
  LANGUAGE_NAMES,
  detectLanguage,
  formatRelativeTime,
  translate,
} from "./i18n";

const originalGlobals = {
  window: Object.getOwnPropertyDescriptor(globalThis, "window"),
  localStorage: Object.getOwnPropertyDescriptor(globalThis, "localStorage"),
  navigator: Object.getOwnPropertyDescriptor(globalThis, "navigator"),
};

afterEach(() => {
  for (const [name, descriptor] of Object.entries(originalGlobals)) {
    if (descriptor) {
      Object.defineProperty(globalThis, name, descriptor);
    } else {
      delete (globalThis as Record<string, unknown>)[name];
    }
  }
});

function setBrowserGlobals(opts: {
  saved?: string | null;
  languages?: string[];
  storageThrows?: boolean;
}) {
  const storage = {
    getItem() {
      if (opts.storageThrows) throw new Error("storage unavailable");
      return opts.saved ?? null;
    },
    setItem() {},
  };
  Object.defineProperty(globalThis, "window", { value: {}, configurable: true, writable: true });
  Object.defineProperty(globalThis, "localStorage", {
    value: storage,
    configurable: true,
    writable: true,
  });
  Object.defineProperty(globalThis, "navigator", {
    value: { languages: opts.languages ?? [], language: opts.languages?.[0] ?? "en-US" },
    configurable: true,
    writable: true,
  });
}

test("exposes the supported languages", () => {
  expect([...LANGUAGES]).toEqual(["en", "pl"]);
  expect(DEFAULT_LANG).toBe("en");
  expect(LANGUAGE_NAMES.en).toBe("English");
  expect(LANGUAGE_NAMES.pl).toBe("Polski");
});

test("translates keys for each language", () => {
  const en = translate("en");
  const pl = translate("pl");

  expect(en("dashboard.loading")).toBe("Loading status…");
  expect(pl("dashboard.loading")).toBe("Wczytywanie statusu…");
  expect(en("card.latency")).toBe("Latency");
  expect(pl("card.latency")).toBe("Opóźnienie");
});

test("interpolates parameters", () => {
  expect(translate("en")("dashboard.error", { error: "boom" })).toBe(
    "Could not reach the status API: boom",
  );
  expect(translate("pl")("dashboard.error", { error: "boom" })).toBe(
    "Nie udało się połączyć z API statusu: boom",
  );
  expect(translate("en")("card.issuer", { issuer: "Test CA" })).toBe("Issuer: Test CA");
});

test("falls back to the key for unknown messages", () => {
  const t = translate("en") as (key: string) => string;
  expect(t("does.not.exist")).toBe("does.not.exist");
});

test("detectLanguage returns the default without a browser", () => {
  expect(typeof (globalThis as Record<string, unknown>).window).toBe("undefined");
  expect(detectLanguage()).toBe(DEFAULT_LANG);
});

test("detectLanguage prefers a saved valid choice", () => {
  setBrowserGlobals({ saved: "pl", languages: ["en-US"] });
  expect(detectLanguage()).toBe("pl");
});

test("detectLanguage ignores an invalid saved choice", () => {
  setBrowserGlobals({ saved: "de", languages: ["pl-PL"] });
  expect(detectLanguage()).toBe("pl");
});

test("detectLanguage uses the browser language list", () => {
  setBrowserGlobals({ languages: ["de-DE", "pl-PL"] });
  expect(detectLanguage()).toBe("pl");
});

test("detectLanguage falls back when no language matches", () => {
  setBrowserGlobals({ languages: ["de-DE", "fr-FR"] });
  expect(detectLanguage()).toBe(DEFAULT_LANG);
});

test("detectLanguage survives unavailable storage", () => {
  setBrowserGlobals({ storageThrows: true, languages: ["pl"] });
  expect(detectLanguage()).toBe("pl");
});

test("formatRelativeTime picks the right unit", () => {
  const realNow = Date.now;
  const now = new Date("2026-01-02T12:00:00Z").getTime();
  Date.now = () => now;
  try {
    const iso = (msAgo: number) => new Date(now - msAgo).toISOString();
    const en = new Intl.RelativeTimeFormat("en", { numeric: "always", style: "narrow" });
    const pl = new Intl.RelativeTimeFormat("pl", { numeric: "always", style: "narrow" });

    expect(formatRelativeTime(iso(30_000), "en")).toBe(en.format(-30, "second"));
    expect(formatRelativeTime(iso(5 * 60_000), "en")).toBe(en.format(-5, "minute"));
    expect(formatRelativeTime(iso(3 * 3_600_000), "en")).toBe(en.format(-3, "hour"));
    expect(formatRelativeTime(iso(4 * 86_400_000), "en")).toBe(en.format(-4, "day"));
    expect(formatRelativeTime(iso(30_000), "pl")).toBe(pl.format(-30, "second"));
  } finally {
    Date.now = realNow;
  }
});

test("formatRelativeTime clamps future timestamps", () => {
  const realNow = Date.now;
  const now = new Date("2026-01-02T12:00:00Z").getTime();
  Date.now = () => now;
  try {
    const future = new Date(now + 60_000).toISOString();
    const en = new Intl.RelativeTimeFormat("en", { numeric: "always", style: "narrow" });
    expect(formatRelativeTime(future, "en")).toBe(en.format(-0, "second"));
  } finally {
    Date.now = realNow;
  }
});
