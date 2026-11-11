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
import { formatRelativeTime, useI18n } from "../lib/i18n";

// 24-hour HH:MM, no seconds, regardless of the browser's locale.
const timeFormat = new Intl.DateTimeFormat("en-GB", {
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

type Row = { time: string; latency: number | null; [gap: string]: number | string | null };

// Builds chart rows. Checks with a TLS certificate form the solid "latency"
// series. Each run of checks without a certificate becomes its own "gapN"
// series holding only the two certified points around the run, drawn as a
// dashed bridge. Runs at the very start or end have nothing to bridge to.
function buildChart(history: CheckResult[]): { data: Row[]; gapKeys: string[] } {
  const hasCert = history.map((h) => h.cert_not_after !== undefined);
  const data: Row[] = history.map((h, i) => ({
    time: timeFormat.format(new Date(h.checked_at)),
    latency: hasCert[i] ? h.latency_ms : null,
  }));

  const gapKeys: string[] = [];
  let i = 0;
  while (i < history.length) {
    if (hasCert[i]) {
      i++;
      continue;
    }
    const start = i;
    while (i < history.length && !hasCert[i]) i++;
    const before = start - 1;
    const after = i;
    if (before >= 0 && after < history.length) {
      const key = `gap${gapKeys.length}`;
      gapKeys.push(key);
      data[before][key] = history[before].latency_ms;
      data[after][key] = history[after].latency_ms;
    }
  }
  return { data, gapKeys };
}

export default function LatencyChart({ history }: { history: CheckResult[] }) {
  const { lang, t } = useI18n();
  const { data, gapKeys } = buildChart(history);

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
            <XAxis dataKey="time" tick={{ fill: "#94a3b8", fontSize: 11 }} minTickGap={40} />
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
              contentStyle={{
                background: "#0f172a",
                border: "1px solid #334155",
                borderRadius: 8,
                color: "#e2e8f0",
              }}
            />
            {gapKeys.map((key) => (
              <Line
                key={key}
                type="linear"
                dataKey={key}
                stroke="#34d399"
                strokeWidth={2}
                strokeDasharray="5 5"
                dot={false}
                connectNulls
                tooltipType="none"
                isAnimationActive={false}
              />
            ))}
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
