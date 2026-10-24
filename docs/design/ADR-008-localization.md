<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-008: Interface, Help, and document languages

**Status:** Accepted
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
   ’ (Swiss German), the minus sign, €, and £.

## Decision

The maintainer decided:

- Language and currency stay separate settings.
- Wave 1 is the six currencies' main languages plus Japanese and Korean.
- Translations are machine-made and checked automatically before they ship.
  Native speakers then improve them through reports.

1. **Interface language is a per-user setting.** The first setup screen has a
   language picker, and App Settings has the same control. The default is the
   first operating-system language GoPMgr ships, else English. The choice sets
   `<html lang>`, every interface string, and date and number formats.
2. **Formats follow the interface language.**
   - With a chosen language, formats keep the system's region when the
     system speaks that language, so English in Britain formats as en-GB.
   - In automatic mode, formats follow the system locale, as they always
     have. So a German system shows German dates even while the text is
     English.
   - Amounts always show the ISO code, never only a symbol, because `$` is
     USD, CAD, and AUD.
3. **Document language is a per-project setting**, defaulting to the
   creator's interface language. Documents are for the project's stakeholders,
   who may not share the user's language. Renderers take the document language
   for headings, labels, and number formats, as they now take the currency.
   Japanese and Korean documents default to IBM Plex Sans JP and KR.
4. **Help and the user guide are written per language.** A page not yet
   translated shows in English with a notice. The user guide ships with the
   app, as Help pages or a packaged file, so users have it at all.
5. **Wave 1 languages.**
   - English is the source.
   - French, German, Spanish, Italian, Portuguese (Portugal), and Dutch are
     the main languages of the six currencies.
   - Japanese and Korean are the maintainer's additions.

   Later waves come from the euro-area list, by demand. A language ships when
   every interface string is translated and passes the checks in decision 7.
   Its Help may lag behind, with the fallback notice.
6. **No new runtime dependency.** The frontend uses typed message catalogs
   and the web view's `Intl` APIs for plurals, numbers, dates, and currency
   names. Go embeds a small catalog for documents and backend messages.
7. **Translation workflow.**
   1. **Translate.** A machine translation produces each catalog, guided by a
      per-language glossary and style guide.
   2. **Check automatically.** A language ships only when all of these pass:
      - Completeness, placeholder parity, and plural categories.
      - Glossary terms.
      - Layout with the real translations.
      - A back-translation check on critical strings. These are encryption,
        recovery codes, signing, deletion, and anything that cannot be undone.
        A critical string whose back-translation loses its meaning blocks
        the release.
   3. **Ship, labelled.** The Language setting says other languages are
      machine translated and tells users how to report a wrong phrase.
   4. **Improve.** Native speakers report problems through the "Translation
      problem" issue template, and later an in-app link that fills in the
      language and screen. A fix marks the string reviewed, and a new machine
      run never overwrites a reviewed string.

## Design

```
First setup screen ──► ui_language (user's app settings) ──► <html lang>, t(key), Intl formats
App Settings ─────────┘
Project Settings ────► document_language (project table) ──► Go renderers ──► PDF, DOCX, ODT
Backend error ───────► stable code + English text ──────────► app shows t(code), else the text
Help, user guide ────► one content file per language ───────► English fallback with a notice
Translation report ──► issue template ──► fix + mark reviewed ──► protected from machine runs
```

- **Storage.**
  - Each user's choice is kept as `AppSettings.ui_language`
    (`"ui_language"`); empty means automatic.
  - Setup screens run before any account exists, so they use the language
    last chosen on this computer, kept in `system.db`. Account creation copies
    it into the new user's settings.
  - Projects get `project.document_language TEXT NOT NULL DEFAULT 'en'`,
    added by migration.
- **Frontend catalogs.** `frontend/src/lib/i18n/` holds `en.ts` as the typed
  source and one file per language. `t('settings.font.apply', { name })` is
  checked against the English keys at compile time. Plurals use
  `Intl.PluralRules` categories. `terminology.ts`'s methodology words become
  catalog entries. A per-language review list records the keys a native
  speaker has corrected.
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

### Japanese and Korean

- **Fonts in the app.**
  - **The gap:** The app has relied on system fonts for CJK text (ADR-007).
    That fails here in two ways. Linux systems often have no CJK font, and the
    setup picker must show 日本語 and 한국어 before either language is chosen.
  - **The fix:** The app bundles IBM Plex Sans JP and KR as WOFF2 in its four
    weights: about 9.5 MB and 1.7 MB, from IBM's npm packages.
  - **Checks:** Like the other WOFF2 files, they are pinned by SHA-256 and
    checked against the bundled TrueType files' version and glyphs.
  - **Effect on ADR-007:** This replaces ADR-007's "No" in the App column for
    JP and KR. SC and TC stay as ADR-007 records them.
- **Line breaking.**
  - **App:** CSS sets `:lang(ja) { line-break: strict; }` and
    `:lang(ko) { word-break: keep-all; }`.
  - **PDFs:** The PDF engine breaks Japanese lines between any two
    characters, so a line can start with 。 or 、.
  - **Fix:** `pdfdoc` moves closing punctuation back to the previous line.
- **Small text.** The app uses 10–11px text in 330 places. Kanji and Hangul
  need at least 12px, so `:lang(ja)` and `:lang(ko)` raise those sizes.
- **Input.**
  - **The risk:** Enter confirms an input-method conversion, so a keydown
    handler that acts on Enter could fire while someone is typing.
  - **The rule:** Handlers ignore events whose `isComposing` is true.
  - **Today:** No handler checks it, but the current Enter handlers act on
    buttons, a password field, or Ctrl/⌘+Enter, so the risk is low.
- **Currency.**
  - **Yen:** Japanese teams can report in yen ([ADR-009](ADR-009-yen-reporting-currency.md)),
    which reads as whole numbers.
  - **Won:** not offered yet. It has no minor unit either, so it can follow
    ADR-009's pattern once the maintainer adds it.
- **Plurals.** Japanese and Korean have one plural category, so their
  messages need only `other`.
- **Style.** Japanese uses です・ます; Korean uses 합니다체. Each has its own
  glossary.

## Options considered

### Catalogs

| Option | Assessment |
| --- | --- |
| **A. In-house catalogs and `Intl` (chosen)** | Medium effort: a small loader, `t()`, and format helpers; most of the work is extracting strings. No dependency added; keys typed from `en.ts`. |
| B. Paraglide JS | Compile-time messages and typed keys, but adds a compiler step, a CLI, and a dependency to a two-dependency frontend. |
| C. svelte-i18n | Runtime ICU messages, but adds `intl-messageformat` and a store API from before Svelte 5's runes. |
| D. Stay English | No translation cost, but the setup question stays "no". |
| E. Languages tied to project currency | Rejected; see "The currency rule, checked". |

### Translation source

| Option | Assessment |
| --- | --- |
| Professional translation | Highest first quality; slow and paid for every release. |
| Machine translation, native review before shipping | Good quality; each language waits on a volunteer. |
| **Machine translation, automated checks, native-speaker reports (chosen)** | Fastest and free; early wording may read unnaturally. The back-translation check guards the strings where a mistranslation could lose data. |

## Consequences

- Easier: adding a language means adding files, and each surface already has
  one source of truth.
- Harder: every new string needs a catalog key, and each release runs the
  translation checks.
- Translation volume per language is about 3,300 strings (interface, backend
  messages, documents) and about 15,500 words of Help and user guide.
- German and French text often runs up to 35 percent longer than English.
  The app uses the `uppercase` class 658 times. CSS uppercasing follows
  `<html lang>`, so `lang` must change with the language (Turkish dotted İ,
  German ß).
- Japanese and Korean add about 11 MB of app fonts.
- Early translations may read unnaturally until reports refine them.
- IBM Plex Mono has no Greek; Greek text in code-style spans falls back to a
  system font.

Revisit when a requested language needs shaping or right-to-left layout:
that needs a PDF text-shaping engine first (see ADR-007). Revisit won
alongside a Korean release.

## Test plan

| Area | Test | Type |
| --- | --- | --- |
| Catalog completeness | Every English key exists in each shipped language, with no extra keys | unit, per language |
| Placeholders | Each translation has the same `{name}` placeholders as English | unit |
| Plurals | Each plural message covers `Intl.PluralRules(lang).resolvedOptions().pluralCategories` | unit |
| Critical strings | Back-translations of critical strings keep their meaning | review script, release gate |
| Reviewed strings | A new machine run leaves reviewed keys unchanged | unit |
| Hard-coded text | A check counts literal text in `.svelte` files; the count may only fall | ratchet script |
| Layout | Pseudo-language (accented, 35 percent longer) and each real catalog on key screens in Chromium; clipped buttons or labels fail | browser smoke |
| CJK text | Under `lang="ja"` and `lang="ko"`, no text is smaller than 12px, and 日本語 and 한국어 render in IBM Plex | browser smoke |
| Input methods | A keydown handler ignores Enter while `isComposing` is true | component |
| Formats | `fr-FR` uses U+202F grouping, `de-CH` an apostrophe (’ or ', by ICU version), `de-DE` uses `.` and `,`, `ja-JP` dates read 2026/10/23; dates per locale | table-driven |
| Setup | The picker defaults from the system, persists to the new account, and sets `<html lang>`; keyboard-only operation works | component |
| Documents | Each kind renders in each document language with every glyph mapped; Japanese lines never start with closing punctuation; one PDF/A-2u gate sample per wave-1 language | Go unit, veraPDF |
| Help | Each section exists per language or shows the fallback notice; the help-guide check runs per language | script |
| Backend messages | Every code has English text; an unknown code shows the English text | Go unit, component |

Targets: 100 percent catalog completeness and every check passing to ship a
language; no fall in the hard-coded-text ratchet; every wave-1 language in
the PDF/A gate.

## Language picker copy

- **Label:** "Language".
- **Options:** each language in its own name, with `lang` set: English,
  Français, Deutsch, Español, Italiano, Português, Nederlands, 日本語, 한국어.
  No flags: flags are countries, and Swiss, Belgian, and Canadian users would
  not find theirs.
- **Setup helper:** "You can change this later in Settings."
- **App Settings helper:** "Changes the language of menus, messages, and Help.
  Languages other than English are machine translated. If a phrase reads
  wrong, report it from Help."
- **Project Settings, "Document language":** "The language of this project's
  PDFs and exports, for the people who read them."
- **Help fallback notice:** "This page isn't available in {language} yet, so
  it's shown in English."
- **Report link (Help):** "Report a translation problem".
- **Translator notes:** Full sentences with placeholders, never joined
  fragments. Keep project-management terms consistent with a glossary
  (charter, baseline, earned value). Allow 35 percent more length.

## Setup and Settings critique

| Finding | Severity | Recommendation |
| --- | --- | --- |
| No language control anywhere | High | Add a compact picker at the top of the first setup card, not an extra step |
| CJK text depends on system fonts that Linux often lacks | High | Bundle IBM Plex Sans JP and KR WOFF2 before shipping Japanese or Korean |
| Dates and times follow the system while words are English | Medium | Pass the interface locale to every `toLocale*String` call |
| `<html lang="en">` is fixed | Medium | Set it from the interface language, for screen readers, hyphenation, CSS case rules, and CJK glyph forms |
| 10–11px labels are too small for kanji and Hangul | Medium | Raise them to 12px under `:lang(ja)` and `:lang(ko)` |
| Currency names are hard-coded English ("US dollar (USD)") | Low | Use `Intl.DisplayNames` with the interface language |
| Uppercase labels widen German text | Low | Check key screens in the pseudo-language |

The setup card itself is calm and clear: one task per step and a visible
recovery-code step. A picker fits in its header without changing that.

## Action items

1. [x] Label document amounts in the project's currency, the currency half
   of this work (`fix: label document amounts in the project's reporting
   currency`).
2. [x] Maintainer decisions: separate language and currency; wave 1 plus
   Japanese and Korean; machine translation with automated checks and
   native-speaker reports.
3. [x] "Translation problem" issue template.
4. [ ] Phase 1: build the localization foundation.
   - The `ui_language` setting and `<html lang>`.
   - Locale-aware formats in the app and documents.
   - The catalog code and its checks.
   - English extraction, screen by screen, behind the hard-coded-text ratchet.

   Slice 1 is done: the catalog code and its checks, `t()`, the format
   helpers, `<html lang>`, the hard-coded-text ratchet, and the sign-in and
   account-creation screens.
   Slice 2 is done: the rest of the account screens (recovery codes,
   password change and reset, and the administrator-access notices), plus
   `tParts`, which keeps a full sentence in the catalog while a component
   styles one placeholder's value, such as a bold name.
5. [ ] Phase 2: ship the wave 1 languages.
   - Interface translations for wave 1, plus the picker.
   - IBM Plex Sans JP and KR WOFF2, with the CJK text size and line-breaking
     rules.
   - Document language, and backend message codes.
6. [ ] Phase 3: Help and the user guide.
   - Help content files, and the user guide inside the app.
   - Both per language, with English fallback, and the help-guide check
     running per language.
   - An in-app report link.
7. [ ] Phase 4: further euro-area languages by demand.
