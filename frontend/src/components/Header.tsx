import { LANGUAGES, LANGUAGE_NAMES, type Lang, useI18n } from "../lib/i18n";

export default function Header() {
  const { t } = useI18n();

  return (
    <header className="mb-8 flex items-start justify-between gap-4">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">PK Status</h1>
        <p className="mt-1 text-slate-400">{t("header.subtitle")}</p>
      </div>
      <LanguagePicker />
    </header>
  );
}

function LanguagePicker() {
  const { lang, t, setLanguage } = useI18n();

  return (
    <label className="flex shrink-0 items-center gap-2 text-sm text-slate-400">
      <span className="sr-only">{t("picker.label")}</span>
      <svg
        aria-hidden="true"
        viewBox="0 0 24 24"
        className="h-4 w-4"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="10" />
        <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
      </svg>
      <select
        value={lang}
        onChange={(e) => setLanguage(e.target.value as Lang)}
        className="cursor-pointer rounded-md border border-slate-700 bg-slate-800 px-2 py-1 text-slate-100 hover:border-slate-500 focus:border-emerald-400 focus:outline-none"
      >
        {LANGUAGES.map((code) => (
          <option key={code} value={code} lang={code}>
            {LANGUAGE_NAMES[code]}
          </option>
        ))}
      </select>
    </label>
  );
}
