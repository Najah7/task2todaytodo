# Progress implementation notes

- Remove `tasks.progress` and `projects.progress` from development migration
  `000003`; keep progress on read DAOs and REST responses.
- Use one SQL projection statement for all requested tasks and projects. In
  that snapshot, deduplicate saved child occurrences by series/date, retain
  deleted children as masks, aggregate saved counts in PostgreSQL, and return
  only active recurrence-root metadata needed to evaluate today's virtual
  rows. This avoids loading full historical rows into Go and keeps task status,
  both child kinds, roots, and project membership on the same database
  snapshot. Reuse domain recurrence date generation and schedule wall-time
  validation for today's virtual rows.
- Apply one shared calculator to task Get/List, project task lists, task
  responses from create/update, and project Get/List/create/update/nested task
  responses, including add/remove task responses. Project aggregation includes all project tasks, independent of
  task-list pagination.
- Child-write transactions lock parent Task before capturing counts. After
  mutation, recompute with one fixed reference time and conditionally update
  status. Do not run automatic status decisions on reads or explicit task
  status commands.
- Regenerate sqlc output from `db/queries`; test calculation, recurrence
  identity, status transitions, paged/nested reads, and database concurrency.
