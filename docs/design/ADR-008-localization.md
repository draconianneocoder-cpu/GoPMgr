<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-008: Interface, Help, and document languages

**Status:** Proposed
**Date:** 2026-10-23
**Deciders:** GoPMgr maintainer

## Context

GoPMgr is English only. There is no language setting, `index.html` fixes
`<html lang="en">`, and every string is written into the code:

| Surface | English text today |
| --- | --- |
| App interface (102 Svelte components) | About 1,550 strings in markup, plus status messages set in code |
| Backend messages shown in the app | About 450 errors |
| PDF, DOCX, and ODT documents (25 kinds) | About 1,300 strings |
| In-app Help | About 10,000 words |
| User guide (`docs/user-guide.md`) | About 5,600 words; not shipped or linked from the app |

Formats are mixed. The app formats 38 dates and numbers with the web view's
locale, so they follow the operating system while the words around them stay
English. The PDF renderers write every amount as `1,284,500.00`.

New projects can use six reporting currencies: USD, EUR, GBP, CAD, AUD, and
CHF. JPY stays readable for legacy projects only.

The maintainer asked whether users can choose their language during setup and
read Help and the user guide in it, and suggested limiting languages to those
of countries whose currencies GoPMgr supports.

## The currency rule, checked

| Currency | Where it is the currency | Official languages |
| --- | --- | --- |
| EUR | 21 euro-area countries, plus Andorra, Monaco, San Marino, Vatican City, Montenegro, Kosovo | Bulgarian, Croatian, Dutch, English, Estonian, Finnish, French, German, Greek, Irish, Italian, Latvian, Lithuanian, Luxembourgish, Maltese, Portuguese, Slovak, Slovene, Spanish, Swedish, Turkish (Cyprus); also Catalan, Albanian, Serbian |
| CHF | Switzerland, Liechtenstein | German, French, Italian, Romansh |
| CAD | Canada | English, French |
| GBP | United Kingdom | English, Welsh, Scottish Gaelic, Irish |
| AUD | Australia, Kiribati, Nauru, Tuvalu | English, plus Pacific languages |
| USD | United States, Ecuador, El Salvador, Panama, Timor-Leste, others | English, Spanish, Portuguese, plus others |

What this shows:

1. **The rule barely limits.** The euro area alone yields 21 official
   languages, and the six currencies together more than 25.
2. **Language and currency belong to different things.** A currency belongs to
   a project and locks once the project has budget data. A language belongs to
   a person. A Swiss team works in CHF in German, French, or Italian; a Japanese
   team can run a USD project. Tying the two would hide a language from someone
   whose project uses another currency.
3. **It works well as a technical filter.** Every language it yields is
   written in Latin, Greek, or Cyrillic script, and none needs shaping or
   right-to-left layout, so the PDF engine prints them correctly. Checked
   through the character map the PDF engine reads, IBM Plex Sans and Serif
   have every letter of 23 of these languages, plus the spaces and signs
   their number formats use: no-break space, narrow no-break space (French),
   ’ (Swiss German), the minus sign, €, and £. The rule leaves out exactly the
   languages with PDF limits (Arabic, Hebrew, Hindi, Thai) or no new-project
   currency (Japanese, Chinese, Korean).

So use the currency markets to choose and order which languages to
translate, and keep language a setting of its own.

## Decision

1. **Interface language is a per-user setting.** The first setup screen has a
   language picker, and App Settings has the same control. The default is the
   first operating-system language GoPMgr ships, else English. The choice sets
   `<html lang>`, every interface string, and date and number formats.
2. **Formats follow the interface language.** Amounts always show the ISO
   code, never only a symbol, because `$` is USD, CAD, and AUD.
3. **Document language is a per-project setting**, defaulting to the
   creator's interface language. Documents are for the project's stakeholders,
   who may not share the user's language. Renderers take the document language
   for headings, labels, and number formats, as they now take the currency.
4. **Help and the user guide are written per language.** A page not yet
   translated shows in English with a notice. The user guide ships with the
   app, as Help pages or a packaged file, so users have it at all.
5. **Languages arrive in waves.** Wave 1 is English (the source), French,
   German, Spanish, Italian, Portuguese (Portugal), and Dutch: the main
   languages of all six currencies. Later waves come from the euro-area list,
   by demand. A language ships only when all its interface strings are
   translated and reviewed by a native speaker; its Help may lag behind,
   with the fallback notice.
6. **No new runtime dependency.** Typed message catalogs in the frontend, the
   web view's `Intl` APIs for plurals, numbers, dates, and currency names, and
   a small embedded catalog in Go for documents and backend messages.

## Design

```
First setup screen ──► ui_language (user's app settings) ──► <html lang>, t(key), Intl formats
App Settings ─────────┘
Project Settings ────► document_language (project table) ──► Go renderers ──► PDF, DOCX, ODT
Backend error ───────► stable code + English text ──────────► app shows t(code), else the text
Help, user guide ────► one content file per language ───────► English fallback with a notice
```

- **Storage.** `AppSettings.ui_language` (`"ui_language"`, empty means
  automatic). Setup screens run before any account exists, so they use the
  language last chosen on this computer, kept in `system.db`, and account
  creation copies it into the new user's settings.
  `project.document_language TEXT NOT NULL DEFAULT 'en'`, added by migration.
- **Frontend catalogs.** `frontend/src/lib/i18n/` holds `en.ts` as the typed
  source and one file per language. `t('settings.font.apply', { name })` is
  checked against the English keys at compile time. Plurals use
  `Intl.PluralRules` categories. `terminology.ts`'s methodology words become
  catalog entries.
- **Go catalogs.** A `internal/locale` package embeds one JSON catalog per
  language for document strings and backend message codes, with English
  fallback. Number formatting moves from `formatMoney`'s fixed separators to
  per-locale separators.
- **Backend messages.** Errors shown to users carry a stable code and
  arguments. The app translates the code and falls back to the English text.
  The 50 most frequent messages go first.
- **Help.** Help prose moves from Svelte markup to one structured content file
  per language (headings, paragraphs, lists, inline code) rendered by one
  component. This needs no Markdown dependency and gives translators plain
  text.

## Options considered

### Option A: in-house catalogs and `Intl` (chosen)

| Dimension | Assessment |
| --- | --- |
| Complexity | Medium: a small catalog loader, `t()`, and format helpers; most of the work is extracting strings |
| Dependencies | None added; the frontend keeps its two runtime dependencies |
| Type safety | Keys typed from `en.ts` |
| Fit | Matches the repository's dependency policy |

### Option B: Paraglide JS

Compile-time messages with tree-shaking and typed keys, but adds a compiler
step, a CLI, and a dependency to a two-dependency frontend.

### Option C: svelte-i18n

Runtime ICU messages, but adds `intl-messageformat` and a store API from
before Svelte 5's runes.

### Option D: stay English, formats from the system

No translation cost, but the setup question stays "no", and dates keep
following the system while words do not.

### Option E: languages tied to project currency

Rejected for the reasons in "The currency rule, checked".

## Consequences

- Easier: adding a language means adding files, and each surface already has
  one source of truth.
- Harder: every new string needs a catalog key, and each release needs
  translation and review time.
- Translation volume per language is about 3,300 strings (interface, backend
  messages, documents) and about 15,500 words of Help and user guide.
- German and French text often runs up to 35 percent longer than English. The app uses
  the `uppercase` class 658 times; CSS uppercasing follows `<html lang>`, so
  `lang` must change with the language (Turkish dotted İ, German ß).
- IBM Plex Mono has no Greek; Greek text in code-style spans falls back to a
  system font.

Revisit when a requested language needs shaping or right-to-left layout:
that needs a PDF text-shaping engine first (see ADR-007).

## Test plan

| Area | Test | Type |
| --- | --- | --- |
| Catalog completeness | Every English key exists in each shipped language, with no extra keys | unit, per language |
| Placeholders | Each translation has the same `{name}` placeholders as English | unit |
| Plurals | Each plural message covers `Intl.PluralRules(lang).resolvedOptions().pluralCategories` | unit |
| Hard-coded text | A check counts literal text in `.svelte` files; the count may only fall | ratchet script |
| Layout | Pseudo-language (accented, 35 percent longer) on key screens in Chromium fails on clipped buttons or labels | browser smoke |
| Formats | `fr-FR` uses U+202F grouping, `de-CH` uses ’, `de-DE` uses `.` and `,`; dates per locale | table-driven |
| Setup | The picker defaults from the system, persists to the new account, and sets `<html lang>`; keyboard-only operation works | component |
| Documents | Each kind renders in each document language with every glyph mapped; one PDF/A-2u gate sample per wave-1 language | Go unit, veraPDF |
| Help | Each section exists per language or shows the fallback notice; the help-guide check runs per language | script |
| Backend messages | Every code has English text; an unknown code shows the English text | Go unit, component |

Targets: 100 percent catalog completeness to ship a language; no fall in the
hard-coded-text ratchet; every wave-1 language in the PDF/A gate.

## Language picker copy

- **Label:** "Language".
- **Options:** each language in its own name, with `lang` set: English,
  Français, Deutsch, Español, Italiano, Português, Nederlands. No flags:
  flags are countries, and Swiss, Belgian, and Canadian users would not find
  theirs.
- **Setup helper:** "You can change this later in Settings."
- **App Settings helper:** "Changes the language of menus, messages, and Help.
  Each project's documents use its document language."
- **Project Settings, "Document language":** "The language of this project's
  PDFs and exports, for the people who read them."
- **Help fallback notice:** "This page isn't available in {language} yet, so
  it's shown in English."
- **Translator notes:** Full sentences with placeholders, never joined
  fragments. Keep project-management terms consistent with a glossary
  (charter, baseline, earned value). Allow 35 percent more length.

## Setup and Settings critique

| Finding | Severity | Recommendation |
| --- | --- | --- |
| No language control anywhere | High | Add a compact picker at the top of the first setup card, not an extra step |
| Dates and times follow the system while words are English | Medium | Pass the interface locale to every `toLocale*String` call |
| Currency names are hard-coded English ("US dollar (USD)") | Low | Use `Intl.DisplayNames` with the interface language |
| `<html lang="en">` is fixed | Medium | Set it from the interface language, for screen readers, hyphenation, and CSS case rules |
| Uppercase labels widen German text | Low | Check key screens in the pseudo-language |

The setup card itself is calm and clear: one task per step and a visible
recovery-code step. A picker fits in its header without changing that.

## Action items

1. [x] Label document amounts in the project's currency, the currency half
   of this work (`fix: label document amounts in the project's reporting
   currency`).
2. [ ] Maintainer: confirm independent language and currency, the wave-1
   list, and the translation source (professional or native-speaker review).
3. [ ] Phase 1: `ui_language` setting, `<html lang>`, locale-aware formats in
   app and documents, catalog code, and English extraction, screen by screen
   behind the hard-coded-text ratchet.
4. [ ] Phase 2: wave-1 interface translations, the picker, document
   language, backend message codes.
5. [ ] Phase 3: Help content files and the user guide in the app, per
   language with fallback; gates per language.
6. [ ] Phase 4: further euro-area languages by demand.
