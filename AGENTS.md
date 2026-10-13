# Task2TodayToDo

## Overview

Task2TodayToDo is a productivity SaaS that turns managed work and fixed commitments into a doable Todo for today. Its main value is helping people spend less effort planning and more time doing. The product serves individuals first; shared Projects support small-team use.

## Problem

Recording tasks does not answer what to do today. People still have to judge priorities, find time around meetings, fit work into available hours, divide time among Projects, and revise plans when circumstances change. Repeating that planning work creates cognitive load without moving the work forward.

## Solution

Manage Projects, Tasks, ActionItems, and Schedules together. The backend uses that source data, active time, Project allocations, fixed commitments, priority, and urgency to generate each user's daily Todo. Users can then adjust the list for the day. Keep planning rules and persisted state in the backend; clients own presentation and interaction. Maintain one source of truth for business rules, API contracts, and design values.

## Features

- **Work and time:** Projects group Tasks and Schedules; Tasks group ActionItems. Tasks and Schedules can also exist without a Project. Schedules have fixed times and can recur. Users define active time and Project allocation windows.
- **Today's Todo:** Generate a plan once for each user-local day, selecting whole eligible ActionItems and direct childless Tasks that fit around fixed Schedules. Project work fits its allocation; work without a Project uses gaps outside Project allocations. After generation, users can add or remove work, reorder display, and complete items. Source changes do not automatically regenerate the plan. Explicit regeneration replaces the saved daily rows using the current revision. See the [Todo guide](backend/internal/application/todo/AGENTS.md) for the exact placement, revision, and Schedule synchronization rules.
- **Collaboration and insight:** Project membership, invitations, assignment, a weekly allocation view, an Inbox, and Todo completion and overdue-work metrics are present in the codebase.
- **Billing:** Free, Pro, and Team plan states are present. Pro and Team are monthly, per-user subscriptions managed through Stripe. Configured Stripe Prices determine current charges. Feature limits and plan entitlements are not implemented by the billing flow; figures and limits in business notes are hypotheses rather than current product rules.
- **Future direction:** Google Calendar integration, MCP support, and mobile access are planned, not current features.

## Backend Map

Read [backend/AGENTS.md](backend/AGENTS.md) first, then guides for changed areas. Links do not load guides automatically.

- [application/AGENTS.md](backend/internal/application/AGENTS.md): context boundaries, operation-specific use cases, repositories, and transactions. Then read the relevant context guide, such as [Todo](backend/internal/application/todo/AGENTS.md) or [Billing](backend/internal/application/billing/AGENTS.md).
- [db/AGENTS.md](backend/db/AGENTS.md): migrations, SQL sources, and generated sqlc code.
- [port/rest/AGENTS.md](backend/internal/port/rest/AGENTS.md): HTTP handlers and errors. `backend/cmd/api/` owns routes and wiring.
- [settings/AGENTS.md](backend/internal/settings/AGENTS.md) and [telemetry/AGENTS.md](backend/internal/telemetry/AGENTS.md): runtime configuration and observability.

Keep affected guides current. Give each file one responsibility; extract abstractions from concrete needs.

## Frontend Map

Read [frontend/AGENTS.md](frontend/AGENTS.md), then [src/features/AGENTS.md](frontend/src/features/AGENTS.md) and the changed feature's guide. Links do not load guides automatically.

- `frontend/src/pages/` composes screens; `frontend/src/router.tsx` declares routes.
- `frontend/src/features/` owns feature UI and interactions; its guide links to feature guides.
- `frontend/src/api/generated/` comes from backend OpenAPI; do not hand-edit. `frontend/src/styles/` owns design values.
- `frontend/e2e/` tests journeys against the real backend.

Keep affected feature guides current.
