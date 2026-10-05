package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/logging"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const integrationRequestID = "123e4567-e89b-12d3-a456-426614174000"

func traceFixture(t *testing.T, output *bytes.Buffer, configure func(*chi.Mux, *sdktrace.TracerProvider, logging.Logger)) (http.Handler, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	logger := logging.New(output, logging.Config{Service: "test", Level: -4})
	router := chi.NewRouter()
	router.Use(NewRequestLogging(logger, fixedID(integrationRequestID)))
	configure(router, provider, logger)
	return otelhttp.NewHandler(
		router,
		"api",
		otelhttp.WithTracerProvider(provider),
		otelhttp.WithPropagators(propagation.TraceContext{}),
	), recorder
}

func TestRequestLoggingTraceCorrelationAcrossLevels(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		level string
	}{
		{name: "info", code: http.StatusOK, level: "INFO"},
		{name: "warn", code: http.StatusNotFound, level: "WARN"},
		{name: "error", code: http.StatusInternalServerError, level: "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			handler, recorder := traceFixture(t, &output, func(router *chi.Mux, provider *sdktrace.TracerProvider, logger logging.Logger) {
				router.Get("/items/{itemID}", func(w http.ResponseWriter, r *http.Request) {
					childCtx, child := provider.Tracer("usecase").Start(r.Context(), "usecase")
					logger.Info(childCtx, "usecase event")
					child.End()
					w.WriteHeader(tt.code)
				})
			})
			req := httptest.NewRequest(http.MethodGet, "/items/42?token=private", nil)
			req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
			req.Header.Set("X-Request-ID", integrationRequestID)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			if res.Header().Get("X-Request-ID") != integrationRequestID {
				t.Fatalf("response request ID = %q", res.Header().Get("X-Request-ID"))
			}
			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			if len(lines) != 3 {
				t.Fatalf("got %d log records: %s", len(lines), output.String())
			}
			var records []map[string]any
			for _, line := range lines {
				var record map[string]any
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}
				records = append(records, record)
			}
			if records[0]["message"] != "request started" || records[1]["message"] != "usecase event" || records[2]["message"] != "request completed" {
				t.Fatalf("unexpected log sequence: %#v", records)
			}
			completionHTTP := records[2]["http"].(map[string]any)
			if records[2]["level"] != tt.level || completionHTTP["path"] != "/items/{itemID}" || completionHTTP["status_code"] != float64(tt.code) {
				t.Fatalf("completion = %#v", records[2])
			}
			if records[0]["request_id"] != integrationRequestID || records[1]["request_id"] != integrationRequestID || records[2]["request_id"] != integrationRequestID {
				t.Fatalf("request ID missing from correlated logs: %#v", records)
			}
			if strings.Contains(output.String(), "private") || strings.Contains(output.String(), "token=") || strings.Contains(output.String(), "/items/42") {
				t.Fatalf("sensitive or untemplated URL logged: %s", output.String())
			}

			spans := tracetest.SpanStubsFromReadOnlySpans(recorder.Ended())
			var server, child *tracetest.SpanStub
			for i := range spans {
				span := &spans[i]
				if span.SpanKind == trace.SpanKindServer {
					server = span
				} else if span.Name == "usecase" {
					child = span
				}
			}
			if server == nil || child == nil {
				t.Fatalf("expected server and usecase spans, got %#v", spans)
			}
			if server.Name != "GET /items/{itemID}" || stringAttribute(server.Attributes, "http.route") != "/items/{itemID}" {
				t.Fatalf("server route metadata: name=%q attrs=%#v", server.Name, server.Attributes)
			}
			if server.SpanContext.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" || server.SpanContext.SpanID().String() == "00f067aa0ba902b7" {
				t.Fatalf("server span context = %s/%s", server.SpanContext.TraceID(), server.SpanContext.SpanID())
			}
			if child.SpanContext.TraceID() != server.SpanContext.TraceID() || child.SpanContext.SpanID().String() == server.SpanContext.SpanID().String() {
				t.Fatalf("child span did not inherit trace or get new span ID: server=%v child=%v", server.SpanContext, child.SpanContext)
			}
			if records[0]["trace_id"] != server.SpanContext.TraceID().String() || records[2]["span_id"] != server.SpanContext.SpanID().String() || records[1]["span_id"] != child.SpanContext.SpanID().String() {
				t.Fatalf("log/span correlation mismatch: %#v server=%v child=%v", records, server.SpanContext, child.SpanContext)
			}
		})
	}
}

func TestRequestLoggingUnmatchedPathNeverLeaks(t *testing.T) {
	var output bytes.Buffer
	handler, recorder := traceFixture(t, &output, func(router *chi.Mux, _ *sdktrace.TracerProvider, logger logging.Logger) {
		notFound := NewRequestLogging(logger, fixedID(integrationRequestID))(http.NotFoundHandler())
		router.NotFound(notFound.ServeHTTP)
	})
	req := httptest.NewRequest(http.MethodGet, "/missing/secret-path?password=private&token=secret", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if strings.Contains(output.String(), "secret-path") || strings.Contains(output.String(), "password") || strings.Contains(output.String(), "private") || strings.Contains(output.String(), "token=") {
		t.Fatalf("unmatched URL leaked into logs: %s", output.String())
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected start and completion logs, got: %s", output.String())
	}
	var start, completion map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &start)
	_ = json.Unmarshal([]byte(lines[1]), &completion)
	startHTTP := start["http"].(map[string]any)
	completionHTTP := completion["http"].(map[string]any)
	if len(startHTTP) != 1 || startHTTP["method"] != http.MethodGet || completionHTTP["path"] != "unmatched" {
		t.Fatalf("unmatched route logging: start=%#v complete=%#v", start, completion)
	}
	for _, span := range tracetest.SpanStubsFromReadOnlySpans(recorder.Ended()) {
		if stringAttribute(span.Attributes, "http.route") != "" {
			t.Fatalf("unmatched path added route span attribute: %#v", span.Attributes)
		}
	}
}

func TestRequestLoggingGeneratedTraceAndUnsampledParent(t *testing.T) {
	tests := []struct {
		name, parent string
		exported     bool
	}{
		{name: "generated", exported: true},
		{name: "unsampled", parent: "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			handler, recorder := traceFixture(t, &output, func(router *chi.Mux, _ *sdktrace.TracerProvider, _ logging.Logger) {
				router.Get("/work", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			})
			req := httptest.NewRequest(http.MethodGet, "/work", nil)
			if tt.parent != "" {
				req.Header.Set("traceparent", tt.parent)
			}
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Header().Get("X-Request-ID") != integrationRequestID {
				t.Fatalf("response request ID = %q", res.Header().Get("X-Request-ID"))
			}
			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			var start, completion map[string]any
			_ = json.Unmarshal([]byte(lines[0]), &start)
			_ = json.Unmarshal([]byte(lines[1]), &completion)
			traceID := start["trace_id"]
			spanID := start["span_id"]
			if traceID == nil || spanID == nil || completion["trace_id"] != traceID || completion["span_id"] != spanID {
				t.Fatalf("trace IDs missing or inconsistent: start=%#v completion=%#v", start, completion)
			}
			if tt.parent != "" && traceID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
				t.Fatalf("trace ID = %v, did not retain incoming trace ID", traceID)
			}
			if tt.parent == "" && traceID == "00000000000000000000000000000000" {
				t.Fatal("generated trace ID is invalid")
			}
			if tt.exported {
				if got := len(recorder.Ended()); got != 1 {
					t.Fatalf("ended spans = %d, want server span", got)
				}
			} else if got := len(recorder.Ended()); got != 0 {
				t.Fatalf("unsampled request exported %d spans", got)
			} else if completion["trace_flags"] != "00" {
				t.Fatalf("unsampled trace flags = %v", completion["trace_flags"])
			}
		})
	}
}

func TestRequestLoggingPanicMarksExportedSpanAndErrorResponse(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncommitted", true: "committed"}[committed], func(t *testing.T) {
			var output bytes.Buffer
			handler, recorder := traceFixture(t, &output, func(router *chi.Mux, _ *sdktrace.TracerProvider, _ logging.Logger) {
				router.Get("/panic/{mode}", func(w http.ResponseWriter, r *http.Request) {
					if chi.URLParam(r, "mode") == "committed" {
						_, _ = w.Write([]byte("committed"))
					}
					panic(errors.New("private panic payload"))
				})
			})
			req := httptest.NewRequest(http.MethodGet, "/panic/"+map[bool]string{false: "uncommitted", true: "committed"}[committed], nil)
			res := httptest.NewRecorder()
			func() {
				defer func() {
					recovered := recover()
					if committed && recovered != http.ErrAbortHandler {
						t.Fatalf("committed panic = %v, want http.ErrAbortHandler", recovered)
					}
					if !committed && recovered != nil {
						t.Fatalf("uncommitted panic rethrown: %v", recovered)
					}
				}()
				handler.ServeHTTP(res, req)
			}()
			if committed {
				if res.Code != http.StatusOK || res.Body.String() != "committed" {
					t.Fatalf("committed response changed: %d %q", res.Code, res.Body.String())
				}
			} else {
				if res.Code != http.StatusInternalServerError || res.Header().Get("X-Request-ID") != integrationRequestID {
					t.Fatalf("panic response = %d headers=%v", res.Code, res.Header())
				}
				var payload rest.ErrResponse
				if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if payload.Error.RequestID != res.Header().Get("X-Request-ID") {
					t.Fatalf("error response request ID = %q", payload.Error.RequestID)
				}
			}
			if strings.Contains(output.String(), "private panic payload") || strings.Contains(res.Body.String(), "private panic payload") {
				t.Fatalf("panic text leaked: logs=%s body=%s", output.String(), res.Body.String())
			}
			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			var completed map[string]any
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &completed); err != nil {
				t.Fatal(err)
			}
			if completed["request_id"] != res.Header().Get("X-Request-ID") || completed["level"] != "ERROR" {
				t.Fatalf("panic completion correlation: %#v headers=%v", completed, res.Header())
			}
			spans := tracetest.SpanStubsFromReadOnlySpans(recorder.Ended())
			var server *tracetest.SpanStub
			for i := range spans {
				if spans[i].SpanKind == trace.SpanKindServer {
					server = &spans[i]
				}
			}
			if server == nil || server.Status.Code != codes.Error {
				t.Fatalf("panic server span not marked error: %#v", server)
			}
			if got := stringAttribute(server.Attributes, "error.type"); got == "" || strings.Contains(got, "private") {
				t.Fatalf("unsafe or missing error.type attribute: %q", got)
			}
		})
	}
}

func stringAttribute(attrs []attribute.KeyValue, key string) string {
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return attr.Value.AsString()
		}
	}
	return ""
}
