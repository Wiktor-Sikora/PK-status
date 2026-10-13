import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { CheckResult } from "../lib/api";

export default function LatencyChart({ history }: { history: CheckResult[] }) {
  const data = history.map((h) => ({
    time: new Date(h.checked_at).toLocaleTimeString(),
    latency: h.ok ? h.latency_ms : null,
    ok: h.ok,
  }));

  if (data.length === 0) {
    return (
      <div className="rounded-xl border border-slate-700 bg-slate-800/60 p-6 text-sm text-slate-400">
        No history yet — waiting for the first checks.
      </div>
    );
  }

  return (
    <div className="rounded-xl border border-slate-700 bg-slate-800/60 p-6 shadow-lg">
      <h3 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-400">
        Latency (ms)
      </h3>
      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 5, right: 10, bottom: 5, left: -10 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
            <XAxis dataKey="time" tick={{ fill: "#94a3b8", fontSize: 11 }} minTickGap={40} />
            <YAxis tick={{ fill: "#94a3b8", fontSize: 11 }} />
            <Tooltip
              contentStyle={{
                background: "#0f172a",
                border: "1px solid #334155",
                borderRadius: 8,
                color: "#e2e8f0",
              }}
            />
            <Line
              type="monotone"
              dataKey="latency"
              stroke="#34d399"
              strokeWidth={2}
              dot={false}
              connectNulls
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
