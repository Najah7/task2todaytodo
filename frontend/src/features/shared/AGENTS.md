# Shared frontend guide

Follow the [feature guide](../AGENTS.md). This directory has no route. It holds presentation and interaction code reused by multiple features; feature-specific rules stay with their owners.

## File map

### Calendar and hooks

- [calendar/dateTime.ts](calendar/dateTime.ts): converts local dates and times to instants and back, including daylight-saving edge cases, so forms agree on time semantics.
- [calendar/dateTime.test.ts](calendar/dateTime.test.ts): checks those conversions and invalid local times.
- [calendar/zonedTime.ts](calendar/zonedTime.ts): formats clock time in a chosen time zone for consistent display.
- [hooks/useFormDraftGuard.ts](hooks/useFormDraftGuard.ts): protects unsaved form edits during navigation or page unload and exposes confirmation state.
- [hooks/useModalFocusTrap.ts](hooks/useModalFocusTrap.ts): keeps keyboard focus inside a dialog and restores it on close.
- [hooks/useModalFocusTrap.test.tsx](hooks/useModalFocusTrap.test.tsx): checks dialog focus and keyboard behavior.

### Notifications and layout

- [notification/index.ts](notification/index.ts): common success, error, warning, and action toast API for feature code.
- [notification/NotificationViewport/index.tsx](notification/NotificationViewport/index.tsx): renders toasts with the selected display mode and localized labels.
- [notification/NotificationViewport/index.module.css](notification/NotificationViewport/index.module.css): viewport appearance.
- [notification/NotificationViewport/NotificationViewport.stories.tsx](notification/NotificationViewport/NotificationViewport.stories.tsx): toast appearance examples.
- [layout/Header/index.tsx](layout/Header/index.tsx): app brand and display/language controls shared by routed pages.
- [layout/Header/index.module.css](layout/Header/index.module.css): header layout.
- [layout/SideMenu/index.tsx](layout/SideMenu/index.tsx): grouped navigation and inbox count.
- [layout/SideMenu/index.module.css](layout/SideMenu/index.module.css): side menu layout.
- [layout/SideMenu/parts/SideMenuItem/index.tsx](layout/SideMenu/parts/SideMenuItem/index.tsx): route link, active state, and badge rendering.
- [layout/SideMenu/parts/SideMenuItem/index.module.css](layout/SideMenu/parts/SideMenuItem/index.module.css): navigation-item appearance.
- [layout/SideMenu/parts/MenuIcon/index.tsx](layout/SideMenu/parts/MenuIcon/index.tsx): icon selected for each navigation tab.

### Reusable components

- [components/ActionItemActualMinutesDialog/index.tsx](components/ActionItemActualMinutesDialog/index.tsx): reusable completion dialog for entering actual work minutes.
- [components/ActionItemActualMinutesDialog/schema.ts](components/ActionItemActualMinutesDialog/schema.ts): validation for that minute input.
- [components/ActionItemActualMinutesDialog/index.test.tsx](components/ActionItemActualMinutesDialog/index.test.tsx): checks dialog interactions.
- [components/ActionItemActualMinutesDialog/index.module.css](components/ActionItemActualMinutesDialog/index.module.css): dialog appearance.
- [components/ConfirmationDialog/index.tsx](components/ConfirmationDialog/index.tsx): generic confirm/cancel dialog for consequential actions.
- [components/ConfirmationDialog/index.module.css](components/ConfirmationDialog/index.module.css): confirmation appearance.
- [components/ConfirmationDialog/ConfirmationDialog.stories.tsx](components/ConfirmationDialog/ConfirmationDialog.stories.tsx): confirmation states.
- [components/PageHeading/index.tsx](components/PageHeading/index.tsx): consistent page title and description.
- [components/PageHeading/index.module.css](components/PageHeading/index.module.css): heading appearance.
- [components/PageHeading/PageHeading.stories.tsx](components/PageHeading/PageHeading.stories.tsx): heading examples.
- [components/SeachSelect/index.tsx](components/SeachSelect/index.tsx): searchable select with keyboard and composition-input support; retain the existing directory spelling.
- [components/SeachSelect/index.module.css](components/SeachSelect/index.module.css): select appearance.
- [components/SeachSelect/SeachSelect.test.tsx](components/SeachSelect/SeachSelect.test.tsx): checks selection and input interactions.
- [components/SeachSelect/SeachSelect.stories.tsx](components/SeachSelect/SeachSelect.stories.tsx): select examples.
- [components/Switcher/index.tsx](components/Switcher/index.tsx): accessible two-choice control used for display and language.
- [components/Switcher/index.module.css](components/Switcher/index.module.css): switch appearance.
- [components/Switcher/Switcher.stories.tsx](components/Switcher/Switcher.stories.tsx): switch examples.
- [components/Wordmark/index.tsx](components/Wordmark/index.tsx): theme-aware brand wordmark shared by app screens.
- [components/Wordmark/index.module.css](components/Wordmark/index.module.css): wordmark appearance.
- [components/Wordmark/Wordmark.stories.tsx](components/Wordmark/Wordmark.stories.tsx): wordmark examples.

## Change checks

- Keep reusable components driven by props; put domain decisions in the consuming feature.
- Check all consumers when changing a shared contract. For calendar changes, verify time-zone and daylight-saving behavior; for dialogs, preserve keyboard focus and Escape handling.
- Keep navigation routes, translated labels, and menu icons aligned with [router.tsx](../../router.tsx) and [i18n](../i18n/AGENTS.md).

## Verification

- Run the colocated tests for changed helpers or components and inspect relevant Storybook states for visual changes.
