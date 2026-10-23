// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// The interface languages GoPMgr plans to offer (ADR-008, wave 1). A
// language is offered only once its catalog is registered in i18n.svelte.ts;
// until then it resolves to English.

export const LOCALES = [
  { code: 'en', name: 'English' },
  { code: 'fr', name: 'Français' },
  { code: 'de', name: 'Deutsch' },
  { code: 'es', name: 'Español' },
  { code: 'it', name: 'Italiano' },
  { code: 'pt-PT', name: 'Português' },
  { code: 'nl', name: 'Nederlands' },
  { code: 'ja', name: '日本語' },
  { code: 'ko', name: '한국어' },
] as const;

export type LocaleCode = (typeof LOCALES)[number]['code'];

export const SOURCE_LOCALE: LocaleCode = 'en';

// baseLanguage returns the language subtag: "pt" for "pt-BR".
export function baseLanguage(tag: string): string {
  return tag.trim().toLowerCase().split(/[-_]/)[0] ?? '';
}

// matchLocale returns the offered locale for a BCP 47 tag, matching the
// exact code first and then the language alone, so "fr-CA" finds "fr" and
// "pt-BR" finds "pt-PT". It returns null when nothing offered matches.
export function matchLocale(tag: string, offered: readonly LocaleCode[]): LocaleCode | null {
  const lower = tag.trim().toLowerCase();
  if (!lower) return null;
  const exact = offered.find((code) => code.toLowerCase() === lower);
  if (exact) return exact;
  const base = baseLanguage(lower);
  return offered.find((code) => baseLanguage(code) === base) ?? null;
}

// resolveLocale picks the interface language: the user's choice when it is
// offered, else the first offered system language, else English.
export function resolveLocale(
  choice: string | null | undefined,
  preferred: readonly string[],
  offered: readonly LocaleCode[],
): LocaleCode {
  const chosen = choice ? matchLocale(choice, offered) : null;
  if (chosen) return chosen;
  for (const tag of preferred) {
    const match = matchLocale(tag, offered);
    if (match) return match;
  }
  return SOURCE_LOCALE;
}

// formatLocaleFor picks the locale for dates and numbers. With no choice
// (automatic), formats follow the system's first language, as they always
// have. With a choice, they follow it, keeping the system's region when the
// system speaks the same language, so English in Britain formats as en-GB.
export function formatLocaleFor(
  choice: string | null | undefined,
  locale: LocaleCode,
  preferred: readonly string[],
): string {
  if (!choice) return preferred.find((tag) => tag.trim() !== '') ?? locale;
  const sameLanguage = preferred.find((tag) => baseLanguage(tag) === baseLanguage(locale));
  return sameLanguage ?? locale;
}
