package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Najah7/task2todaytodo/docs"
	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/config"
	"github.com/Najah7/task2todaytodo/internal/logging"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/port/rest/middleware"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

// @title						task2todaytodo API
// @version					1.0
// @description				Task scheduling API server.
// @BasePath					/api
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	conf, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, append([]any{"API initialization failed", "operation", "load_configuration"}, logging.ErrorFields(err)...)...)
		os.Exit(1)
	}
	provider, metadata, err := telemetry.Setup(context.Background(), telemetry.Metadata{
		Service:     conf.Application.Service,
		Environment: conf.Application.Environment,
		Version:     conf.Application.Version,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, append([]any{"API initialization failed", "operation", "configure_telemetry"}, logging.ErrorFields(err)...)...)
		os.Exit(1)
	}
	logger := logging.New(os.Stdout, logging.Config{
		Service:     metadata.Service,
		Environment: metadata.Environment,
		Version:     metadata.Version,
		Level:       conf.Application.LogLevel,
	})
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error(context.Background(), "OpenTelemetry error", logging.ErrorFields(err)...)
	}))
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	ids := adapter.NewULID()
	app, closeApp, err := application.New(conf.Database, ids, logger)
	if err != nil {
		logger.Error(context.Background(), "API initialization failed", append([]any{"operation", "initialize_application"}, logging.ErrorFields(err)...)...)
		stop()
		telemetry.Shutdown(provider, logger)
		os.Exit(1)
	}
	r := chi.NewRouter()
	r.Use(middleware.StripTrailingSlash)
	r.Use(middleware.NewRequestLogging(logger, adapter.NewUUID()))

	handler, err := rest.NewHandler(app.UseCase, ids, conf.Secrets.PageTokenKey)
	if err != nil {
		logger.Error(context.Background(), "API initialization failed", append([]any{"operation", "initialize_http_handler"}, logging.ErrorFields(err)...)...)
		var closeErr error
		application.Close(closeApp, &closeErr)
		if closeErr != nil {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "shutdown_application"}, logging.ErrorFields(closeErr)...)...)
		}
		stop()
		telemetry.Shutdown(provider, logger)
		os.Exit(1)
	}

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Route("/monitor", func(r chi.Router) {
		r.Get("/health", rest.HealthCheckHandler)
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/signup", handler.Signup.Signup)
		r.Post("/login", handler.Login.Login)
		auth := r.With(middleware.NewAuthMiddleware(app.UseCase.PersonalAccessToken.Authenticate))

		// Personal Access Tokens
		auth.Delete("/personal-access-token:revoke", handler.PersonalAccessToken.Revoke)

		// Users
		auth.Get("/users/me", handler.User.Get)
		auth.Patch("/users/me", handler.User.UpdateBasicInfo)
		auth.Patch("/users/me/timezone", handler.User.UpdateTimezone)
		auth.Patch("/users/me/password", handler.User.UpdatePassword)

		// Role & Permissions
		auth.Get("/roles", handler.Role.List)
		auth.Get("/permissions", handler.Role.ListPermissions)
		auth.Get("/timezones", handler.Timezone.List)

		// Admin operations
		auth.Post("/admin/jobs/backfill-schedules:run", handler.Admin.RunBackfill)
		auth.Get("/admin/operations", handler.Admin.ListOperations)
		auth.Get("/admin/operations/{id}", handler.Admin.GetOperation)

		// Projects
		auth.Get("/projects", handler.Project.List)
		auth.Get("/projects/options", handler.Project.Options)
		auth.Get("/projects/{id}", handler.Project.Get)
		auth.Get("/projects/{id}/revisions", handler.Project.ListRevisions)
		auth.Patch("/projects/{id}/status", handler.Project.ChangeStatus)
		auth.Post("/projects/{id}:start", handler.Project.Start)
		auth.Post("/projects/{id}:hold", handler.Project.Hold)
		auth.Post("/projects/{id}:wait", handler.Project.Wait)
		auth.Post("/projects/{id}:complete", handler.Project.Complete)
		auth.Post("/projects/{id}:reopen", handler.Project.Reopen)
		auth.Post("/projects/{id}/restore", handler.Project.Restore)
		auth.Post("/projects", handler.Project.Create)
		auth.Patch("/projects/{id}", handler.Project.Update)
		auth.Delete("/projects/{id}", handler.Project.Delete)

		// Project Members
		auth.Get("/projects/{id}/members", handler.ProjectMember.ListMembers)
		auth.Put("/projects/{id}/members/{user_id}", handler.ProjectMember.UpsertMember)
		auth.Delete("/projects/{id}/members/{user_id}", handler.ProjectMember.DeleteMember)

		// Project Tasks
		auth.Get("/projects/{id}/tasks", handler.Project.ListTasks)
		auth.Post("/projects/{id}/tasks", handler.Project.CreateTask)
		auth.Post("/projects/{id}/tasks:add", handler.Project.AddTask)
		auth.Post("/projects/{id}/tasks:remove", handler.Project.RemoveTask)

		// Project Schedules
		auth.Get("/projects/{id}/schedules", handler.Schedule.ListProjectOccurrences)
		auth.Post("/projects/{id}/schedules", handler.Schedule.CreateProjectBlueprint)

		// Tasks
		auth.Get("/tasks", handler.Task.List)
		auth.Get("/tasks/{id}", handler.Task.Get)
		auth.Get("/tasks/{id}/revisions", handler.Task.ListRevisions)
		auth.Get("/tasks/{id}/assignees", handler.Task.ListAssignees)
		auth.Post("/tasks", handler.Task.Create)
		auth.Post("/tasks/{id}:start", handler.Task.Start)
		auth.Post("/tasks/{id}:hold", handler.Task.Hold)
		auth.Post("/tasks/{id}:wait", handler.Task.Wait)
		auth.Post("/tasks/{id}:complete", handler.Task.Complete)
		auth.Post("/tasks/{id}:reopen", handler.Task.Reopen)
		auth.Patch("/tasks/{id}", handler.Task.Update)
		auth.Patch("/tasks/{id}/assignees", handler.Task.Assign)
		auth.Delete("/tasks/{id}", handler.Task.Delete)

		// Action Items
		auth.Get("/tasks/{taskId}/action-items", handler.ActionItem.List)
		auth.Post("/tasks/{taskId}/action-items", handler.ActionItem.Create)
		auth.Post("/tasks/{taskId}/action-items/{id}:complete", handler.ActionItem.Complete)
		auth.Post("/tasks/{taskId}/action-items/{id}:reopen", handler.ActionItem.Reopen)
		auth.Post("/tasks/{taskId}/action-items/{id}:reorder", handler.ActionItem.Reorder)
		auth.Patch("/tasks/{taskId}/action-items/{id}", handler.ActionItem.Update)
		auth.Delete("/tasks/{taskId}/action-items/{id}", handler.ActionItem.Delete)

		// Schedule blueprints
		auth.Post("/schedule-blueprints", handler.Schedule.CreateBlueprint)
		auth.Get("/schedule-blueprints/{id}", handler.Schedule.GetBlueprint)
		auth.Get("/schedule-blueprints/{id}/revisions", handler.Schedule.ListBlueprintRevisions)
		auth.Patch("/schedule-blueprints/{id}", handler.Schedule.UpdateBlueprint)
		auth.Delete("/schedule-blueprints/{id}/following/{occurrence_date}", handler.Schedule.DeleteBlueprintFollowing)
		auth.Get("/schedule-blueprints/{id}/assignees", handler.Schedule.ListAssignees)
		auth.Get("/schedule-blueprints/{id}/tags", handler.Schedule.ListBlueprintTags)
		
		// Schedules
		auth.Get("/schedules", handler.Schedule.ListOccurrences)
		auth.Get("/schedules/{blueprint_id}/{occurrence_date}", handler.Schedule.GetOccurrence)
		auth.Get("/schedules/{blueprint_id}/{occurrence_date}/revisions", handler.Schedule.ListOccurrenceRevisions)
		auth.Patch("/schedules/{blueprint_id}/{occurrence_date}", handler.Schedule.UpdateOccurrence)
		auth.Delete("/schedules/{blueprint_id}/{occurrence_date}", handler.Schedule.DeleteOccurrence)
		auth.Post("/schedules/{blueprint_id}/{occurrence_date}:complete", handler.Schedule.CompleteOccurrence)
		auth.Post("/schedules/{blueprint_id}/{occurrence_date}:reopen", handler.Schedule.ReopenOccurrence)
		auth.Post("/schedules/{blueprint_id}/{occurrence_date}:skip", handler.Schedule.SkipOccurrence)
		auth.Post("/schedules/{blueprint_id}/{occurrence_date}:restore", handler.Schedule.RestoreOccurrence)
		auth.Post("/schedules/{blueprint_id}/{occurrence_date}:reschedule", handler.Schedule.RescheduleOccurrence)

		// Shared Tag catalog and Task associations
		auth.Get("/tags", handler.Tag.List)
		auth.Post("/tags", handler.Tag.Create)
		auth.Get("/tags/{id}", handler.Tag.Get)
		auth.Patch("/tags/{id}", handler.Tag.Rename)
		auth.Delete("/tags/{id}", handler.Tag.Delete)
		auth.Post("/tasks/{id}/tags:add", handler.TaskTag.AddToTask)
		auth.Post("/tasks/{id}/tags:remove", handler.TaskTag.RemoveFromTask)
		auth.Post("/schedule-blueprints/{id}/tags:add", handler.Schedule.AddTagToBlueprint)
		auth.Post("/schedule-blueprints/{id}/tags:remove", handler.Schedule.RemoveTagFromBlueprint)
	})

	srv := &http.Server{
		Addr:              conf.Server.Addr,
		Handler:           otelhttp.NewHandler(r, metadata.Service),
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          log.New(serverLogWriter{logger: logger}, "", 0),
	}

	logger.Info(context.Background(), "server starting", "address", conf.Server.Addr, "swagger_url", swaggerURL(conf.Server.Addr))
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	exitCode := 0
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "serve_http"}, logging.ErrorFields(err)...)...)
			exitCode = 1
		}
	case <-shutdownCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr := srv.Shutdown(ctx)
		cancel()
		if shutdownErr != nil {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "shutdown_http"}, logging.ErrorFields(shutdownErr)...)...)
			exitCode = 1
			if err := srv.Close(); err != nil {
				logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "close_http"}, logging.ErrorFields(err)...)...)
				exitCode = 1
			}
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "serve_http"}, logging.ErrorFields(err)...)...)
			exitCode = 1
		} else if shutdownErr == nil {
			logger.Info(context.Background(), "server stopped")
		}
	}

	var closeErr error
	application.Close(closeApp, &closeErr)
	if closeErr != nil {
		logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "shutdown_application"}, logging.ErrorFields(closeErr)...)...)
		exitCode = 1
	}
	stop()
	telemetry.Shutdown(provider, logger)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func swaggerURL(serverAddr string) string {
	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		host = serverAddr
		port = ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	return "http://" + host + "/swagger/index.html"
}

type serverLogWriter struct{ logger logging.Logger }

func (w serverLogWriter) Write(p []byte) (int, error) {
	w.logger.Error(context.Background(), "HTTP server error", "operation", "serve_http", "source", "net/http")
	return len(p), nil
}

var _ io.Writer = serverLogWriter{}
