# Admin guide

See the [application guide](../AGENTS.md) for application boundaries.

## Package structure

- Keep `job/` and `operation/` as subareas of the single Admin context.
- `job/usecase` owns endpoint-triggered job execution, the asynchronous runner,
  and lifecycle behavior it needs. It may consume `operation/dao` and
  `operation/usecase.ErrLeaseLost` through its narrow repository port.
- `operation/dao`, `operation/repository`, and `operation/usecase` own operation
  persistence and read APIs, including `ErrOperationNotFound` and `ErrLeaseLost`.
- Operation packages must not import job packages. The application root
  composes both usecase groups; REST receives them as separate groups.
- Keep `AdminStore.Operations` backed by the operation repository. Do not add
  compatibility packages at the previous flat paths.

## Admin jobs

- The backend provides authenticated admin-job trigger endpoints only.
- External infrastructure or schedulers, such as AWS EventBridge, own job
  invocation timing and recurrence. Do not implement cron-, timer-, or
  calendar-based automatic job invocation inside the API process.
- External scheduler setup and deployment are outside application code scope.
- After an endpoint invocation, on-demand asynchronous execution may use the
  existing backfill-schedules LRO/PAT permission model. Background job
  execution is allowed; this policy concerns who schedules invocation, not
  where the invoked work runs.
