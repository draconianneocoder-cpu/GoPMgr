// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Interface language: the active catalog, t() for text, and the format
// locale for dates and numbers. See docs/design/ADR-008-localization.md.

import { render, type Catalog, type MessageParams } from './catalog';
import { LOCALES, SOURCE_LOCALE, formatLocaleFor, resolveLocale, type LocaleCode } from './locales';
import { en, type MessageKey } from './messages/en';

// Registered catalogs. A language is offered once its catalog is here and
// passes checkCatalog against English.
const catalogs: Partial<Record<LocaleCode, Catalog>> = { en };

export const offeredLocales = LOCALES.filter((l) => l.code in catalogs);

// registeredCatalogs lets the catalog checks test every registered language.
export const registeredCatalogs: Readonly<Partial<Record<LocaleCode, Catalog>>> = catalogs;

export const i18n = $state<{ locale: LocaleCode; formatLocale: string }>({
  locale: SOURCE_LOCALE,
  formatLocale: SOURCE_LOCALE,
});

// systemLanguages returns the web view's languages, most preferred first.
export function systemLanguages(): readonly string[] {
  if (typeof navigator === 'undefined') return [];
  const list = navigator.languages?.length ? navigator.languages : [navigator.language];
  return list.filter((tag): tag is string => typeof tag === 'string' && tag !== '');
}

// applyLanguage sets the interface language from a user's choice ('' or
// null means automatic) and updates <html lang>, which screen readers,
// hyphenation, CSS text-transform, and CJK glyph selection all follow.
export function applyLanguage(choice?: string | null, preferred: readonly string[] = systemLanguages()): LocaleCode {
  const offered = offeredLocales.map((l) => l.code);
  const locale = resolveLocale(choice, preferred, offered);
  i18n.locale = locale;
  i18n.formatLocale = formatLocaleFor(choice, locale, preferred);
  if (typeof document !== 'undefined') document.documentElement.lang = locale;
  return locale;
}

// t returns the text for key in the interface language, falling back to
// English for a key a translation lacks.
export function t(key: MessageKey, params?: MessageParams): string {
  const message = catalogs[i18n.locale]?.[key] ?? en[key];
  return render(message, params, i18n.locale, i18n.formatLocale);
}

// formatDate, formatTime, and formatNumber format in the format locale.
export function formatDate(value: Date | number | string, options?: Intl.DateTimeFormatOptions): string {
  return new Date(value).toLocaleDateString(i18n.formatLocale, options);
}

export function formatTime(value: Date | number | string, options?: Intl.DateTimeFormatOptions): string {
  return new Date(value).toLocaleTimeString(i18n.formatLocale, options);
}

export function formatNumber(value: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(i18n.formatLocale, options).format(value);
}
