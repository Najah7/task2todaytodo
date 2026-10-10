# Frontend guide

## Core

- Keep the frontend thin. Centralize business logic in the backend; do not distribute or duplicate it across the system. Revisit the design when frontend logic becomes complex.
- Preserve a single source of truth across the system: business rules, data, state, API contracts, and design values.
- The frontend owns presentation and interaction. Client validation provides early feedback; the backend remains responsible for data validity.

## Feature guidance

Before working in `src/features/`, composing feature Pages in `src/pages/`, or editing their tests and stories, read [src/features/AGENTS.md](src/features/AGENTS.md). It defines component structure, file ownership, Context/Provider responsibilities, and List/New/Edit conventions.

## State

- The backend is the source of truth for server data; TanStack Query owns its client cache. Do not maintain a second server-data cache in Jotai or local State. Editing drafts and temporary UI state are local.
- Use React Router's URL state for routes and shareable navigation state. Use Jotai only for frontend-specific state that genuinely needs global access.

## API

- Generate API types, clients, and Query hooks with Orval from the backend OpenAPI specification. Never hand-edit generated code or duplicate API contracts.
- Handwrite only necessary integration, such as shared HTTP behavior and actions after a successful request. Do not add wrappers that merely repeat the generated API.
- Use React Hook Form and Zod for forms; colocate `schema.ts` with the form.

## Styles

- Use CSS Modules to encapsulate each component's layout and appearance. Do not import another component's private styles.
- Keep global CSS limited to broadly shared, focused concerns: typography classes and design tokens such as colors, spacing, and radii.
- `src/styles/` is the design-value source of truth; `DESIGN.md` references it. Use global `text-*` typography classes and CSS variables instead of redefining values locally.
- Prefer `rem` for scalable dimensions and `em` for media queries; `px` is appropriate for thin borders and outlines.

## Notifications

- Define a small shared toast interface in `src/features/shared/notification`; features use this interface for toast notifications.
- Show field validation errors, such as required fields and date relationships, persistently next to the relevant inputs.
- Use toasts to report success or failure for create, save, status change, move to trash, restore, and network errors.
- After a failed status change, notify the user with a toast and refetch the relevant server state to reconcile the UI.

## Tests

- `e2e/`: Playwright tests for important user journeys against the real backend and database. Do not mock API responses.
- Run E2E with an isolated, disposable Docker Compose environment. Keep test data independent so tests can run in parallel.
- Colocate unit tests with complex or high-impact components, pure functions, and schemas. Use Vitest and React Testing Library; mock dependencies here when needed.
- Test meaningful behavior and failures. Keep detailed cases out of E2E; do not retest trivial rendering or library internals.

## Conventions

- TypeScript, React, Vite, React Router, TanStack Query, Jotai, Orval, and CSS Modules are the existing stack.
- `~/` resolves to `frontend/src/`. Use it for project imports; `./...` is allowed for the same directory and descendants. Do not use `../...` imports.
- Keep system text in `src/features/i18n/` for Japanese and English. Do not translate user-created data.
- See `README.md` for commands and `e2e/README.md` for the E2E environment.
