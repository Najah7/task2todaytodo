package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func NewRequestLogging(logger logging.Logger, ids shared.ID) func(http.Handler) http.Handler {
	logger = logging.OrNop(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := incomingRequestID(r.Header.Get("X-Request-ID"))
			if requestID == "" {
				requestID = ids.Generate()
			}
			w.Header().Set("X-Request-ID", requestID)
			ctx := logging.WithRequestID(r.Context(), requestID)
			r = r.WithContext(ctx)
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			started := time.Now()
			logger.Info(ctx, "request started", "http", map[string]any{"method": r.Method})

			var panicType string
			var panicValue any
			rethrowPanic := false
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						panicValue = recovered
						panicType = fmt.Sprintf("%T", recovered)
						span := trace.SpanFromContext(ctx)
						span.SetStatus(codes.Error, "")
						span.SetAttributes(attribute.String("error.type", panicType))
						committed := wrapped.Status() != 0
						if recovered != http.ErrAbortHandler && wrapped.Status() == 0 {
							rest.WriteError(wrapped, http.StatusInternalServerError, rest.ErrSpec{
								Code:    rest.DetailInternalErrorCode,
								Message: rest.DetailInternalErrorMsg,
							})
						}
						rethrowPanic = recovered == http.ErrAbortHandler || committed
					}
				}()
				next.ServeHTTP(wrapped, r)
			}()

			path := requestPath(r)
			status := wrapped.Status()
			aborted := panicValue == http.ErrAbortHandler
			if status == 0 && !aborted {
				status = http.StatusOK
			}
			if path != "unmatched" {
				span := trace.SpanFromContext(ctx)
				span.SetName(r.Method + " " + path)
				span.SetAttributes(attribute.String("http.route", path))
			}
			attrs := []any{"http", map[string]any{"method": r.Method, "path": path, "status_code": status}, "duration_ms", float64(time.Since(started).Nanoseconds()) / float64(time.Millisecond)}
			if panicType != "" {
				attrs = append(attrs, "panic_type", panicType)
			}
			if aborted {
				attrs = append(attrs, "aborted", true)
			}
			switch {
			case status >= http.StatusInternalServerError || panicType != "" || aborted:
				logger.Error(ctx, "request completed", attrs...)
			case status >= http.StatusBadRequest:
				logger.Warn(ctx, "request completed", attrs...)
			default:
				logger.Info(ctx, "request completed", attrs...)
			}
			if rethrowPanic {
				if aborted {
					panic(panicValue)
				}
				panic(http.ErrAbortHandler)
			}
		})
	}
}

func incomingRequestID(value string) string {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return ""
	}
	id, err := uuid.Parse(value)
	if err != nil || id.String() != strings.ToLower(value) {
		return ""
	}
	return id.String()
}

func requestPath(r *http.Request) string {
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}
