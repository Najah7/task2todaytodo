# Repo layout and important directories

TODO

# Tech Stack
- [Typescript](https://github.com/microsoft/typescript): Programming Language
- [Vite](https://github.com/vitejs/vite): Frontend build tool
- [React](https://github.com/react/react): UI library
- [Tanstack Router](https://github.com/tanstack/router): Routing library
- [TanStack Query](https://github.com/tanstack/query): Data fetching and caching library
- [Oxlint](https://github.com/oxc-project/oxc): Linter for TypeScript and JavaScript

# How to run the project

This template provides a minimal setup to get React working in Vite with HMR and some Oxlint rules.

Currently, two official plugins are available:

- [@vitejs/plugin-react](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react) uses [Oxc](https://oxc.rs)
- [@vitejs/plugin-react-swc](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react-swc) uses [SWC](https://swc.rs/)

## React Compiler

The React Compiler is enabled on this template. See [this documentation](https://react.dev/learn/react-compiler) for more information.

Note: This will impact Vite dev & build performances.

## Expanding the Oxlint configuration

If you are developing a production application, we recommend enabling type-aware lint rules by installing `oxlint-tsgolint` and editing `.oxlintrc.json`:

```json
{
  "$schema": "./node_modules/oxlint/configuration_schema.json",
  "plugins": ["react", "typescript", "oxc"],
  "options": {
    "typeAware": true
  },
  "rules": {
    "react/rules-of-hooks": "error",
    "react/only-export-components": ["warn", { "allowConstantExport": true }]
  }
}
```

See the [Oxlint rules documentation](https://oxc.rs/docs/guide/usage/linter/rules) for the full list of rules and categories.

# Build, test, and lint commands

Run these commands from `frontend/`:

```bash
pnpm build
pnpm lint
pnpm test:unit
pnpm exec playwright install chromium --only-shell
pnpm test:e2e
```

Unit tests use Vitest and React Testing Library and live beside their implementation. E2E tests live in `e2e/` and use Playwright with a real backend and database. `pnpm test:e2e` starts an isolated Docker Compose environment and removes it afterward; a running Docker daemon is required. See [E2E setup and lifecycle](e2e/README.md).

# Storybook

Run Storybook from `frontend/`:

```bash
pnpm storybook
pnpm build-storybook
```

The development UI is available at `http://localhost:6006`. The static build is written to `storybook-static/`.

Colocate `Component.stories.tsx` with its component. Storybook reuses the Vite configuration, including the `~/` alias, and imports the application's global CSS. Use the toolbar to switch between light/dark themes and Japanese/English.

Keep shared decorators, mocks, and interaction helpers under `.storybook/`. Authentication helpers live in `.storybook/auth/` and are imported with `~storybook/auth/...`.

Stories cover Switcher, Wordmark, SubmitButton, EmailField, PasswordField, LoginForm, and SignupForm. Form stories include validation errors, API errors, pending submissions, and login retry after signup. Field stories include read-only and password visibility states.

Storybook uses MSW to mock authentication responses, so these previews run without a backend. Submitting a default form shows a Today heading inside the preview. Unhandled `/api/` requests are blocked. E2E tests continue to use the real backend. The service worker lives in `.storybook/public/` and is excluded from the application build; regenerate it after upgrading MSW with `pnpm exec msw init .storybook/public --save`.

# Engineering conventions and PR expectations

See [AGENTS.md](AGENTS.md) for the core frontend architecture, state, styling, and testing conventions.

# Constraints and do-not rules

TODO

# What done means and how to verify work

TODO
