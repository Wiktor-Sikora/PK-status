// User preferences: which targets are hidden from the dashboard, and whether
// the settings tab is open. A tiny shared store keeps the separate React
// islands (Header, Dashboard) in sync.
import { useSyncExternalStore } from "react";

const HIDDEN_KEY = "pk-status-hidden-targets";
const ORDER_KEY = "pk-status-card-order";
const EMPTY: ReadonlySet<string> = new Set();
const EMPTY_ORDER: readonly string[] = [];

let hidden: Set<string> | null = null;
let order: string[] | null = null;
let settingsOpen = false;
const listeners = new Set<() => void>();

function emit() {
  listeners.forEach((listener) => listener());
}

function loadHidden(): ReadonlySet<string> {
  if (hidden !== null) return hidden;
  if (typeof window === "undefined") return EMPTY;
  try {
    const raw = localStorage.getItem(HIDDEN_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    hidden = new Set(
      Array.isArray(parsed) ? parsed.filter((value): value is string => typeof value === "string") : [],
    );
  } catch {
    hidden = new Set();
  }
  return hidden;
}

export function setTargetHidden(target: string, isHidden: boolean) {
  const next = new Set(loadHidden());
  if (isHidden) next.add(target);
  else next.delete(target);
  hidden = next;
  try {
    localStorage.setItem(HIDDEN_KEY, JSON.stringify([...next]));
  } catch {
    // ignore unavailable storage
  }
  emit();
}

export function showAllTargets() {
  hidden = new Set();
  try {
    localStorage.removeItem(HIDDEN_KEY);
  } catch {
    // ignore
  }
  emit();
}

export function hideAllTargets(targets: Iterable<string>) {
  hidden = new Set(targets);
  try {
    localStorage.setItem(HIDDEN_KEY, JSON.stringify([...hidden]));
  } catch {
    // ignore unavailable storage
  }
  emit();
}

export function setSettingsOpen(open: boolean) {
  settingsOpen = open;
  emit();
}

function loadOrder(): readonly string[] {
  if (order !== null) return order;
  if (typeof window === "undefined") return EMPTY_ORDER;
  try {
    const raw = localStorage.getItem(ORDER_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    order = Array.isArray(parsed)
      ? parsed.filter((value): value is string => typeof value === "string")
      : [];
  } catch {
    order = [];
  }
  return order;
}

export function setCardOrder(next: string[]) {
  order = next;
  try {
    localStorage.setItem(ORDER_KEY, JSON.stringify(next));
  } catch {
    // ignore unavailable storage
  }
  emit();
}

export function useCardOrder(): readonly string[] {
  return useSyncExternalStore(subscribe, loadOrder, () => EMPTY_ORDER);
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useHiddenTargets(): ReadonlySet<string> {
  return useSyncExternalStore(subscribe, loadHidden, () => EMPTY);
}

export function useSettingsOpen(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => settingsOpen,
    () => false,
  );
}
