package telemetry

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/settings"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	apitrace "go.opentelemetry.io/otel/trace"
	collecttrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestSetupUsesResolvedMetadataInResource(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_SERVICE_NAME", "process-env-api")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=process-env-ignored,deployment.environment.name=process-env,service.version=0.0.0,other=ignored")
	identity, err := newResource(context.Background(), settings.ApplicationSettings{
		Service: "configured-api", Environment: "production", Version: "9.8.7",
	}, settings.LoggerSettings{ResourceAttributes: map[string]string{
		"service.name": "ignored", "deployment.environment.name": "ignored", "service.version": "ignored",
		"telemetry.sdk.name": "ignored", "team": "platform",
	}})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	attrs := resourceAttributes(identity)
	want := map[string]string{
		"service.name":                "configured-api",
		"deployment.environment.name": "production",
		"service.version":             "9.8.7",
		"team":                        "platform",
	}
	for key, value := range want {
		if attrs[key] != value {
			t.Errorf("resource %s = %q, want %q", key, attrs[key], value)
		}
	}
	if attrs["telemetry.sdk.name"] == "ignored" {
		t.Fatal("application resource attributes overrode telemetry SDK metadata")
	}
	if _, exists := attrs["other"]; exists {
		t.Fatal("resource metadata was reread from process environment")
	}
}

func TestSetupDefaultsMetadataAndNoExporterShutdown(t *testing.T) {
	restoreOpenTelemetryGlobals(t)
	setDefaultSamplerEnv(t)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
	cfg := loadConfigWithDefaults(t)
	provider, err := Setup(context.Background(), cfg.Application, cfg.Logger)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	shutdown(t, provider)
}

func TestHTTPHandlerExtractsW3CTraceContext(t *testing.T) {
	restoreOpenTelemetryGlobals(t)
	setDefaultSamplerEnv(t)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
	cfg := loadConfigWithDefaults(t)
	provider, err := Setup(context.Background(), cfg.Application, cfg.Logger)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer func() {
		shutdown(t, provider)
	}()
	wantID := "4bf92f3577b34da6a3ce929d0e0e4736"
	wantTraceID, _ := apitrace.TraceIDFromHex(wantID)
	called := false
	handler := otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		span := apitrace.SpanFromContext(r.Context())
		if !span.IsRecording() {
			t.Error("request span is not recording")
		}
		if got := span.SpanContext().TraceID(); got != wantTraceID {
			t.Errorf("trace ID = %s, want %s", got, wantTraceID)
		}
		w.WriteHeader(http.StatusNoContent)
	}), "test-api")
	req := httptest.NewRequest(http.MethodGet, "/monitor/health", nil)
	req.Header.Set("traceparent", "00-"+wantID+"-00f067aa0ba902b7-01")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if !called || res.Code != http.StatusNoContent {
		t.Fatalf("handler called = %v, response status = %d", called, res.Code)
	}
}

func TestOTLPHTTPExporterSendsProtobufTrace(t *testing.T) {
	restoreOpenTelemetryGlobals(t)
	setDefaultSamplerEnv(t)
	type receivedExport struct {
		body        []byte
		contentType string
		token       string
	}
	requests := make(chan receivedExport, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- receivedExport{
			body: body, contentType: r.Header.Get("Content-Type"), token: r.Header.Get("x-test-token"),
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write([]byte{})
	}))
	defer server.Close()
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", server.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS", "x-test-token=trace-test")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_TIMEOUT", "2000")
	t.Setenv("OTEL_EXPORTER_OTLP_TIMEOUT", "2000")
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS",
		"OTEL_EXPORTER_OTLP_COMPRESSION", "OTEL_EXPORTER_OTLP_INSECURE",
		"OTEL_EXPORTER_OTLP_CERTIFICATE", "OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE",
		"OTEL_EXPORTER_OTLP_CLIENT_KEY", "OTEL_EXPORTER_OTLP_TRACES_COMPRESSION",
		"OTEL_EXPORTER_OTLP_TRACES_INSECURE", "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE",
		"OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE", "OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_SERVICE_NAME", "payload-api")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment.name=testing,service.version=5.6.7")
	provider, err := Setup(context.Background(), settings.ApplicationSettings{
		Service: "payload-api", Environment: "testing", Version: "5.6.7",
	}, settings.LoggerSettings{ResourceAttributes: map[string]string{
		"deployment.environment.name": "ignored",
		"service.version":             "ignored",
	}})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	_, span := provider.Tracer("telemetry_test").Start(context.Background(), "exported-span")
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	var received receivedExport
	select {
	case received = <-requests:
	case <-time.After(time.Second):
		t.Fatal("OTLP exporter made no HTTP request")
	}
	if received.contentType != "application/x-protobuf" || len(received.body) == 0 || received.token != "trace-test" {
		t.Fatalf("export request content type=%q body length=%d token=%q", received.contentType, len(received.body), received.token)
	}
	var payload collecttrace.ExportTraceServiceRequest
	if err := proto.Unmarshal(received.body, &payload); err != nil {
		t.Fatalf("decode OTLP protobuf payload: %v", err)
	}
	if len(payload.ResourceSpans) != 1 || len(payload.ResourceSpans[0].ScopeSpans) == 0 {
		t.Fatalf("unexpected resource spans: %+v", payload.ResourceSpans)
	}
	resource := payload.ResourceSpans[0].Resource
	resourceValues := map[string]string{}
	for _, kv := range resource.Attributes {
		if value := kv.GetValue().GetStringValue(); value != "" {
			resourceValues[kv.GetKey()] = value
		}
	}
	for key, want := range map[string]string{
		"service.name":                "payload-api",
		"deployment.environment.name": "testing",
		"service.version":             "5.6.7",
	} {
		if resourceValues[key] != want {
			t.Errorf("export resource %s = %q, want %q", key, resourceValues[key], want)
		}
	}
	spans := payload.ResourceSpans[0].ScopeSpans[0].Spans
	if len(spans) != 1 || spans[0].Name != "exported-span" {
		t.Fatalf("exported spans = %+v", spans)
	}
}

func TestSamplerFromEnvRatioDefaultsAndValidation(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "traceidratio")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "")
	sampler, err := samplerFromEnv()
	if err != nil {
		t.Fatalf("samplerFromEnv() error = %v", err)
	}
	if got, want := sampler.Description(), sdktrace.TraceIDRatioBased(1).Description(); got != want {
		t.Errorf("sampler description = %q, want %q", got, want)
	}

	for _, value := range []string{"NaN", "-0.1", "1.01", "invalid"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", value)
			if _, err := samplerFromEnv(); err == nil {
				t.Errorf("samplerFromEnv(%q) succeeded, want validation error", value)
			}
		})
	}
}

func resourceAttributes(resource *resource.Resource) map[string]string {
	attrs := make(map[string]string)
	for _, kv := range resource.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	return attrs
}

func loadConfigWithDefaults(t *testing.T) settings.Settings {
	t.Helper()
	encodedKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("PAGE_TOKEN_KEY", encodedKey)
	cfg, err := settings.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}

func shutdown(t *testing.T, provider *sdktrace.TracerProvider) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func restoreOpenTelemetryGlobals(t *testing.T) {
	t.Helper()
	provider := otel.GetTracerProvider()
	propagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
	})
}

func setDefaultSamplerEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OTEL_TRACES_SAMPLER", "parentbased_always_on")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "")
}
