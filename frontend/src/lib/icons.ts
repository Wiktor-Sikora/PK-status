import type { CheckResult } from "./api";

export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

export interface Faculty {
  key: string;
  name: string;
  icon: string;
  aliases?: string[];
}

// Faculty crests are stored in public/favicons, named by faculty abbreviation.
export const FACULTIES: Faculty[] = [
  { key: "wa", name: "WA", icon: "/favicons/WA.svg", aliases: ["arch"] },
  { key: "wieik", name: "WIEiK", icon: "/favicons/WIEiK.svg", aliases: ["elektron"] },
  { key: "wiim", name: "WIiM", icon: "/favicons/WIiM.png", aliases: ["it"] },
  { key: "wiitch", name: "WIiTCh", icon: "/favicons/WIiTCh.png", aliases: ["chemia"] },
  { key: "wilit", name: "WILiT", icon: "/favicons/WILiT.png" },
  { key: "wimif", name: "WIMiF", icon: "/favicons/WIMiF.svg", aliases: ["imf"] },
  { key: "wisie", name: "WISiE", icon: "/favicons/WISiE.svg" },
  { key: "wm", name: "WM", icon: "/favicons/WM.svg", aliases: ["mech"] },
];

const FACULTY_BY_ALIAS = new Map<string, Faculty>();
for (const faculty of FACULTIES) {
  FACULTY_BY_ALIAS.set(faculty.key, faculty);
  for (const alias of faculty.aliases ?? []) FACULTY_BY_ALIAS.set(alias, faculty);
}

// Icons for non-faculty PK sites, keyed by target name or host label.
const SITE_ICONS: Record<string, string> = {
  cd: "/favicons/cd.png",
  cpip: "/favicons/cpip.png",
  ctt: "/favicons/ctt.png",
  delta: "/favicons/delta.png",
  "ehms.pk.edu.pl": "/favicons/ehms.pk.edu.pl.png",
  een: "/favicons/een.png",
  iro: "/favicons/iro.png",
  sd: "/favicons/SD.svg",
  szkoladoktorska: "/favicons/SD.svg",
  transfer: "/favicons/ctt.png",
  "www.biblos.pk.edu.pl": "/favicons/www.biblos.pk.edu.pl.png",
};

const PK_ICON = "/favicons/pk.edu.pl.png";

// Match either the target name or a host label (e.g. plany.wieik… → WIEiK).
export function facultyFor(result: CheckResult): Faculty | undefined {
  const host = hostOf(result.target);
  const candidates = [result.name ?? "", host, ...host.split(".")];
  for (const candidate of candidates) {
    const faculty = FACULTY_BY_ALIAS.get(candidate.trim().toLowerCase());
    if (faculty) return faculty;
  }
  return undefined;
}

export function iconSrcFor(result: CheckResult): string {
  const faculty = facultyFor(result);
  if (faculty) return faculty.icon;

  const host = hostOf(result.target);
  const candidates = [result.name ?? "", host, ...host.split(".")];
  for (const candidate of candidates) {
    const icon = SITE_ICONS[candidate.trim().toLowerCase()];
    if (icon) return icon;
  }

  // Eclipse keeps its letter fallback until it gets a dedicated icon.
  if (host.split(".").includes("eclipse")) return `/favicons/${host}.png`;

  return PK_ICON;
}
