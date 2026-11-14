import { useEffect, useState } from "react";
import { fetchHistory, fetchStatus, type CheckResult } from "../lib/api";
import { useI18n } from "../lib/i18n";
import { useHiddenTargets, useSettingsOpen } from "../lib/settings";
import StatusCard from "./StatusCard";
import LatencyChart from "./LatencyChart";
import Settings from "./Settings";

const REFRESH_MS = 30_000;

export default function Dashboard() {
  const { t } = useI18n();
  const hidden = useHiddenTargets();
  const settingsOpen = useSettingsOpen();
  const [status, setStatus] = useState<CheckResult[]>([]);
  const [history, setHistory] = useState<Record<string, CheckResult[]>>({});
  // undefined = no error; null = error without a message (shown as a translated fallback)
  const [error, setError] = useState<string | null | undefined>(undefined);
  const [loading, setLoading] = useState(true);

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

  const visible = status.filter((s) => !hidden.has(s.target));

  if (visible.length === 0) {
    return <p className="text-slate-400">{t("dashboard.noSelection")}</p>;
  }

  return (
    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
      {visible.map((s) => (
        <StatusCard key={s.target} result={s}>
          <LatencyChart history={history[s.target] ?? []} />
        </StatusCard>
      ))}
    </div>
  );
}
