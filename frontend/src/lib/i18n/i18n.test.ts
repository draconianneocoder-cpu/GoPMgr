// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { afterEach, describe, expect, it } from 'vitest';

import { applyLanguage, formatDate, formatNumber, i18n, offeredLocales, t } from './i18n.svelte';
import { formatLocaleFor, matchLocale, resolveLocale, type LocaleCode } from './locales';

const wave1: LocaleCode[] = ['en', 'fr', 'de', 'es', 'it', 'pt-PT', 'nl', 'ja', 'ko'];

afterEach(() => {
  applyLanguage('', ['en-US']);
});

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
