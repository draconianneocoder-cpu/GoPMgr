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

// One piece of a rendered message: literal text, or the value of the
// placeholder named by param.
export interface MessagePart {
  text: string;
  param?: string;
}

// pick returns the text for the plural category of params.count, if the
// message has plural forms.
function pick(message: Message, params: MessageParams | undefined, locale: string): string {
  if (typeof message === 'string') return message;
  const category = new Intl.PluralRules(locale).select(Number(params?.count ?? 0));
  return message[category] ?? message.other;
}

// renderParts picks the plural form, if any, and splits the text at each
// filled placeholder. A component can then style one value, such as a
// person's name, wherever a translation places it, without joining
// sentence fragments or rendering HTML. A number is formatted for the
// format locale; a placeholder with no value stays as written, so a missing
// parameter is visible rather than silently blank.
export function renderParts(
  message: Message,
  params: MessageParams | undefined,
  locale: string,
  formatLocale: string,
): MessagePart[] {
  const text = pick(message, params, locale);
  const parts: MessagePart[] = [];
  let literal = '';
  let last = 0;
  for (const match of text.matchAll(PLACEHOLDER)) {
    const name = match[1];
    const value = params?.[name];
    if (value === undefined) continue;
    literal += text.slice(last, match.index);
    if (literal) parts.push({ text: literal });
    literal = '';
    parts.push({ text: typeof value === 'number' ? new Intl.NumberFormat(formatLocale).format(value) : value, param: name });
    last = match.index + match[0].length;
  }
  literal += text.slice(last);
  if (literal) parts.push({ text: literal });
  return parts;
}

// render is renderParts joined into one string.
export function render(message: Message, params: MessageParams | undefined, locale: string, formatLocale: string): string {
  return renderParts(message, params, locale, formatLocale)
    .map((part) => part.text)
    .join('');
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
