import { useEffect, useState } from "react";
import { fetchHistory, fetchStatus, type CheckResult } from "../lib/api";
import StatusCard from "./StatusCard";
import LatencyChart from "./LatencyChart";

const REFRESH_MS = 30_000;

export default function Dashboard() {
  const [status, setStatus] = useState<CheckResult[]>([]);
  const [history, setHistory] = useState<CheckResult[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;

    async function load() {
      try {
        const latest = await fetchStatus();
        if (!active) return;
        setStatus(latest);
        setError(null);

        // For now there is a single primary target; chart the first one.
        if (latest.length > 0) {
          const hist = await fetchHistory(latest[0].target, 100);
          if (active) setHistory(hist);
        }
      } catch (e) {
        if (active) setError(e instanceof Error ? e.message : "failed to load");
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
    return <p className="text-slate-400">Loading status…</p>;
  }

  if (error) {
    return (
      <p className="rounded-md bg-rose-500/10 px-4 py-3 text-rose-300">
        Could not reach the status API: {error}
      </p>
    );
  }

  if (status.length === 0) {
    return <p className="text-slate-400">No checks recorded yet.</p>;
  }

  return (
    <div className="space-y-6">
      {status.map((s) => (
        <StatusCard key={s.target} result={s} />
      ))}
      <LatencyChart history={history} />
    </div>
  );
}
