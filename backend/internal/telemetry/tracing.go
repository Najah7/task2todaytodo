package telemetry

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/Najah7/task2todaytodo/internal/settings"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Setup installs W3C propagation and an SDK tracer provider.
func Setup(ctx context.Context, app settings.ApplicationSettings, logger settings.LoggerSettings) (*trace.TracerProvider, error) {
	identity, err := newResource(ctx, app, logger)
	if err != nil {
		return nil, err
	}
	sampler, err := samplerFromEnv()
	if err != nil {
		return nil, err
	}
	exporter, err := newSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("configure OpenTelemetry trace exporter: %w", err)
	}
	options := []trace.TracerProviderOption{
		trace.WithResource(identity),
		trace.WithSampler(sampler),
	}
	if exporter != nil {
		options = append(options, trace.WithBatcher(exporter))
	}
	provider := trace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return provider, nil
}

func newResource(ctx context.Context, app settings.ApplicationSettings, logger settings.LoggerSettings) (*resource.Resource, error) {
	attributes := make([]attribute.KeyValue, 0, len(logger.ResourceAttributes)+3)
	for key, value := range logger.ResourceAttributes {
		attributes = append(attributes, attribute.String(key, value))
	}
	attributes = append(attributes,
		attribute.String("service.name", app.Service),
		attribute.String("service.version", app.Version),
		attribute.String("deployment.environment.name", app.Environment),
	)
	identity, err := resource.New(ctx,
		resource.WithAttributes(attributes...),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("configure OpenTelemetry resource: %w", err)
	}
	return identity, nil
}

func newSpanExporter(ctx context.Context) (trace.SpanExporter, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_EXPORTER")))
	if name == "" {
		name = "otlp"
	}
	switch name {
	case "none":
		return nil, nil
	case "otlp":
	default:
		return nil, fmt.Errorf("unsupported OTEL_TRACES_EXPORTER; expected otlp or none")
	}
	protocol := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")))
	if protocol == "" {
		protocol = strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
	}
	if protocol == "" {
		protocol = "http/protobuf"
	}
	switch protocol {
	case "grpc":
		return otlptracegrpc.New(ctx)
	case "http/protobuf":
		return otlptracehttp.New(ctx)
	default:
		return nil, fmt.Errorf("unsupported OTLP trace protocol; expected grpc or http/protobuf")
	}
}

func samplerFromEnv() (trace.Sampler, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")))
	if name == "" {
		name = "parentbased_always_on"
	}
	switch name {
	case "always_on":
		return trace.AlwaysSample(), nil
	case "always_off":
		return trace.NeverSample(), nil
	case "parentbased_always_on":
		return trace.ParentBased(trace.AlwaysSample()), nil
	case "parentbased_always_off":
		return trace.ParentBased(trace.NeverSample()), nil
	case "traceidratio", "parentbased_traceidratio":
		arg := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
		if arg == "" {
			arg = "1.0"
		}
		ratio, err := strconv.ParseFloat(arg, 64)
		if err != nil || math.IsNaN(ratio) || ratio < 0 || ratio > 1 {
			return nil, fmt.Errorf("OTEL_TRACES_SAMPLER_ARG must be a number from 0 to 1")
		}
		delegate := trace.TraceIDRatioBased(ratio)
		if name == "parentbased_traceidratio" {
			return trace.ParentBased(delegate), nil
		}
		return delegate, nil
	default:
		return nil, fmt.Errorf("unsupported OTEL_TRACES_SAMPLER")
	}
}
