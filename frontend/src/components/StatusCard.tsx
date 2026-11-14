import type { CheckResult } from "../lib/api";
import { useState, type ReactNode } from "react";
import { useI18n } from "../lib/i18n";
import { hostOf, iconSrcFor } from "../lib/icons";

export default function StatusCard({
  result,
  children,
}: {
  result: CheckResult;
  children?: ReactNode;
}) {
  const { t } = useI18n();
  const title = result.name || hostOf(result.target);

  return (
    <div className="rounded-xl border border-slate-700 bg-slate-800/60 p-6 shadow-lg">
      <div className="flex items-center gap-3">
        <Favicon src={iconSrcFor(result)} label={title} />
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
          <Stat label={t("card.latency")} value={`${result.latency_ms} ms`} />
        </dl>
      </div>

      {result.error && (
        <p className="mt-4 rounded-md bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
          {result.error}
        </p>
      )}
      {result.cert_issuer && (
        <p className="mt-4 text-xs text-slate-500">
          {t("card.issuer", { issuer: result.cert_issuer })}
        </p>
      )}
      {children}
    </div>
  );
}

// Icons are stored as static files in public/favicons. When one is missing,
// fall back to the first letter of the service name.
function Favicon({ src, label }: { src: string; label: string }) {
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
      src={src}
      alt=""
      onError={() => setFailed(true)}
      className="h-12 w-12 shrink-0 object-contain"
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
