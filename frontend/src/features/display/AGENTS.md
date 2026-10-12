# Display feature guide

Follow the [feature guide](../AGENTS.md). Display preference is shared across Pages; it has no dedicated route.

## Features and pages

- Switch between light and dark display. The control appears in the shared Header and on `/login` and `/signup`.
- Startup uses a saved choice when available, otherwise the OS color preference. A change updates the root `data-display`, Jotai state, and browser storage.

## Key files

- [index.ts](index.ts): initialization and `useDisplay` keep the active mode and persistence together.
- [DisplaySwitcher/index.tsx](DisplaySwitcher/index.tsx): user control, built on the shared Switcher.
- [store/mode.ts](../../store/mode.ts): shared mode atom; [styles](../../styles/) consume `data-display` for theme values.

## Change checks

- Initialize the display before React renders in [main.tsx](../../main.tsx) to avoid a mismatched first theme.
- Keep storage-unavailable fallback and the OS-preference default; update the root attribute and atom together.
- Keep the switcher's text in [i18n](../i18n/AGENTS.md) and its behavior accessible via the shared Switcher.

## Verification

- [Display E2E](../../../e2e/switch-display.spec.ts) covers switching and persistence.
