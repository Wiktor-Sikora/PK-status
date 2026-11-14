import { useState } from "react";
import type { CheckResult } from "../lib/api";
import { useI18n } from "../lib/i18n";
import { CATEGORIES, categoryFor, hostOf, iconSrcFor } from "../lib/icons";
import { setSettingsOpen, setTargetHidden, showAllTargets, useHiddenTargets } from "../lib/settings";

interface Group {
  key: string;
  name: string;
  icon?: string;
  results: CheckResult[];
}

function groupResults(results: CheckResult[]): Group[] {
  const groups: Group[] = CATEGORIES.map((category) => ({
    key: category.key,
    name: category.name,
    icon: category.icon,
    results: [],
  }));
  const byKey = new Map(groups.map((group) => [group.key, group]));
  const other: Group = { key: "other", name: "", results: [] };

  for (const result of results) {
    const category = categoryFor(result);
    (category ? byKey.get(category.key)! : other).results.push(result);
  }

  const ordered = groups.filter((group) => group.results.length > 0);
  if (other.results.length > 0) ordered.push(other);
  return ordered;
}

export default function Settings({ results }: { results: CheckResult[] }) {
  const { t } = useI18n();
  const hidden = useHiddenTargets();
  const groups = groupResults(results);

  return (
    <section className="rounded-xl border border-slate-700 bg-slate-800/60 p-6 shadow-lg">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-slate-100">{t("settings.title")}</h2>
          <p className="mt-1 text-sm text-slate-400">{t("settings.description")}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            onClick={showAllTargets}
            disabled={hidden.size === 0}
            className="cursor-pointer rounded-md border border-slate-600 px-3 py-1 text-sm text-slate-300 hover:border-slate-400 hover:text-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {t("settings.showAll")}
          </button>
          <button
            type="button"
            onClick={() => setSettingsOpen(false)}
            className="cursor-pointer rounded-md bg-emerald-500 px-3 py-1 text-sm font-medium text-slate-900 hover:bg-emerald-400"
          >
            {t("settings.close")}
          </button>
        </div>
      </div>

      {results.length === 0 ? (
        <p className="mt-6 text-slate-400">{t("dashboard.empty")}</p>
      ) : (
        <div className="mt-6 space-y-3">
          {groups.map((group) => {
            const label = group.key === "other" ? t("settings.other") : group.name;
            const visibleCount = group.results.filter((result) => !hidden.has(result.target)).length;
            return (
              <details key={group.key} className="group rounded-lg border border-slate-700">
                <summary className="flex cursor-pointer list-none items-center gap-3 px-4 py-3 text-sm hover:bg-slate-700/30 [&::-webkit-details-marker]:hidden">
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 24 24"
                    className="h-4 w-4 shrink-0 transition-transform group-open:rotate-90"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={2}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M9 18l6-6-6-6" />
                  </svg>
                  {group.icon && (
                    <img src={group.icon} alt="" className="h-5 w-5 shrink-0 object-contain" />
                  )}
                  <span className="min-w-0 flex-1 truncate font-medium text-slate-200">{label}</span>
                  <span className="shrink-0 text-xs text-slate-500">
                    {visibleCount}/{group.results.length}
                  </span>
                </summary>
                <ul className="divide-y divide-slate-700/70 border-t border-slate-700/70">
                  {group.results.map((result) => {
                    const visible = !hidden.has(result.target);
                    const title = result.name || result.target;
                    return (
                      <li key={result.target}>
                        <label className="flex cursor-pointer items-center gap-3 px-4 py-3">
                          <input
                            type="checkbox"
                            checked={visible}
                            onChange={(e) => setTargetHidden(result.target, !e.target.checked)}
                            className="h-4 w-4 shrink-0 cursor-pointer accent-emerald-400"
                          />
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-slate-100">{title}</span>
                            {title !== result.target && (
                              <span className="block truncate text-xs text-slate-500">
                                {result.target}
                              </span>
                            )}
                          </span>
                          {!hostOf(result.target).split(".").includes("eclipse") && (
                            <EntryIcon result={result} />
                          )}
                        </label>
                      </li>
                    );
                  })}
                </ul>
              </details>
            );
          })}
        </div>
      )}
    </section>
  );
}

function EntryIcon({ result }: { result: CheckResult }) {
  const [failed, setFailed] = useState(false);
  const label = result.name || result.target;

  if (failed) {
    return (
      <span
        aria-hidden="true"
        className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-slate-700 text-xs font-semibold uppercase text-slate-200"
      >
        {label.charAt(0)}
      </span>
    );
  }

  return (
    <img
      src={iconSrcFor(result)}
      alt=""
      onError={() => setFailed(true)}
      className="h-6 w-6 shrink-0 object-contain"
    />
  );
}
