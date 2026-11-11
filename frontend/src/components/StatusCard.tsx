import type { CheckResult } from "../lib/api";
import { useState, type ReactNode } from "react";

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

export default function StatusCard({
  result,
  children,
}: {
  result: CheckResult;
  children?: ReactNode;
}) {
  const title = result.name || hostOf(result.target);

  return (
    <div className="rounded-xl border border-slate-700 bg-slate-800/60 p-6 shadow-lg">
      <div className="flex items-center gap-3">
        <Favicon host={hostOf(result.target)} label={title} />
        <div className="min-w-0 flex-1">
          <h2 className="text-lg font-semibold text-slate-100">
            {title}
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
        <dl className="shrink-0 text-right">
          <Stat label="Latency" value={`${result.latency_ms} ms`} />
        </dl>
      </div>

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
      {children}
    </div>
  );
}

// Favicons are stored as static files in public/favicons/<host>.png. When one
// is missing, fall back to the first letter of the service name.
function Favicon({ host, label }: { host: string; label: string }) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <span
        aria-hidden="true"
        className="flex h-12 w-12 shrink-0 items-center justify-center rounded-md bg-slate-700 text-xl font-semibold uppercase text-slate-200"
      >
        {label.charAt(0)}
      </span>
    );
  }

  return (
    <img
      src={`/favicons/${host}.png`}
      alt=""
      onError={() => setFailed(true)}
      className="h-12 w-12 shrink-0 rounded-md object-contain"
    />
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wide text-slate-500">{label}</dt>
      <dd className="mt-1 text-xl font-semibold text-slate-100">{value}</dd>
    </div>
  );
}
