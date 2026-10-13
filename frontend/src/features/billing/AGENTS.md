# Billing feature guide

Follow the [feature guide](../AGENTS.md). The page path is `/billing`, defined in [router.tsx](../../router.tsx); API contracts are generated from the backend Billing OpenAPI tag.

## Behavior

- Display subscription state returned by the backend and direct all plan changes through the billing API.
- New subscribers use Stripe Checkout; paid subscribers use the app's plan-change endpoint and Customer Portal.
- Do not implement product access checks or repeat billing rules in the client.
- Use Checkout and Portal URLs only when they point to the expected Stripe hosts.
- Prices are JPY 1,350/month for Pro and JPY 2,250/month for Team, quantity one per user.

## Verification

- `pnpm build` and the billing page unit test cover the client contract and interaction states.
