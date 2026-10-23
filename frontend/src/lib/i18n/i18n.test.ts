// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it, vi } from 'vitest';

import { render } from './catalog';
import { applyLanguage, formatDate, formatNumber, formatTime, i18n, offeredLocales, systemLanguages, t } from './i18n.svelte';
import { canonicalTag, formatLocaleFor, matchLocale, resolveLocale, type LocaleCode } from './locales';

const wave1: LocaleCode[] = ['en', 'fr', 'de', 'es', 'it', 'pt-PT', 'nl', 'ja', 'ko'];

afterEach(() => {
  vi.restoreAllMocks();
  applyLanguage('', ['en-US']);
});

const oct23 = Date.UTC(2026, 9, 23, 15, 4);

describe('language resolution', () => {
  it.each([
    ['fr-CA', 'fr'],
    ['pt-BR', 'pt-PT'],
    ['PT', 'pt-PT'],
    ['ja-JP', 'ja'],
    ['ko', 'ko'],
    ['de_CH', 'de'],
    ['zh-CN', null],
    ['', null],
  ])('matches %s to %s', (tag, want) => {
    expect(matchLocale(tag, wave1)).toBe(want);
  });

  it('prefers the choice, then the first offered system language, then English', () => {
    expect(resolveLocale('de', ['fr-FR'], wave1)).toBe('de');
    expect(resolveLocale('', ['zh-CN', 'ko-KR', 'fr-FR'], wave1)).toBe('ko');
    expect(resolveLocale('xx', ['zh-CN'], wave1)).toBe('en');
    expect(resolveLocale(null, [], wave1)).toBe('en');
  });

  it('keeps system formats when automatic, and the system region when it shares the language', () => {
    expect(formatLocaleFor('', 'en', ['de-DE', 'en-US'])).toBe('de-DE');
    expect(formatLocaleFor('en', 'en', ['de-DE', 'en-GB'])).toBe('en-GB');
    expect(formatLocaleFor('fr', 'fr', ['de-DE'])).toBe('fr');
    expect(formatLocaleFor('', 'en', [])).toBe('en');
  });
});

describe('applyLanguage', () => {
  it('offers English only until another catalog is registered', () => {
    expect(offeredLocales.map((l) => l.code)).toEqual(['en']);
  });

  it('sets <html lang> and keeps a German system’s date formats while the text stays English', () => {
    expect(applyLanguage('', ['de-DE'])).toBe('en');
    expect(document.documentElement.lang).toBe('en');
    expect(i18n.formatLocale).toBe('de-DE');
    expect(formatDate(Date.UTC(2026, 9, 23, 12), { timeZone: 'UTC' })).toBe('23.10.2026');
    expect(formatNumber(1284500.5)).toBe('1.284.500,5');
  });

  it('falls back to English for a language without a catalog', () => {
    expect(applyLanguage('ja', ['ja-JP'])).toBe('en');
    expect(t('login.submit')).toBe('SIGN IN');
  });
});

describe('t', () => {
  it('returns the English text and fills placeholders', () => {
    expect(t('createAccount.title')).toBe('Create your account');
    expect(t('recoveryCodes.failed.body', { error: 'disk full' })).toContain('(disk full)');
  });

  it('leaves a placeholder with no value visible', () => {
    expect(t('recoveryCodes.failed.body')).toContain('({error})');
  });
});

describe('system language tags', () => {
  it.each([
    ['de_CH', 'de-CH'],
    ['en_US.UTF-8', 'en-US'],
    ['sr_RS@latin', 'sr-RS'],
    [' fr-ca ', 'fr-CA'],
    ['C', null],
    ['C.UTF-8', null],
    ['POSIX', null],
    ['', null],
    ['x', null],
    ['!!', null],
  ])('canonicalTag(%j) is %j', (tag, want) => {
    expect(canonicalTag(tag)).toBe(want);
  });

  it('formats in the system region when the system reports a POSIX tag', () => {
    expect(formatLocaleFor('', 'en', ['de_CH.UTF-8'])).toBe('de-CH');
    expect(formatLocaleFor('', 'en', ['C', 'de-DE'])).toBe('de-DE');
    expect(formatLocaleFor('', 'en', ['C'])).toBe('en');
    expect(formatLocaleFor('en', 'en', ['de-DE', 'en_GB'])).toBe('en-GB');
  });

  // Intl throws a RangeError for a tag it rejects; one bad system tag must
  // not break every date and number on screen.
  it.each(['de_CH', 'en_US.UTF-8', 'C', 'POSIX', '', 'x', '!!', 'zz-ZZ'])(
    'keeps formatting working for a system tag of %j',
    (tag) => {
      applyLanguage('', [tag]);
      expect(() => formatNumber(1284500.5)).not.toThrow();
      expect(() => formatDate(oct23)).not.toThrow();
      expect(() => formatTime(oct23)).not.toThrow();
      expect(() => render('{n} items', { n: 1284500 }, i18n.locale, i18n.formatLocale)).not.toThrow();
    },
  );

  it('reads the web view languages, most preferred first, without blanks', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['de-CH', '', 'en-US']);
    expect(systemLanguages()).toEqual(['de-CH', 'en-US']);
  });

  it('falls back to navigator.language when the language list is empty', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue([]);
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('ko-KR');
    expect(systemLanguages()).toEqual(['ko-KR']);
  });

  // main.ts calls applyLanguage() with no arguments before the first frame.
  it('follows the system at startup', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['ja-JP', 'en-US']);
    expect(applyLanguage()).toBe('en');
    expect(document.documentElement.lang).toBe('en');
    expect(i18n.formatLocale).toBe('ja-JP');
    expect(formatDate(oct23, { timeZone: 'UTC' })).toBe('2026/10/23');
  });
});

// ADR-008 test plan: formats follow the format locale. Short numeric
// dates and grouped numbers are stable across ICU versions, with two
// exceptions the checks allow for: newer ICU data groups de-CH with ' rather
// than ’, and puts U+202F rather than a space before PM.
describe('formats per locale', () => {
  it.each([
    ['en-US', '1,284,500.5', '10/23/2026'],
    ['en-GB', '1,284,500.5', '23/10/2026'],
    ['de-DE', '1.284.500,5', '23.10.2026'],
    ['de-CH', /^1['’]284['’]500\.5$/u, '23.10.2026'],
    ['fr-FR', '1\u202f284\u202f500,5', '23/10/2026'],
    ['ja-JP', '1,284,500.5', '2026/10/23'],
  ])('%s formats numbers as %s and dates as %s', (tag, number, date) => {
    applyLanguage('', [tag]);
    const formatted = formatNumber(1284500.5);
    if (typeof number === 'string') expect(formatted).toBe(number);
    else expect(formatted).toMatch(number);
    expect(formatDate(oct23, { timeZone: 'UTC' })).toBe(date);
  });

  it('formats times in the format locale', () => {
    applyLanguage('', ['en-GB']);
    expect(formatTime(oct23, { timeZone: 'UTC', hour: '2-digit', minute: '2-digit' })).toBe('15:04');
    applyLanguage('', ['en-US']);
    expect(formatTime(oct23, { timeZone: 'UTC', hour: '2-digit', minute: '2-digit' })).toMatch(/^03:04\sPM$/u);
  });
});
