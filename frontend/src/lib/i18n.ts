// Minimal i18n: dictionaries + a tiny shared store so every React island
// (and plain DOM bits like <html lang>) stay in sync with the chosen language.
import { useSyncExternalStore } from "react";

export const LANGUAGES = ["en", "pl"] as const;
export type Lang = (typeof LANGUAGES)[number];

export const DEFAULT_LANG: Lang = "en";
const STORAGE_KEY = "pk-status-lang";

export const LANGUAGE_NAMES: Record<Lang, string> = {
  en: "English",
  pl: "Polski",
};

const en = {
  "meta.description": "Live status of Politechnika Krakowska services",
  "header.subtitle": "Live status of Politechnika Krakowska services.",
  "picker.label": "Language",
  "dashboard.loading": "Loading status…",
  "dashboard.error": "Could not reach the status API: {error}",
  "dashboard.errorFallback": "failed to load",
  "dashboard.empty": "No checks recorded yet.",
  "card.latency": "Latency",
  "card.issuer": "Issuer: {issuer}",
  "chart.title": "Latency (ms)",
  "chart.latency": "latency",
  "chart.checked": "Checked",
  "chart.empty": "No history yet — waiting for the first checks.",
};

export type MessageKey = keyof typeof en;

const pl: Record<MessageKey, string> = {
  "meta.description": "Aktualny status usług Politechniki Krakowskiej",
  "header.subtitle": "Aktualny status usług Politechniki Krakowskiej.",
  "picker.label": "Język",
  "dashboard.loading": "Wczytywanie statusu…",
  "dashboard.error": "Nie udało się połączyć z API statusu: {error}",
  "dashboard.errorFallback": "błąd wczytywania",
  "dashboard.empty": "Brak zarejestrowanych sprawdzeń.",
  "card.latency": "Opóźnienie",
  "card.issuer": "Wystawca: {issuer}",
  "chart.title": "Opóźnienie (ms)",
  "chart.latency": "opóźnienie",
  "chart.checked": "Sprawdzono",
  "chart.empty": "Brak historii — oczekiwanie na pierwsze sprawdzenia.",
};

const messages: Record<Lang, Record<MessageKey, string>> = { en, pl };

function isLang(value: unknown): value is Lang {
  return typeof value === "string" && (LANGUAGES as readonly string[]).includes(value);
}

/** Saved user choice first, then the browser's preferred languages, then English. */
export function detectLanguage(): Lang {
  if (typeof window === "undefined") return DEFAULT_LANG;
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (isLang(saved)) return saved;
  } catch {
    // storage may be unavailable (privacy mode); fall through
  }
  const preferred = navigator.languages?.length ? navigator.languages : [navigator.language];
  for (const tag of preferred) {
    const base = tag?.toLowerCase().split("-")[0];
    if (isLang(base)) return base;
  }
  return DEFAULT_LANG;
}

// --- store -----------------------------------------------------------------

let current: Lang | null = null;
const listeners = new Set<() => void>();

function getLang(): Lang {
  if (current === null) {
    current = detectLanguage();
    applyToDocument(current);
  }
  return current;
}

function applyToDocument(lang: Lang) {
  if (typeof document === "undefined") return;
  document.documentElement.lang = lang;
  document
    .querySelector('meta[name="description"]')
    ?.setAttribute("content", messages[lang]["meta.description"]);
}

export function setLanguage(lang: Lang) {
  current = lang;
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch {
    // ignore
  }
  applyToDocument(lang);
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export type Translate = (key: MessageKey, params?: Record<string, string | number>) => string;

export function translate(lang: Lang): Translate {
  return (key, params) => {
    let text = messages[lang][key] ?? messages[DEFAULT_LANG][key] ?? key;
    if (params) {
      for (const [name, value] of Object.entries(params)) {
        text = text.replaceAll(`{${name}}`, String(value));
      }
    }
    return text;
  };
}

/** React hook: current language + translate function. Re-renders on change. */
export function useI18n() {
  // Server snapshot is the default language, so static HTML is English and the
  // client switches to the detected language right after hydration.
  const lang = useSyncExternalStore(subscribe, getLang, () => DEFAULT_LANG);
  return { lang, t: translate(lang), setLanguage };
}

// --- locale-aware formatting -------------------------------------------------

export function formatRelativeTime(iso: string, lang: Lang): string {
  const secs = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: "always", style: "narrow" });
  if (secs < 60) return rtf.format(-secs, "second");
  const mins = Math.round(secs / 60);
  if (mins < 60) return rtf.format(-mins, "minute");
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return rtf.format(-hrs, "hour");
  return rtf.format(-Math.round(hrs / 24), "day");
}
