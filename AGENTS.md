# Task2TodayToDo

## Product

The primary value is automatically generating today's TodoList as an execution plan from managed TodoItems and Schedules. Managing Tasks and Schedules together in one platform is the supporting value.

Reduce the burden of maintaining long-term plans so users can focus on today's work. Keep Schedule times fixed when generating the plan, and place TodoItems into available time in priority order. Users can manually adjust the daily plan.

## Domain

- Project: Groups Tasks and Schedules. Tasks and Schedules may also exist without a Project.
- Task: Groups TodoItems.
- TodoItem: Work without a fixed start and end time; one-off or recurring.
- Schedule: Work with a fixed start and end time; one-off or recurring.
- TodoList: The generated execution plan for a day, combining TodoItems and Schedules.

## Principles

- Centralize business rules in the backend. Clients own presentation and interaction.
- Preserve a single source of truth across the system for business rules, data, API contracts, and design values.
- Build concrete solutions first. Extract abstractions from demonstrated needs; abstract upfront only with a clear, stable requirement.

## Future direction

Planned: Google Calendar integration, MCP support, and a mobile app.

## Local guidance

Before working in backend or frontend, read its guide: [backend/AGENTS.md](backend/AGENTS.md) or [frontend/AGENTS.md](frontend/AGENTS.md). Local guides and READMEs define implementation conventions and commands. Relevant specifications define detailed behavior.
