// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// A message is either plain text or one text per plural category, chosen by
// Intl.PluralRules for the {count} parameter. "other" is always required:
// it is the only category Japanese and Korean use.
export type PluralMessage = Partial<Record<Intl.LDMLPluralRule, string>> & { other: string };
export type Message = string | PluralMessage;
export type Catalog = Record<string, Message>;

export type MessageParams = Record<string, string | number>;

const PLACEHOLDER = /\{(\w+)\}/g;

// placeholders returns the {names} a text uses.
export function placeholders(text: string): Set<string> {
  return new Set(Array.from(text.matchAll(PLACEHOLDER), (m) => m[1]));
}

// render picks the plural form, if any, and fills its placeholders. A number
// is formatted for the format locale; a placeholder with no value stays as
// written, so a missing parameter is visible rather than silently blank.
export function render(message: Message, params: MessageParams | undefined, locale: string, formatLocale: string): string {
  let text: string;
  if (typeof message === 'string') {
    text = message;
  } else {
    const category = new Intl.PluralRules(locale).select(Number(params?.count ?? 0));
    text = message[category] ?? message.other;
  }
  if (!params) return text;
  return text.replace(PLACEHOLDER, (whole, name: string) => {
    const value = params[name];
    if (value === undefined) return whole;
    return typeof value === 'number' ? new Intl.NumberFormat(formatLocale).format(value) : value;
  });
}

export interface CatalogProblem {
  key: string;
  problem: string;
}

function forms(message: Message): [string, string][] {
  return typeof message === 'string' ? [['text', message]] : Object.entries(message) as [string, string][];
}

// checkCatalog compares a translation with the English source. A language
// ships only when this returns no problems (ADR-008, decision 7).
export function checkCatalog(source: Catalog, target: Catalog, locale: string): CatalogProblem[] {
  const problems: CatalogProblem[] = [];
  const categories = new Intl.PluralRules(locale).resolvedOptions().pluralCategories;

  for (const [key, original] of Object.entries(source)) {
    const translated = target[key];
    if (translated === undefined) {
      problems.push({ key, problem: 'missing' });
      continue;
    }
    if (typeof original !== typeof translated) {
      problems.push({ key, problem: typeof original === 'string' ? 'should be plain text' : 'should have plural forms' });
      continue;
    }
    const expected = placeholders(typeof original === 'string' ? original : original.other);
    if (typeof translated !== 'string') {
      for (const category of categories) {
        if (!translated[category]) problems.push({ key, problem: `missing plural form "${category}"` });
      }
    }
    for (const [form, text] of forms(translated)) {
      if (text.trim() === '') problems.push({ key, problem: `empty ${form}` });
      const found = placeholders(text);
      const unknown = [...found].filter((name) => !expected.has(name));
      if (unknown.length) problems.push({ key, problem: `unknown placeholder {${unknown.join('}, {')}} in ${form}` });
      const required = form === 'text' || form === 'other' ? [...expected].filter((name) => !found.has(name)) : [];
      if (required.length) problems.push({ key, problem: `lost placeholder {${required.join('}, {')}} in ${form}` });
    }
  }
  for (const key of Object.keys(target)) {
    if (!(key in source)) problems.push({ key, problem: 'not in the English catalog' });
  }
  return problems;
}
