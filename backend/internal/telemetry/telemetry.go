package telemetry

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Metadata identifies this API instance in both logs and telemetry.
type Metadata struct {
	Service     string
	Environment string
	Version     string
}

// Setup installs W3C propagation and an SDK tracer provider. Metadata from
// standard OpenTelemetry resource environment variables overrides app defaults.
func Setup(ctx context.Context, defaults Metadata) (*trace.TracerProvider, Metadata, error) {
	identity, metadata, err := newResource(ctx, defaults)
	if err != nil {
		return nil, Metadata{}, err
	}
	sampler, err := samplerFromEnv()
	if err != nil {
		return nil, Metadata{}, err
	}
	exporter, err := newSpanExporter(ctx)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("configure OpenTelemetry trace exporter: %w", err)
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
	return provider, metadata, nil
}

func newResource(ctx context.Context, defaults Metadata) (*resource.Resource, Metadata, error) {
	resourceFromEnv, err := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK())
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("read OpenTelemetry resource environment: %w", err)
	}
	metadata := resolveMetadata(defaults, resourceFromEnv)
	identity, err := resource.Merge(resourceFromEnv, resource.NewWithAttributes(
		"",
		attribute.String("service.name", metadata.Service),
		attribute.String("service.version", metadata.Version),
		attribute.String("deployment.environment.name", metadata.Environment),
	))
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("configure OpenTelemetry resource: %w", err)
	}
	return identity, metadata, nil
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

func resolveMetadata(defaults Metadata, env *resource.Resource) Metadata {
	metadata := defaults
	if metadata.Service == "" {
		metadata.Service = "api"
	}
	if metadata.Environment == "" {
		metadata.Environment = "development"
	}
	if metadata.Version == "" {
		metadata.Version = "1.0.0"
	}
	for _, kv := range env.Attributes() {
		switch string(kv.Key) {
		case "service.name":
			if value := kv.Value.AsString(); value != "" {
				metadata.Service = value
			}
		case "deployment.environment.name":
			if value := kv.Value.AsString(); value != "" {
				metadata.Environment = value
			}
		case "service.version":
			if value := kv.Value.AsString(); value != "" {
				metadata.Version = value
			}
		}
	}
	// OTEL_SERVICE_NAME is defined to take precedence over service.name in
	// OTEL_RESOURCE_ATTRIBUTES.
	if service := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); service != "" {
		metadata.Service = service
	}
	return metadata
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
