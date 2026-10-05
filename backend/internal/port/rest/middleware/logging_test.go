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
	"github.com/go-chi/chi/v5"
)

type fixedID string

func (id fixedID) Generate() string { return string(id) }

type testLogger struct{ output *bytes.Buffer }

func (l testLogger) Debug(ctx context.Context, msg string, args ...any) {
	logging.New(l.output, logging.Config{Level: -4}).Debug(ctx, msg, args...)
}
func (l testLogger) Info(ctx context.Context, msg string, args ...any) {
	logging.New(l.output, logging.Config{Level: -4}).Info(ctx, msg, args...)
}
func (l testLogger) Warn(ctx context.Context, msg string, args ...any) {
	logging.New(l.output, logging.Config{Level: -4}).Warn(ctx, msg, args...)
}
func (l testLogger) Error(ctx context.Context, msg string, args ...any) {
	logging.New(l.output, logging.Config{Level: -4}).Error(ctx, msg, args...)
}

func TestRequestLoggingIDsStatusAndSafePath(t *testing.T) {
	tests := []struct {
		name, inbound, wantID string
		status                int
		level                 string
	}{
		{name: "valid", inbound: "550E8400-E29B-41D4-A716-446655440000", wantID: "550e8400-e29b-41d4-a716-446655440000", status: 200, level: "INFO"},
		{name: "invalid", inbound: "not-an-id", wantID: "123e4567-e89b-12d3-a456-426614174000", status: 404, level: "WARN"},
		{name: "absent", wantID: "123e4567-e89b-12d3-a456-426614174000", status: 503, level: "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			handler := NewRequestLogging(testLogger{&output}, fixedID("123e4567-e89b-12d3-a456-426614174000"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if logging.RequestID(r.Context()) != tt.wantID {
					t.Errorf("context request ID = %q", logging.RequestID(r.Context()))
				}
				if tt.status != 200 {
					w.WriteHeader(tt.status)
				}
			}))
			req := httptest.NewRequest(http.MethodGet, "/private/path?token=must-not-log&password=secret", nil)
			if tt.inbound != "" {
				req.Header.Set("X-Request-ID", tt.inbound)
			}
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if got := res.Header().Get("X-Request-ID"); got != tt.wantID {
				t.Fatalf("response request ID = %q", got)
			}
			logs := output.String()
			if strings.Contains(logs, "must-not-log") || strings.Contains(logs, "password") || strings.Contains(logs, "secret") || strings.Contains(logs, "token=") {
				t.Fatalf("sensitive URL data logged: %s", logs)
			}
			lines := strings.Split(strings.TrimSpace(logs), "\n")
			if len(lines) != 2 {
				t.Fatalf("got %d log lines: %s", len(lines), logs)
			}
			var completed map[string]any
			var started map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &started); err != nil {
				t.Fatal(err)
			}
			startHTTP := started["http"].(map[string]any)
			if len(startHTTP) != 1 || startHTTP["method"] != http.MethodGet {
				t.Fatalf("start http attrs = %#v", startHTTP)
			}
			if err := json.Unmarshal([]byte(lines[1]), &completed); err != nil {
				t.Fatal(err)
			}
			if completed["level"] != tt.level || completed["request_id"] != tt.wantID {
				t.Fatalf("completion log = %#v", completed)
			}
			httpAttrs := completed["http"].(map[string]any)
			if httpAttrs["status_code"] != float64(tt.status) || httpAttrs["path"] != "unmatched" {
				t.Fatalf("http attrs = %#v", httpAttrs)
			}
			if _, ok := completed["duration_ms"].(float64); !ok {
				t.Fatalf("duration_ms missing or not numeric: %#v", completed)
			}
		})
	}
}

func TestRequestLoggingPanicBeforeAndAfterCommit(t *testing.T) {
	for _, committed := range []bool{false, true} {
		var output bytes.Buffer
		handler := NewRequestLogging(testLogger{&output}, fixedID("123e4567-e89b-12d3-a456-426614174000"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if committed {
				_, _ = w.Write([]byte("already sent"))
			}
			panic(errors.New("secret panic payload"))
		}))
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
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
		}()
		if committed {
			if res.Code != http.StatusOK || res.Body.String() != "already sent" {
				t.Fatalf("committed response changed: %d %q", res.Code, res.Body.String())
			}
		} else if res.Code != http.StatusInternalServerError || !strings.Contains(res.Header().Get("Content-Type"), "application/json") || strings.Contains(res.Body.String(), "secret panic payload") {
			t.Fatalf("panic response = %d, %q", res.Code, res.Body.String())
		}
		if strings.Contains(output.String(), "secret panic payload") || !strings.Contains(output.String(), "panic_type") {
			t.Fatalf("panic logging unsafe or missing: %s", output.String())
		}
	}
}

func TestRequestLoggingPreservesFlusher(t *testing.T) {
	flushed := false
	handler := NewRequestLogging(testLogger{&bytes.Buffer{}}, fixedID("123e4567-e89b-12d3-a456-426614174000"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("http.Flusher interface not preserved")
			return
		}
		flusher.Flush()
		flushed = true
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !flushed {
		t.Fatal("response was not flushed")
	}
}

func TestRequestLoggingRethrowsAbortHandler(t *testing.T) {
	var output bytes.Buffer
	handler := NewRequestLogging(testLogger{&output}, fixedID("123e4567-e89b-12d3-a456-426614174000"))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("http.ErrAbortHandler was not rethrown")
		}
		var completed map[string]any
		line := strings.Split(strings.TrimSpace(output.String()), "\n")[1]
		if err := json.Unmarshal([]byte(line), &completed); err != nil {
			t.Fatal(err)
		}
		httpAttrs := completed["http"].(map[string]any)
		if httpAttrs["status_code"] != float64(0) || completed["aborted"] != true || completed["level"] != "ERROR" {
			t.Fatalf("abort completion = %#v", completed)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRequestLoggingUsesResolvedChiRouteForCompletion(t *testing.T) {
	var output bytes.Buffer
	router := chi.NewRouter()
	router.Use(NewRequestLogging(testLogger{&output}, fixedID("123e4567-e89b-12d3-a456-426614174000")))
	router.Get("/items/{itemID}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/secret-id", nil))
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	var completed map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &completed); err != nil {
		t.Fatal(err)
	}
	httpAttrs := completed["http"].(map[string]any)
	if httpAttrs["path"] != "/items/{itemID}" {
		t.Fatalf("completion path = %v", httpAttrs["path"])
	}
}
