# Astro Landing Page

This directory owns the independent Astro mock of the Task2TodayToDo landing page. It is separate from the React app in `frontend/`; keep landing-only work here.

## Architecture

- `src/pages/index.astro` composes page sections and holds data for repeated editorial items, feature cards, steps, pricing plans, and FAQs.
- `src/components/` contains reusable `Brand`, `EditorialItem`, `FeatureCard`, `StepItem`, `PlanCard`, and `FaqItem` components. Keep repeated content in page data and avoid abstractions for one-off sections. `Brand` imports wordmark art from `frontend/src/assets/service_name.webp` and recolors it for the dark footer.
- `src/layouts/Layout.astro` owns Japanese document metadata, font links, the favicon imported from `frontend/src/assets/logo.webp`, and client-side demo, mobile navigation, reveal, and active-section behavior.
- `src/styles/global.css` contains the landing page design and responsive styles.
- Signup is a static mock: the final CTA is disabled and no account flow is connected.

## Development

Run commands from `landing/`:

```sh
npm install
npx astro dev --background
npm run build
```

Manage the background dev server with `npx astro dev status`, `npx astro dev logs`, and `npx astro dev stop`.

## Astro Documentation

Full documentation: https://docs.astro.build

- [Routing](https://docs.astro.build/en/guides/routing/)
- [Astro components](https://docs.astro.build/en/basics/astro-components/)
- [Client-side scripts](https://docs.astro.build/en/guides/client-side-scripts/)
- [Styling](https://docs.astro.build/en/guides/styling/)
- [Internationalization](https://docs.astro.build/en/guides/internationalization/)
