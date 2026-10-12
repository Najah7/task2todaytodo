# Inbox feature guide

Follow the [feature guide](../AGENTS.md). The path is defined in [router.tsx](../../router.tsx); list policy is in the [backend Inbox guide](../../../../backend/internal/application/inbox/AGENTS.md).

## Features and page

- `/inbox`: browse a combined Task/Schedule list, filter by type/Project/title, page through results, quick-create work, and complete or add work to Today.

## Key files

- [pages/Inbox](../../pages/Inbox/index.tsx): screen composition.
- [InboxAPIDataProvider](providers/InboxAPIDataProvider/index.tsx), [InboxNavigationProvider](providers/InboxNavigationProvider/index.tsx): list queries, URL filters, and pagination tokens.
- [InboxActionProvider](providers/InboxActionProvider/index.tsx): quick-create and row actions through Task, Schedule, and Todo APIs.
- [InboxList](components/InboxList/index.tsx), [InboxQuickCreate](components/InboxQuickCreate/index.tsx), [InboxPagination](components/InboxPagination/index.tsx): presentation and interactions.

## Change checks

- Treat the combined list, counts, and page tokens as server-owned. Keep URL filters and pagination scope synchronized when changing search state.
- Quick actions mutate Task, Schedule, or Todo resources, not Inbox records; invalidate/refetch affected Query data and preserve useful cached rows during background refresh.
- Schedule formatting uses occurrence timezone and all-day behavior; Task and Schedule capabilities decide available row actions.

## Verification

- [Inbox E2E](../../../e2e/inbox.spec.ts), InboxActionProvider and InboxNavigationProvider tests, and InboxList tests.
