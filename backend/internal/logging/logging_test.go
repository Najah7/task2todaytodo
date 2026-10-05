package logging

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestLoggerWritesMetadataAndCorrelation(t *testing.T) {
	var output strings.Builder
	logger := New(&output, Config{Service: "api", Environment: "test", Version: "v1", Level: -4})
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3}, SpanID: trace.SpanID{4, 5}, TraceFlags: trace.FlagsSampled,
	})
	ctx := WithRequestID(trace.ContextWithSpanContext(context.Background(), spanContext), "request-123")
	logger.Info(ctx, "started", "answer", 42)

	var record map[string]any
	if err := json.Unmarshal([]byte(output.String()), &record); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"service": "api", "environment": "test", "version": "v1", "message": "started",
		"request_id": "request-123", "trace_id": spanContext.TraceID().String(), "span_id": spanContext.SpanID().String(), "trace_flags": "01",
	} {
		if got := record[key]; got != want {
			t.Errorf("%s = %v, want %q", key, got, want)
		}
	}
	if _, exists := record["time"]; exists {
		t.Fatal("default time key leaked")
	}
	if _, exists := record["timestamp"]; !exists {
		t.Fatal("timestamp missing")
	}
	if record["answer"] != float64(42) {
		t.Fatalf("answer = %v", record["answer"])
	}
}

func TestLoggerLevelFilteringAndMissingCorrelation(t *testing.T) {
	var output strings.Builder
	logger := New(&output, Config{Level: 0})
	logger.Debug(context.Background(), "hidden")
	logger.Info(context.Background(), "visible")
	if strings.Contains(output.String(), "hidden") || !strings.Contains(output.String(), "visible") {
		t.Fatalf("unexpected output: %s", output.String())
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(output.String()), &record); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"request_id", "trace_id", "span_id", "trace_flags"} {
		if _, exists := record[key]; exists {
			t.Errorf("unexpected %s in record", key)
		}
	}
}

type stateError struct{ state string }

func (e stateError) Error() string    { return "sensitive database detail" }
func (e stateError) SQLState() string { return e.state }

func TestErrorHelpers(t *testing.T) {
	err := errors.New("sensitive database detail")
	fields := ErrorFields(stateError{state: "23505"})
	if len(fields) != 4 || fields[0] != "error_type" || fields[2] != "sqlstate" || fields[3] != "23505" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
	if strings.Contains(strings.Join([]string{fields[1].(string)}, " "), "sensitive") {
		t.Fatal("error detail leaked")
	}
	if !IsRoutineError(context.Canceled) || !IsRoutineError(stateError{state: "23503"}) || IsRoutineError(err) {
		t.Fatal("routine error classification incorrect")
	}
	if !IsNotFound(fmt.Errorf("wrapped lookup: %w", sql.ErrNoRows)) || IsNotFound(err) {
		t.Fatal("not found error classification incorrect")
	}
}
