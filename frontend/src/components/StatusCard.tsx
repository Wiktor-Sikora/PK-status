import type { CheckResult } from "../lib/api";

function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  const secs = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.round(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.round(hrs / 24)}d ago`;
}

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

export default function StatusCard({ result }: { result: CheckResult }) {
  const up = result.ok;
  const certWarn =
    result.cert_days_left !== undefined && result.cert_days_left < 30;

  return (
    <div className="rounded-xl border border-slate-700 bg-slate-800/60 p-6 shadow-lg">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-slate-100">
            {hostOf(result.target)}
          </h2>
          <a
            href={result.target}
            target="_blank"
            rel="noreferrer"
            className="text-sm text-slate-400 hover:text-slate-200"
          >
            {result.target}
          </a>
        </div>
        <span
          className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium ${
            up
              ? "bg-emerald-500/15 text-emerald-400"
              : "bg-rose-500/15 text-rose-400"
          }`}
        >
          <span
            className={`h-2.5 w-2.5 rounded-full ${
              up ? "bg-emerald-400" : "bg-rose-400"
            }`}
          />
          {up ? "Operational" : "Down"}
        </span>
      </div>

      <dl className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Stat label="Latency" value={`${result.latency_ms} ms`} />
        <Stat
          label="HTTP"
          value={result.status_code ? String(result.status_code) : "—"}
        />
        <Stat
          label="Cert expires"
          value={
            result.cert_days_left !== undefined
              ? `${result.cert_days_left}d`
              : "—"
          }
          warn={certWarn}
        />
        <Stat label="Checked" value={relativeTime(result.checked_at)} />
      </dl>

      {result.error && (
        <p className="mt-4 rounded-md bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
          {result.error}
        </p>
      )}
      {result.cert_issuer && (
        <p className="mt-4 text-xs text-slate-500">
          Issuer: {result.cert_issuer}
        </p>
      )}
    </div>
  );
}

function Stat({
  label,
  value,
  warn,
}: {
  label: string;
  value: string;
  warn?: boolean;
}) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-slate-500">{label}</dt>
      <dd
        className={`mt-1 text-xl font-semibold ${
          warn ? "text-amber-400" : "text-slate-100"
        }`}
      >
        {value}
      </dd>
    </div>
  );
}
