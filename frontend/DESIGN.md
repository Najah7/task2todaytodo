---
version: alpha
name: Task2TodayTodo
description: A personal task manager that breaks large tasks into subtasks and schedules them into today. Light and dark themes share layout and typography, with theme-specific surfaces, action colors, and depth treatment defined in CSS.
style_source: src/styles/
---

## Source of Truth

The global CSS in `src/styles/` is the single source of truth for UI design values. This document explains their purpose, usage, and screen composition. Read the current values from CSS; do not duplicate palettes, typography scales, dimensions, or shadows here.

| Concern | Authoritative definition |
| --- | --- |
| Palette and foreground/background pairs | [color.css](src/styles/color.css) |
| Spacing scale and shared padding | [spacing.css](src/styles/spacing.css) |
| Corner radii | [radius.css](src/styles/radius.css) |
| Shared dimensions and proportions | [sizes.css](src/styles/sizes.css) |
| Borders, focus, and shadows | [effects.css](src/styles/effects.css) |
| Font tokens and global typography classes | [typography.css](src/styles/typography.css) |

[index.css](src/index.css) loads these definitions. Use their custom properties from component CSS Modules and their global `text-*` classes from markup. Component-specific layout rules and responsive breakpoints live in the relevant CSS Module, such as [SideMenu](src/features/shared/layout/SideMenu/index.module.css).

To change a design value, edit its CSS definition. Update this document when the intent, role assignment, or screen composition changes. CSS takes precedence if a value in an older document or reference image differs. Dates, counts, and times below are content examples; brand artwork geometry belongs to the asset specification.

## Overview

Task2TodayTodo turns big tasks into today's to-dos. The user collects tasks, splits them into subtasks, and moves the right subtasks into **Today**. Every screen exists to make that one move — *task → today* — fast and trustworthy.

The visual language is a deliberate crossover: **a to-do list × freee** (the Japanese cloud accounting app). We borrow freee's way of handling business data, not its brand:

- A calm **warm-gray ground** with **white cards** in light mode, and **warm-charcoal ground** with **slightly lighter cards** in dark mode.
- **Black actions** in light mode and **near-white actions** in dark mode. Orange appears only when something needs attention or marks "now". Blue is reserved for the agreed pale overdue-days label on Projects; use its semantic theme tokens and do not use blue for other roles.
- **Business-grade tables**: column headers on a tinted row and right-aligned tabular numbers. Add a **totals row** (合計) only where the screen specification calls for one; Projects explicitly has none.
- **Suggest, then confirm.** Like freee's automatic bookkeeping, the app pre-fills a guess (project, date) and the user approves it with a per-row **登録 (Register)** button, or approves many at once with **まとめて登録 (Register selected)**.
- **Plan vs. actual (予実)** thinking in reporting: planned time, actual time, variance, completion rate.

The UI supports Japanese and English. System-provided interface text follows the selected language; Japanese navigation uses 今日、タスク受信箱、プロジェクト、カレンダー、KPIダッシュボード、プロフィール, while English navigation uses Today, Task Inbox, Projects, Calendar, KPI Dashboard, Profile. User-created task and project text stays as entered. Body text uses the global `text-body` style, with the density of Japanese business software.

The tone is polite and helpful, never chatty. It uses short です・ます sentences that tell the user what will happen ("登録すると、今日のタスクに追加されます" = registering adds it to today's tasks).

## Colors

Use [color.css](src/styles/color.css) for all color values and pairs. Its `:root` definitions provide light mode and `:root[data-display="dark"]` provides dark mode. Both modes use the same variable names and typography. Neutrals carry structure, theme-specific primary colors carry action, orange carries attention, and the overdue pair marks only overdue Project days.

At startup, use the saved display mode or the OS preference (`prefers-color-scheme`) when no selection is saved. Place DisplaySwitcher next to LanguageSwitcher in both application and authentication headers. Manual choices are saved in `localStorage`; if storage is unavailable, the selection applies for the current session. [features/display](src/features/display/index.ts) initializes the mode before React renders and applies manual changes in event handlers.

| Role | CSS reference | Usage |
| --- | --- | --- |
| Ground | `--color-ground` | Page background |
| Surface | `--color-surface` | Cards, top bar, sidebar, and neutral buttons |
| Input | `--color-input` | Inputs and selects; recessed below cards in dark mode |
| Raised | `--color-raised` | Table headers/totals, dialog footers, read-only fields, and done-task blocks |
| Elevated | `--color-elevated` | Menus |
| Borders | `--color-border`, `--color-border-strong`, `--color-input-border` | Dividers, control outlines, and input outlines respectively |
| Text | `--color-text`, `--color-text-body`, `--color-text-muted` | Primary, secondary, and supporting text |
| Placeholder | `--color-placeholder` | Placeholder text only |
| Actions | `--color-action-background`, `--color-action-text` | Primary buttons and checked/selected controls |
| Selection | `--color-selected-background`, `--color-selected-text`, `--color-selected-border` | Active navigation, suggestions, info bands, and calendar events |
| Attention | `--color-attention-background`, `--color-attention-foreground` | Waiting counts and deadline warnings |
| Overdue label | `--color-overdue-background`, `--color-overdue-foreground` | Project rows showing `超過xx日` / `Overdue by xx days` |
| Success | `--color-success-background`, `--color-success-foreground` | Done state |
| Danger | `--color-danger` | Negative variances and Sunday dates |
| Errors | `--color-danger-text`, `--color-danger-soft`, `--color-danger-border` | Error messages and banners |
| Overlay | `--color-overlay` | Page dimming behind dialogs |
| Selected row | `--color-row-selected` | Selected table rows |
| Weekend | `--color-weekend` | Weekend calendar columns |

Use `--color-primary` for links, progress fills, actual-time bars, today's date, and the Next tag. Use `--color-primary-strong` for emphasized navigation/info text, `--color-avatar` for the avatar background, and `--color-chart-planned` for planned chart bars. Use `--color-attention` for the logo and calendar's current-time marker. The Projects overdue-day label is the only blue exception and uses `--color-overdue-background` / `--color-overdue-foreground`. Orange never marks an action; small orange text and badges use the attention foreground/background pair. Text on primary actions uses `--color-on-primary`, which becomes dark in dark mode.

## Typography

Use the global classes from [typography.css](src/styles/typography.css). The file owns font families, sizes, weights, line heights, and tabular figures. Components choose a role rather than defining their own typography.

| Purpose | Global class |
| --- | --- |
| Page title | `text-page-title` |
| Login title | `text-login-title` |
| Card or section title | `text-card-title` |
| Summary value | `text-stat-value` |
| Body text | `text-body` |
| Emphasized body text / active navigation | `text-body-strong` |
| Table numbers | `text-numeric` |
| Table headers and labels | `text-label` |
| Login field labels | `text-field-label` |
| Page description | `text-description` |
| Supporting captions | `text-caption` |
| Check-row details | `text-detail` |
| Chart date labels | `text-chart-date` |
| Status tag | `text-tag` |
| Button / small button | `text-button` / `text-button-small` |

- Keep Japanese, Latin, and digits in the shared UI font family so mixed text stays even.
- Use the regular and bold weights defined in CSS. Do not introduce light or thin text.
- Numbers use tabular figures and are right-aligned in tables.
- Times and durations use `h:mm` (`0:45`, `12:30`). Large summary values may use words (`2時間36分`).
- Dates follow Japanese convention with the weekday in parentheses: `2026年10月5日（月）`. Ranges use `〜`.
- Negative variances use the accounting triangle and danger color: `▲1:30`, never a minus sign.
- The wordmark is outlined artwork. Never typeset it live or use its display typeface for UI text.

## Layout

Use [sizes.css](src/styles/sizes.css) for shared dimensions and [spacing.css](src/styles/spacing.css) for the spacing scale and padding. The desktop layout has a fixed full-width top bar, a left sidebar below it, and content on the right. Adapt composition to the available width and text size; reference images do not establish a fixed viewport.

- **Top bar**: [Header](src/features/shared/layout/Header/index.tsx) owns its markup and CSS Module. Use `--size-topbar-height` and `--padding-topbar`; responsive header height lives in [sizes.css](src/styles/sizes.css) so Header and SideMenu use the same value. Logo on the left, search field using `--size-search-width` / `--size-search-height`, and Help, Notifications, and avatar on the right.
- **Sidebar**: `--size-sidebar-width` and `--padding-sidebar`. Navigation uses `--size-nav-item-height`. Today and Task Inbox come first; Manage / 管理 groups Projects / プロジェクト and Calendar / カレンダー; Insights / 分析 groups KPI Dashboard / KPIダッシュボード. Profile / プロフィール sits at the bottom. Separate it from the other links with a divider. Waiting Inbox items have an attention-colored count.
- **Language**: place the EN / JP switch on the right side of the header. Navigation groups read 管理 / 分析 in Japanese and Manage / Insights in English.
- **Display**: place the sun / moon switch beside the language switch, before Help when present. On narrow screens, wrap the header so both controls remain usable with enlarged text.
- **Content**: `--padding-page`, with `--space-lg` between blocks. Start each page with a title, a description beneath it, and actions on the right, with the primary action last.
- **Spacing**: use the `--space-*` scale. Card headers use `--padding-card-header`; table cells use `--padding-table-cell`.
- **Grids**: content comes first. Today uses a main area and `--size-today-side-column`; KPI uses `--ratio-kpi-chart` relative to the project table column.

Font-relative sizes and spacing use `rem`; breakpoints use `em`. Borders and focus outlines use `px`. Let text containers grow or wrap instead of clipping enlarged text.

## Elevation & Depth

Use [effects.css](src/styles/effects.css) and [radius.css](src/styles/radius.css).

- **Flat**: ground, top bar, and sidebar. Separate bars with `--border-width` and `--color-border`, without shadows.
- **Card**: surface background, `--radius-md`, and `--shadow-card`. In dark mode this token supplies only a border-colored outline, without a drop shadow.
- **Menu**: elevated background, strong outline, `--radius-md`, and `--shadow-menu`.
- **Dialog**: surface background, strong outline, `--radius-lg`, and `--shadow-dialog`, over `--color-overlay`.
- **Focus**: use a primary-colored border/outline at `--focus-width`; inputs add `--shadow-focus`. Keyboard outlines use `--focus-outline-offset`.

Do not stack shadows or use elevation to indicate focus.

## Shapes

- `--radius-full`: buttons, count badges, progress bars, avatar, and today's date circle.
- `--radius-md`: cards.
- `--radius-sm`: navigation items and search field.
- `--radius-xs`: inputs, selects, checkboxes, tags, segmented controls, icon buttons, and calendar blocks.
- App icon tile geometry belongs to the brand artwork, including its rounded silhouette.
- Icons use `--icon-stroke-width`, round caps, and consistent line forms. Navigation icons use `--size-nav-icon`; control icons use `--size-control-icon-small` or `--size-control-icon`.
- The move-to-today action uses the same bent-arrow glyph (↳) as the logo.

## Components

**Two-option switchers.** [Switcher](src/features/shared/components/Switcher/index.tsx) owns the shared track, thumb, interaction states, and motion. LanguageSwitcher supplies EN / JP and language state; DisplaySwitcher supplies sun / moon SVG icons and display state. Left / right order stays fixed, with the selected value inside the thumb and the other value visible on the track.

- Use the shared dimensions in [sizes.css](src/styles/sizes.css), the `text-switcher` role in [typography.css](src/styles/typography.css), and theme-specific colors in [color.css](src/styles/color.css). Light mode uses a light-gray track with a dark thumb; dark mode uses a dark-gray track with a near-white thumb. Track labels keep body-text contrast in normal and hover states.
- The thumb moves using `--motion-switcher-duration` and `--motion-switcher-easing`, stretches while pressed, and swaps its content halfway through the movement. [effects.css](src/styles/effects.css) owns motion and thumb-shadow tokens. Honor `prefers-reduced-motion` by removing transitions.
- The whole control has an input-height hit area. Hover changes the track fill, keyboard focus outlines the track, and disabled controls are dimmed and cannot change state. A native button with switch semantics supports Enter / Space; Left / Right and Home / End choose either side. The accessible label is localized and the selected value is provided as its description.

**Buttons.** Use `--radius-full`, `--size-button-height`, and `--padding-button`; small buttons use `--size-button-small-height` and `--padding-button-small`.

- Primary: action foreground/background pair and `text-button` (or `text-button-small`).
- Outline: surface background, primary text and border. Use for 確認する (Review) and 見る (View).
- Neutral: surface background, normal text, strong border, and `text-body`. Use for プランを調整 (Adjust plan) and CSVで出力 (Export CSV).
- Each page has at most one primary button in its header.
- Text links are primary-colored, bold, and underlined so their affordance is visible without color.

**Status tags.** Use `--size-tag-height`, `--radius-xs`, `text-tag`, and horizontal `--space-sm` padding.

- 完了 (Done): success pair.
- 次 (Next): action pair.
- 予定 (Scheduled): normal body text on raised with a divider-colored outline.
- 未着手 (Not started): body text on surface with a strong border.
- 今日 / deadline warnings: attention pair.
- Project overdue days: overdue pair, with the row copy `超過xx日` / `Overdue by xx days`.
- 推測 (Suggested): selection pair, `text-caption`, and `--size-suggestion-tag-height`.

**Tabs.** Use `--size-tab-height`, horizontal `--space-md` padding, and `text-body`. The active tab uses primary text, `text-body-strong`, and an underline at `--size-tab-indicator`. Each tab has a count chip; waiting items use the attention pair. Example: 振り分け待ち 4 / 登録済み 9 / 対象外 1.

**Tables.** Headers use raised, `text-label`, and `--size-table-header-height`. Rows use surface, `text-body`, `--size-table-row-height`, and divider-colored borders. Numbers use `text-numeric` and right alignment. Where specified, a 合計 totals row uses raised and `text-body-strong`; Projects has no totals row. Selected rows use `--color-row-selected`. Inline editors use `--size-inline-editor-height` and `--radius-xs`.

**Suggestion row (Task Inbox).** Columns: checkbox · subtask title (parent task beneath in `text-caption`) · estimate · project select · date select with 推測 · 登録 button. A bulk bar reads 2件を選択中 followed by まとめて登録. An info band uses the selection pair, `text-description`, and `--padding-info-band` to preview the effect, such as 今日の見積りは 2:45 → 3:50 になります.

**やることリスト.** One row per thing to check. Each row has a round `--size-check-icon` icon on the selection background (attention background for waiting items), a `text-body-strong` sentence, `text-detail` supporting text, and a button on the right.

**Summary strip.** Equal cells separated by divider-colored borders. Use `--padding-stat`, `text-label`, and `text-stat-value`. Negative values use danger color and ▲.

**Progress.** Use `--size-progress-height`, `--radius-full`, a divider-colored track, and a primary fill. Always pair it with a tabular percentage or n / m.

**Charts.** Plan/actual bars use `--size-chart-bar-width` and `--space-xs` gaps: plan uses `--color-chart-planned`, actual uses primary. Use `--radius-chart-bar` on bar tops. Gridlines use the divider color and border width; axes use muted `text-caption`, and dates beneath weekdays use `text-chart-date`.

**Calendar.**

- Week-view hour cells use `--size-calendar-hour`. The reference shows 9:00 to 18:00; this is sample content.
- Events use the selection pair and selection border.
- Today's tasks use surface and a strong border. The next task uses a primary border at `--border-width-emphasis`. Done tasks use raised and muted, struck-through text.
- Current time uses attention color, `--size-calendar-now-line`, `--size-calendar-now-dot`, and an attention-foreground time label.
- Today's date uses `--size-calendar-today-date` and the action pair. Sunday dates use danger; Saturday dates use normal text. Weekend columns use `--color-weekend`.
- Place a legend at the bottom of the card.

**Forms (Login).**

- Center a card at `--size-login-card-width` with the wordmark above it.
- Use `text-field-label` above inputs. Inputs use `--size-input-height`, `--padding-input`, `--radius-xs`, and the input border color.
- The login button spans the form width at `--size-login-button-height`, using the primary pill style.
- Stack neutral social sign-in pills under an または divider. Use the providers' official marks.
- Use a separate small card for はじめての方は、無料で登録できます with an outline sign-up button.

## Screens

- **Today**: page header (date `2026年10月5日（月）`; actions: プランを調整 (Adjust plan), ＋タスクを追加 (Add task)).
  - やることリスト card: items waiting in the Inbox, an upcoming deadline, and a new report.
  - 今日のタスク (Today's tasks) table: status · time · task · project · estimate · action. The next task carries a small 開始 (Start) primary button. A totals row closes the table.
  - Side column: 今日の進捗 (Today's progress, `1 / 4` with a progress bar) and 次の予定 (Next event).
- **Task Inbox**: tabs, then one card holding the filter bar (project, date, text filter, bulk register), the suggestion table and the info band. Header actions: ルールを設定 (Set rules, neutral) and ＋タスクを追加 (Add task, primary).
- **Projects**: tabs 進行中 / 保留 / 完了 / オープン / 他者待ち / ゴミ箱 (In progress / Pending / Done / Open / Waiting on others / Trash), in that order; default 進行中. Trash remains in the same list layout.
  - One card: a four-cell summary strip for selected status and total count, end dates from today through 14 days ahead, overdue count, and today's Projects placeholder `- 件`. Counts cover the full selected tab, not the visible cursor page.
  - Table columns: project title and goal · progress · deadline · remaining days · today's tasks placeholder · status select · edit/trash action. Trash rows replace edit/status controls with restore. Do not show subtasks, completion counts, a totals row, or a separate upcoming-subtask card. Keep per-Project progress bars and percentages.
  - User-sortable data columns are title, progress, deadline, and remaining days. Sorting is single-column and server-side before 20-row cursor pagination. Default deadline order is earliest first, with overdue rows first and no deadline last. For an equal date, higher priority comes first. Keep deterministic ties.
  - The `今日のタスク` / `Today's tasks` cells remain `-` placeholders until TodoList-based Project counts exist. Overdue copy uses the overdue color pair. Other deadlines use the existing neutral and attention roles.
- **Project form**: create `/projects/new` and edit `/projects/:id/edit` use the same form layout. Keep one wide card with a left label column and right input column, row dividers, and a raised footer action bar. Fields: required project title · Goal (WHY) · Details · type · priority · start date · end date. Put Details immediately after Goal (WHY); keep the API fields `goal` and `description`. Initial type is `other`, priority is `low`, and both dates are empty. Do not include weekly work hours or templates. Validate end date against start date but allow past dates. Keep field errors beside their inputs. Save only on explicit submit.
- **Calendar**: one card with a toolbar (今日 (Today), previous/next, the date range, and a 日/週/月 (Day/Week/Month) segmented control), the week grid, and a legend.
- **KPI Dashboard**: a period row (週次 (Weekly) select and a stepper), then the summary strip (達成率 (completion rate), 実績時間 (actual time), 予定時間 (planned time), 予定との差異 (variance)). Below it, the chart card 日別の予定と実績 (planned vs. actual by day) next to the table card プロジェクト別 (by project): planned · actual · variance · completion, closed by a totals row and a footnote explaining ▲.
- **Login**: see Forms.

## Brand Assets

- **Wordmark "Task2TodayTodo"**: Outfit Black outlines with tracking about −2.8%. Only the **2** carries the brand idea: its base runs on into an arrow. The arrow's shaft equals the 2's bar thickness, the arrowhead is 2.35× that height and 1.12× that length, and its tip tucks under the T of "Today".
  - Colors: ink text `--color-brand-ink` with an orange 2 (on light), or white text with an orange 2 (on dark). Single-color versions exist.
  - Files: `wordmark-ink.svg`, `wordmark-white.svg`, `wordmark-mono-*.svg`.
  - Current implementation: [Wordmark](src/features/shared/components/Wordmark/index.tsx) uses the existing raster artwork as separate ink/orange SVG masks. It preserves the artwork geometry and uses CSS tokens for theme-specific text and unchanged brand orange.
- **Short logo "T2TT"**: the first T, the arrow-2, and "TT" (Today + Todo) joined under one shared crossbar. Use it where the full name doesn't fit, down to 32px.
  - Files: `t2tt-ink.svg`, `t2tt-white.svg`, `t2tt-mono-*.svg`, `t2tt-app.svg`.
- **Square mark (no text)**: a white rounded block (the task) and, below it, an orange bent arrow (↳) turning right toward today, built with the same arrow proportions. Use it for favicons and anything at 16–24px.
  - Files: `mark-app.svg` (dark tile `--color-brand-dark`), `mark-light.svg`, `mark-plain*.svg`, `mark-mono-*.svg`.
- Keep clear space around any logo equal to the height of the arrowhead. Never recolor the 2 to anything but the brand orange (or the single-color version), and never add effects.

## Do's and Don'ts

- **Do** use theme-specific primary colors for actions and attention orange for attention and now. Never introduce blue.
- **Do** use the attention foreground/background pair for small orange text and badges; keep white-on-orange out of these roles.
- **Do** pre-fill guesses and label them 推測. Let users confirm with 登録 per row or まとめて登録 for a selection.
- **Do** close numeric tables with 合計, right-align numbers, and use tabular figures and `h:mm` durations.
- **Do** show negative variances with danger color and ▲.
- **Do** place at most one primary action last in each page header.
- **Do** write short, polite です・ます copy using タスク, プロジェクト, and 今日.
- **Do** use the CSS ground and surface roles consistently.
- **Don't** stack shadows, use heavy shadows, or float the layout bars.
- **Don't** use blue outside the Projects overdue-day label. Don't use gradients, illustrations, or emoji in product UI.
- **Don't** copy freee's logo, mascot, illustrations, or exact screens. Borrow interaction patterns and density only.
- **Don't** typeset the wordmark live or use Outfit elsewhere in the UI.
