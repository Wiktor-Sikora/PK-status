import { useEffect, useRef, useState, type ReactNode } from "react";
import { fetchHistory, fetchStatus, type CheckResult } from "../lib/api";
import { useI18n } from "../lib/i18n";
import { setCardOrder, useCardOrder, useHiddenTargets, useSettingsOpen } from "../lib/settings";
import StatusCard from "./StatusCard";
import LatencyChart from "./LatencyChart";
import Settings from "./Settings";

const REFRESH_MS = 30_000;

function applyOrder(results: CheckResult[], order: readonly string[]): CheckResult[] {
  const rank = new Map(order.map((target, index) => [target, index]));
  return results
    .map((result, index) => ({
      result,
      index,
      rank: rank.get(result.target) ?? Number.MAX_SAFE_INTEGER,
    }))
    .sort((a, b) => a.rank - b.rank || a.index - b.index)
    .map((entry) => entry.result);
}

function moveItem(order: string[], from: string, to: string): string[] {
  const fromIndex = order.indexOf(from);
  const toIndex = order.indexOf(to);
  if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) return order;
  const next = [...order];
  const [moved] = next.splice(fromIndex, 1);
  next.splice(toIndex, 0, moved);
  return next;
}

export default function Dashboard() {
  const { t } = useI18n();
  const hidden = useHiddenTargets();
  const order = useCardOrder();
  const settingsOpen = useSettingsOpen();
  const [status, setStatus] = useState<CheckResult[]>([]);
  const [history, setHistory] = useState<Record<string, CheckResult[]>>({});
  // undefined = no error; null = error without a message (shown as a translated fallback)
  const [error, setError] = useState<string | null | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [dragging, setDragging] = useState<string | null>(null);
  const [previewOrder, setPreviewOrder] = useState<string[] | null>(null);
  const previewRef = useRef<string[] | null>(null);

  useEffect(() => {
    let active = true;

    async function load() {
      try {
        const latest = await fetchStatus();
        if (!active) return;
        setStatus(latest);
        setError(undefined);

        const entries = await Promise.all(
          latest.map(async (s) => [s.target, await fetchHistory(s.target, 100)] as const),
        );
        if (active) setHistory(Object.fromEntries(entries));
      } catch (e) {
        if (active) setError(e instanceof Error ? e.message : null);
      } finally {
        if (active) setLoading(false);
      }
    }

    load();
    const id = setInterval(load, REFRESH_MS);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);

  useEffect(() => {
    if (!dragging) return;
    const dragged = dragging;

    function setPreview(next: string[] | null) {
      previewRef.current = next;
      setPreviewOrder(next);
    }

    function handleMove(event: PointerEvent) {
      const element = document
        .elementFromPoint(event.clientX, event.clientY)
        ?.closest<HTMLElement>("[data-card-target]");
      const target = element?.dataset.cardTarget;
      if (!target || target === dragged) return;

      const base = previewRef.current ?? applyOrder(status, order).map((s) => s.target);
      const next = moveItem(base, dragged, target);
      if (next !== base) setPreview(next);
    }

    function finish(commit: boolean) {
      if (commit && previewRef.current) setCardOrder(previewRef.current);
      setPreview(null);
      setDragging(null);
    }

    function handleUp() {
      finish(true);
    }
    function handleCancel() {
      finish(false);
    }

    window.addEventListener("pointermove", handleMove);
    window.addEventListener("pointerup", handleUp);
    window.addEventListener("pointercancel", handleCancel);
    return () => {
      window.removeEventListener("pointermove", handleMove);
      window.removeEventListener("pointerup", handleUp);
      window.removeEventListener("pointercancel", handleCancel);
    };
  }, [dragging, status, order]);

  if (loading) {
    return <p className="text-slate-400">{t("dashboard.loading")}</p>;
  }

  if (error !== undefined) {
    return (
      <p className="rounded-md bg-rose-500/10 px-4 py-3 text-rose-300">
        {t("dashboard.error", { error: error ?? t("dashboard.errorFallback") })}
      </p>
    );
  }

  if (status.length === 0) {
    return settingsOpen ? (
      <Settings results={status} />
    ) : (
      <p className="text-slate-400">{t("dashboard.empty")}</p>
    );
  }

  if (settingsOpen) {
    return <Settings results={status} />;
  }

  const ordered = applyOrder(status, previewOrder ?? order);
  const visible = ordered.filter((s) => !hidden.has(s.target));

  if (visible.length === 0) {
    return <p className="text-slate-400">{t("dashboard.noSelection")}</p>;
  }

  function startDrag(target: string) {
    previewRef.current = applyOrder(status, order).map((s) => s.target);
    setPreviewOrder(previewRef.current);
    setDragging(target);
  }

  return (
    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
      {visible.map((s) => (
        <SortableCard
          key={s.target}
          target={s.target}
          dragLabel={t("card.drag")}
          isDragging={dragging === s.target}
          onDragStart={() => startDrag(s.target)}
        >
          <StatusCard result={s}>
            <LatencyChart history={history[s.target] ?? []} />
          </StatusCard>
        </SortableCard>
      ))}
    </div>
  );
}

function SortableCard({
  target,
  dragLabel,
  isDragging,
  onDragStart,
  children,
}: {
  target: string;
  dragLabel: string;
  isDragging: boolean;
  onDragStart: () => void;
  children: ReactNode;
}) {
  return (
    <div
      data-card-target={target}
      className={`group relative rounded-xl transition ${
        isDragging ? "z-10 opacity-80 ring-2 ring-emerald-400 shadow-2xl" : ""
      }`}
    >
      <button
        type="button"
        aria-label={dragLabel}
        title={dragLabel}
        onPointerDown={(event) => {
          if (event.pointerType === "mouse" && event.button !== 0) return;
          event.preventDefault();
          onDragStart();
        }}
        className="absolute left-1.5 top-1.5 z-10 flex h-6 w-6 cursor-grab touch-none items-center justify-center rounded text-slate-500 opacity-0 transition-opacity hover:bg-slate-700 hover:text-slate-200 focus-visible:opacity-100 group-hover:opacity-100 active:cursor-grabbing"
      >
        <svg aria-hidden="true" viewBox="0 0 16 16" fill="currentColor" className="h-4 w-4">
          <circle cx="6" cy="4" r="1.3" />
          <circle cx="10" cy="4" r="1.3" />
          <circle cx="6" cy="8" r="1.3" />
          <circle cx="10" cy="8" r="1.3" />
          <circle cx="6" cy="12" r="1.3" />
          <circle cx="10" cy="12" r="1.3" />
        </svg>
      </button>
      {children}
    </div>
  );
}
