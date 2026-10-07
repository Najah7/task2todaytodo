# Task2TodayToDo

[日本語](README.ja.md)

***Don’t live for tomorrow. Give today everything you’ve got.***

task2todaytodo manages Projects, Tasks, TodoItems, and independent Schedules, then turns them into a daily execution plan.

The core problem is that planning work across a week or month is easy to start but hard to maintain. Interruptions create drift, and manually repairing a long-term calendar quickly becomes too much work. task2todaytodo avoids that burden by letting users manage Projects, Tasks, TodoItems, and Schedules together in a structured source of truth, then generating only today's TodoList when it is needed.

By separating long-lived task and schedule management from day-by-day execution, the product can create dynamic, flexible TodoLists for the day without forcing users to maintain a perfect long-term schedule.

# Values

- Task Management: manage Tasks and their TodoItems, including repeatable and one-off work.
- Schedule Management: manage fixed-time Schedules independently or within Projects, including repeatable and one-off work. Future integrations can synchronize them with calendar systems such as Google Calendar.
- Generate Today's TodoList: generate a TodoList as the execution plan for the day, based on managed tasks and schedules.

## Backend quick start

Prerequisites: Docker, Go, Air.

```bash
cd backend
cp .env.example .env # first setup only
openssl rand -base64 32 # set output as PAGE_TOKEN_KEY in backend/.env
make env-up
make migrate-up
air
```

API docs: <http://localhost:8080/swagger/index.html>

Keep `PAGE_TOKEN_KEY` stable across API instances. Changing it invalidates existing page tokens for up to 24 hours.

Check Useful backend commands:

```bash
make help        # show help
```

### Logs and traces

The API writes structured JSON logs to stdout. Set `LOG_LEVEL` to `DEBUG`,
`INFO`, `WARN`, or `ERROR`. Every HTTP request log includes a request ID;
valid inbound UUID request IDs are retained and invalid or missing IDs are
replaced with generated UUIDs.
Request logs include `trace_id`, `span_id`, and `trace_flags` when an active
trace span is present.

The API creates OpenTelemetry server spans and propagates W3C Trace Context.
Trace export defaults to OTLP (`OTEL_TRACES_EXPORTER=otlp`) with
`http/protobuf`; `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` selects gRPC. The API
honors standard OTLP endpoint, headers, timeout, compression, and TLS
environment settings. Configure an external Collector endpoint, for example
`OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4318` for a Collector
reachable from the Air container. The repository does not start a Collector.
Sampling defaults to `parentbased_always_on`; use `OTEL_TRACES_SAMPLER` and
`OTEL_TRACES_SAMPLER_ARG` to select another supported SDK sampler or ratio.

Local `.env.example` defaults to `OTEL_TRACES_EXPORTER=none`; set it to `otlp`
when a Collector is available. `OTEL_SERVICE_NAME` defaults to `api`,
`ENVIRONMENT` to `development`, and `APP_VERSION` to `1.0.0`. OpenTelemetry
resource attributes can override environment and version metadata; service
name follows the standard `OTEL_SERVICE_NAME` precedence. The resolved values
are shared by logs and trace resources.

For PostgreSQL diagnostics, use the PostgreSQL server's container logs. Keep
SQL parameter values and credentials out of application logs. A Collector can
route API traces and PostgreSQL server logs to your chosen backend.
