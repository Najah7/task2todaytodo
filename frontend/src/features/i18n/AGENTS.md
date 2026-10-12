# i18n feature guide

Follow the [feature guide](../AGENTS.md). Translation is shared across Pages; it has no dedicated route.

## Features and pages

- Supply Japanese and English system text across the app. The language control appears in the shared Header and on `/login` and `/signup`.
- A valid `?lang=ja|en` link takes precedence, then a saved choice; Japanese is the default. Selection is persisted and sets the document's `lang` attribute.

## Key files

- [index.tsx](index.tsx): `LanguageProvider`, initial selection, persistence, and document language.
- [languageContext.ts](languageContext.ts), [hooks.ts](hooks.ts): typed language state and `useI18n` message interpolation.
- [messages/index.ts](messages/index.ts), [messages/types.ts](messages/types.ts): Japanese/English catalogs and keys derived from the Japanese catalog.
- [LanguageSwitcher/index.tsx](LanguageSwitcher/index.tsx): visible language control using the shared Switcher.

## Change checks

- Add or change a system message in both catalogs with the same key and placeholders; use typed `MessageKey` and `useI18n` at call sites.
- Do not translate user-created content. Preserve URL-language precedence, the Japanese fallback, and `document.documentElement.lang`.
- Text changes may alter widths in [shared layout](../shared/AGENTS.md) and feature screens; inspect both languages where layout matters.

## Verification

- [Language E2E](../../../e2e/switch-language.spec.ts) covers switching and persistence.
