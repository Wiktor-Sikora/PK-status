import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceArea,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { CheckResult } from "../lib/api";
import { formatRelativeTime, useI18n } from "../lib/i18n";

// 24-hour HH:MM, no seconds, regardless of the browser's locale.
const timeFormat = new Intl.DateTimeFormat("en-GB", {
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

type Row = { key: string; latency: number | null };
type Gap = { start: string; end: string };

// Builds chart rows. Checks with a TLS certificate form the solid "latency"
// series. Each run of checks without a certificate becomes a red band covering
// that interval. Runs at the very start or end are clamped to the chart edge.
function buildChart(history: CheckResult[]): { data: Row[]; gaps: Gap[] } {
  const hasCert = history.map((h) => h.cert_not_after !== undefined);
  const data: Row[] = history.map((h, i) => ({
    key: h.checked_at,
    latency: hasCert[i] ? h.latency_ms : null,
  }));

  const gaps: Gap[] = [];
  let i = 0;
  while (i < history.length) {
    if (hasCert[i]) {
      i++;
      continue;
    }
    const start = i;
    while (i < history.length && !hasCert[i]) i++;
    const left = Math.max(start - 1, 0);
    const right = Math.min(i, data.length - 1);
    gaps.push({ start: data[left].key, end: data[right].key });
  }
  return { data, gaps };
}

export default function LatencyChart({ history }: { history: CheckResult[] }) {
  const { lang, t } = useI18n();
  const { data, gaps } = buildChart(history);

  if (data.length === 0) {
    return (
      <div className="mt-6 text-sm text-slate-400">
        {t("chart.empty")}
      </div>
    );
  }

  return (
    <div className="mt-6">
      <div className="mb-4 flex items-baseline justify-between gap-4">
        <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-400">
          {t("chart.title")}
        </h3>
        <p className="text-sm text-slate-400">
          {t("chart.checked")}{" "}
          <span className="font-semibold text-slate-100">
            {formatRelativeTime(history[history.length - 1].checked_at, lang)}
          </span>
        </p>
      </div>
      <div className="h-48 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 5, right: 0, bottom: 5, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
            {gaps.map((gap) => (
              <ReferenceArea
                key={`${gap.start}-${gap.end}`}
                x1={gap.start}
                x2={gap.end}
                fill="#ef4444"
                fillOpacity={0.2}
                stroke="none"
              />
            ))}
            <XAxis
              dataKey="key"
              tickFormatter={(value: string) => timeFormat.format(new Date(value))}
              tick={{ fill: "#94a3b8", fontSize: 11 }}
              minTickGap={40}
            />
            <YAxis
              width={40}
              tickLine={false}
              // Left-align labels at the chart's left edge so they line up
              // with the card content above.
              tick={({ y, payload }: { y: number; payload: { value: number } }) => (
                <text x={0} y={y} dy={4} textAnchor="start" fill="#94a3b8" fontSize={11}>
                  {payload.value}
                </text>
              )}
            />
            <Tooltip
              labelFormatter={(value) => timeFormat.format(new Date(String(value)))}
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
              name={t("chart.latency")}
              stroke="#34d399"
              strokeWidth={2}
              dot={false}
              connectNulls={false}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
