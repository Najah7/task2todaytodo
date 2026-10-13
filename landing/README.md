# Task2TodayToDo landing page

Astro project for the Task2TodayToDo Japanese landing page. The page is a static mock; signup is not connected yet.

## Development

Run commands from this directory.

```sh
npm install
npx astro dev --background
npm run build
```

Manage the background dev server with `npx astro dev status`, `npx astro dev logs`, and `npx astro dev stop`.

## Structure

- `src/pages/index.astro` composes page sections and content.
- `src/components/` contains reusable repeated page items and cards.
- `src/layouts/Layout.astro` owns document metadata and client-side interactions.
- `src/styles/global.css` holds the landing page styles.
