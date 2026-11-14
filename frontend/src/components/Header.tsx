import { LANGUAGES, LANGUAGE_FLAGS, LANGUAGE_NAMES, type Lang, useI18n } from "../lib/i18n";
import { setSettingsOpen, useSettingsOpen } from "../lib/settings";

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
  const settingsOpen = useSettingsOpen();

  return (
    <div className="flex shrink-0 items-center gap-2 text-sm text-slate-400">
      <label className="flex items-center gap-2">
        <span className="sr-only">{t("picker.label")}</span>
        {/*<svg
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
        </svg>*/}
        <select
          value={lang}
          onChange={(e) => setLanguage(e.target.value as Lang)}
          className="cursor-pointer rounded-md border border-slate-700 bg-slate-800 px-2 py-1 text-slate-100 hover:border-slate-500 focus:border-emerald-400 focus:outline-none"
        >
          {LANGUAGES.map((code) => (
            <option key={code} value={code} lang={code}>
              {LANGUAGE_FLAGS[code]} {LANGUAGE_NAMES[code]}
            </option>
          ))}
        </select>
      </label>
      <button
        type="button"
        onClick={() => setSettingsOpen(!settingsOpen)}
        aria-label={t("settings.open")}
        aria-pressed={settingsOpen}
        title={t("settings.open")}
        className={`cursor-pointer rounded-md p-1 hover:bg-slate-800 hover:text-slate-100 ${
          settingsOpen ? "bg-slate-800 text-emerald-400" : ""
        }`}
      >
        <svg
          aria-hidden="true"
          viewBox="0 0 24 24"
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
          <circle cx="12" cy="12" r="3" />
        </svg>
      </button>
    </div>
  );
}
