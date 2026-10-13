# Task2TodayToDo

## Product

The primary value is generating today's doable workload list from managed ActionItems, eligible direct Tasks, and Schedules. Managing Tasks and Schedules together in one platform is the supporting value.

Reduce the burden of maintaining long-term plans so users can focus on today's work. Automatically generate a Todo when first created for a user-local day, selecting whole ActionItems and existing direct childless Tasks using positive active time, project allocations, fixed Schedules, the gap policy, and priority multiplied by urgency. Project work fits only in its own allocation; projectless work fits active-time gaps outside every project allocation. An allocation must fit wholly inside active time. Do not split work or create virtual Projects. Keep Schedule times fixed and visible. After initial generation, users can add or remove work and change display order; no automatic regeneration or repair occurs. Internal work timestamps stay hidden and unchanged by display reorder or Schedule edits.

Users may explicitly regenerate today's Todo with its current revision. Regeneration is destructive: it replaces every saved row, including completed, manual, hidden, and reordered rows, with a fresh plan from current sources. It preserves completion and actual minutes on source records, keeps the Todo identity, and advances its revision. Stale revisions must leave the Todo unchanged. Source changes never trigger automatic regeneration.

When a Schedule is created, its start time changes, or it is restored, insert it before the item with the nearest internal `scheduled_start` greater than or equal to the Schedule start, using that item's current display position as anchor; append if no such item exists. Preserve other items' relative order. `position` is the persisted display order.

## Domain

- Project: Groups Tasks and Schedules. Tasks and Schedules may also exist without a Project.
- Task: Groups ActionItems.
- ActionItem: One-off work without a fixed start and end time.
- Schedule: Work with a fixed start and end time; one-off or recurring.
- Todo: The daily doable workload list, initially generated once for a user-local day and combining eligible work and fixed Schedules. Users can then add/remove work and reorder its display.
- Active time: Positive user-local work intervals defined by `active_start_time` and `active_end_time`, with per-day `daily_active_time_exceptions` (`active_start_local`, `active_end_local`, timezone, and revision). Project allocation intervals must fit wholly inside active time.

## Principles

- Centralize business rules in the backend. Clients own presentation and interaction.
- Preserve a single source of truth across the system for business rules, data, API contracts, and design values.
- Give each file one clear responsibility. Keep related functions and types together when they serve that responsibility; split files that handle distinct concerns.
- Build concrete solutions first. Extract abstractions from demonstrated needs; abstract upfront only with a clear, stable requirement.
- Maintain AGENTS.md as well since it's part of the source of truth for the system.

## Future direction

Planned: Google Calendar integration, MCP support, and a mobile app.

## Local guidance

Before working in backend or frontend, read its guide: [backend/AGENTS.md](backend/AGENTS.md) or [frontend/AGENTS.md](frontend/AGENTS.md). Local guides and READMEs define implementation conventions and commands. Relevant specifications define detailed behavior.
