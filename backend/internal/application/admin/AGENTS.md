# Admin guide

See the [application guide](../AGENTS.md) for application boundaries.

## Features

- Trigger Schedule backfill and allocation prefill as asynchronous jobs. Persist operation status, progress, results, and errors for later reads.
- The API executes jobs on demand; external infrastructure decides when to invoke them.

## Endpoints

- `POST /api/admin/jobs/backfill-schedules:run`, `/api/admin/jobs/prefill-allocations:run`.
- `GET /api/admin/operations`, `/api/admin/operations/{id}`. Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Key files

- [job/usecase/runner.go](job/usecase/runner.go): asynchronous operation runner.
- [job/usecase/backfill_schedules.go](job/usecase/backfill_schedules.go), [job/usecase/prefill_allocations.go](job/usecase/prefill_allocations.go): job-specific orchestration.
- [operation/usecase/get_operation.go](operation/usecase/get_operation.go), [operation/repository/operations.go](operation/repository/operations.go): operation reads and persistence.

## Change checks

- Preserve the PAT permission model and operation lease/checkpoint behavior; jobs must survive HTTP request completion and retain committed work on failure.
- Keep job-to-operation dependencies one-way. Schedule and Allocation own their business rules; external infrastructure owns invocation timing.

## Verification

- [Operation integration tests](../../../tests/integration/admin/operations_integration_test.go); Schedule and Allocation integration tests cover their job effects.

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
