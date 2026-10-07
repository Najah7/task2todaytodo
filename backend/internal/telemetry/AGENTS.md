# Telemetry guide

These rules cover OpenTelemetry trace setup, instrumentation, propagation, and
lifecycle across the backend. Consult this guide when changing traces in any
layer, including `cmd/api`, REST, and middleware. See the
[backend guide](../../AGENTS.md) for commands and the
[logging guide](../logging/AGENTS.md) for log behavior and trace-to-log
correlation.

## SDK, resource, and propagation

- `internal/telemetry` owns official OpenTelemetry SDK setup, shared resource
  metadata, propagation, sampling, and trace exporter configuration.
- Resolve service metadata once for logs and trace resources. Defaults are
  `OTEL_SERVICE_NAME=api`, `ENVIRONMENT=development`, and `APP_VERSION=1.0.0`.
  OpenTelemetry resource attributes may override environment and version;
  `OTEL_SERVICE_NAME` takes precedence over `service.name` in
  `OTEL_RESOURCE_ATTRIBUTES`.
- Use W3C Trace Context and Baggage propagation. Instrument HTTP server spans
  with official `otelhttp`; obtain span context from OpenTelemetry. Do not parse
  `traceparent` or generate trace IDs manually.
- Sampling defaults to `parentbased_always_on`. `OTEL_TRACES_SAMPLER` and
  `OTEL_TRACES_SAMPLER_ARG` select another supported sampler or ratio.

## Export and shutdown

- Configure OTLP export with standard `OTEL_*` environment variables. Support
  HTTP/protobuf and gRPC, plus standard endpoint, headers, timeout, compression,
  and TLS settings. If `OTEL_TRACES_EXPORTER` is unset, it defaults to `otlp`;
  the default protocol is `http/protobuf`. Set
  `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` to select gRPC.
- Local `.env.example` sets `OTEL_TRACES_EXPORTER=none`. `none` disables trace
  export while retaining SDK span context. The repository does not run an OTLP
  Collector; configure an external Collector when exporting traces.
- After HTTP shutdown, flush and shut down the trace provider with bounded
  contexts.
