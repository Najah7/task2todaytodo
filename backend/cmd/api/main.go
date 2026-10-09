package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
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
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
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
	if run() != nil {
		os.Exit(1)
	}
}

func run() (runErr error) {
	operation := "load_configuration"
	logger := logging.New(os.Stdout, logging.Config{
		Service: "api", Environment: "development", Version: "1.0.0", Level: slog.LevelInfo,
	})
	defer func() {
		if runErr != nil {
			attrs := append([]any{"operation", operation}, logging.ErrorFields(runErr)...)
			logger.Error(context.Background(), "API stopped with error", attrs...)
		}
	}()

	conf, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger = logging.New(os.Stdout, logging.Config{
		Service: conf.ServiceName, Environment: conf.Environment, Version: conf.AppVersion, Level: conf.LogLevel,
	})
	operation = "configure_telemetry"
	provider, metadata, err := telemetry.Setup(context.Background(), telemetry.Metadata{
		Service: conf.ServiceName, Environment: conf.Environment, Version: conf.AppVersion,
	})
	if err != nil {
		return fmt.Errorf("configure telemetry: %w", err)
	}
	logger = logging.New(os.Stdout, logging.Config{
		Service: metadata.Service, Environment: metadata.Environment, Version: metadata.Version, Level: conf.LogLevel,
	})
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error(context.Background(), "OpenTelemetry error", logging.ErrorFields(err)...)
	}))
	defer func() {
		flushCtx, cancelFlush := context.WithTimeout(context.Background(), 5*time.Second)
		if err := provider.ForceFlush(flushCtx); err != nil {
			logger.Error(context.Background(), "flush telemetry provider", logging.ErrorFields(err)...)
		}
		cancelFlush()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			logger.Error(context.Background(), "shutdown telemetry provider", logging.ErrorFields(err)...)
		}
	}()
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	operation = "configure_page_tokens"
	pageTokens, err := pagination.NewCodec(conf.PageTokenKey)
	if err != nil {
		return fmt.Errorf("configure page token codec: %w", err)
	}
	ids := adapter.NewULID()
	operation = "initialize_application"
	app, err := application.New(conf.Database, ids, logger)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	r := chi.NewRouter()
	r.Use(middleware.StripTrailingSlash)
	r.Use(middleware.NewRequestLogging(logger, adapter.NewUUID()))

	ulid := ids

	userHandler := rest.NewUserHandler(app.UseCase.User)
	signupHandler := rest.NewSignupHandler(app.UseCase.User, ulid)
	loginHandler := rest.NewLoginHandler(app.UseCase.PersonalAccessToken)
	personalAccessTokenHandler := rest.NewPersonalAccessTokenHandler(app.UseCase.PersonalAccessToken)
	projectHandler := rest.NewProjectHandler(app.UseCase.Project, app.UseCase.Task, ulid, pageTokens)
	taskHandler := rest.NewTaskHandler(app.UseCase.Task, ulid, pageTokens)
	actionItemHandler := rest.NewActionItemHandler(app.UseCase.ActionItem, ulid, pageTokens)
	scheduleHandler := rest.NewScheduleHandler(app.UseCase.Schedule, ulid, pageTokens)
	tagHandler := rest.NewTagHandler(app.UseCase.Tag, ulid, pageTokens)
	taskTagHandler := rest.NewTaskTagHandler(app.UseCase.TaskTag)
	projectMemberHandler := rest.NewProjectMemberHandler(app.UseCase.Project)
	roleHandler := rest.NewRoleHandler(app.UseCase.Role)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Route("/monitor", func(r chi.Router) {
		r.Get("/health", rest.HealthCheckHandler)
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/signup", signupHandler.Signup)
		r.Post("/login", loginHandler.Login)
		auth := r.With(middleware.NewAuthMiddleware(app.UseCase.PersonalAccessToken.Authenticate))

		// Personal Access Tokens
		auth.Delete("/personal-access-token:revoke", personalAccessTokenHandler.Revoke)

		// Users
		auth.Get("/users/me", userHandler.Get)
		auth.Patch("/users/me", userHandler.UpdateBasicInfo)
		auth.Patch("/users/me/timezone", userHandler.UpdateTimezone)
		auth.Patch("/users/me/password", userHandler.UpdatePassword)

		// Role & Permissions
		auth.Get("/roles", roleHandler.List)
		auth.Get("/permissions", roleHandler.ListPermissions)

		// Projects
		auth.Get("/projects", projectHandler.List)
		auth.Get("/projects/options", projectHandler.Options)
		auth.Get("/projects/{id}", projectHandler.Get)
		auth.Get("/projects/{id}/revisions", projectHandler.ListRevisions)
		auth.Patch("/projects/{id}/status", projectHandler.ChangeStatus)
		auth.Post("/projects/{id}/restore", projectHandler.Restore)
		auth.Post("/projects", projectHandler.Create)
		auth.Patch("/projects/{id}", projectHandler.Update)
		auth.Delete("/projects/{id}", projectHandler.Delete)

		// Project Members
		auth.Get("/projects/{id}/members", projectMemberHandler.ListMembers)
		auth.Put("/projects/{id}/members/{user_id}", projectMemberHandler.UpsertMember)
		auth.Delete("/projects/{id}/members/{user_id}", projectMemberHandler.DeleteMember)

		// Project Tasks
		auth.Get("/projects/{id}/tasks", projectHandler.ListTasks)
		auth.Post("/projects/{id}/tasks", projectHandler.CreateTask)
		auth.Post("/projects/{id}/tasks:add", projectHandler.AddTask)
		auth.Post("/projects/{id}/tasks:remove", projectHandler.RemoveTask)

		// Project Schedules
		auth.Get("/projects/{id}/schedules", scheduleHandler.ListProject)
		auth.Post("/projects/{id}/schedules", scheduleHandler.CreateForProject)
		auth.Post("/projects/{id}/schedules:add", scheduleHandler.AddToProject)
		auth.Post("/projects/{id}/schedules:remove", scheduleHandler.RemoveFromProject)

		// Tasks
		auth.Get("/tasks", taskHandler.List)
		auth.Get("/tasks/{id}", taskHandler.Get)
		auth.Get("/tasks/{id}/revisions", taskHandler.ListRevisions)
		auth.Get("/tasks/{id}/assignees", taskHandler.ListAssignees)
		auth.Post("/tasks", taskHandler.Create)
		auth.Post("/tasks/{id}:start", taskHandler.Start)
		auth.Post("/tasks/{id}:hold", taskHandler.Hold)
		auth.Post("/tasks/{id}:wait", taskHandler.Wait)
		auth.Post("/tasks/{id}:complete", taskHandler.Complete)
		auth.Post("/tasks/{id}:reopen", taskHandler.Reopen)
		auth.Patch("/tasks/{id}", taskHandler.Update)
		auth.Patch("/tasks/{id}/assignees", taskHandler.Assign)
		auth.Delete("/tasks/{id}", taskHandler.Delete)

		// Action Items
		auth.Get("/tasks/{taskId}/action-items", actionItemHandler.List)
		auth.Post("/tasks/{taskId}/action-items", actionItemHandler.Create)
		auth.Post("/tasks/{taskId}/action-items/{id}:complete", actionItemHandler.Complete)
		auth.Post("/tasks/{taskId}/action-items/{id}:reopen", actionItemHandler.Reopen)
		auth.Post("/tasks/{taskId}/action-items/{id}:skip", actionItemHandler.Skip)
		auth.Post("/tasks/{taskId}/action-items/{id}:restore", actionItemHandler.Restore)
		auth.Post("/tasks/{taskId}/action-items/{id}:reorder", actionItemHandler.Reorder)
		auth.Patch("/tasks/{taskId}/action-items/{id}", actionItemHandler.Update)
		auth.Put("/tasks/{taskId}/action-items/{id}/frequency", actionItemHandler.UpdateFrequency)
		auth.Delete("/tasks/{taskId}/action-items/{id}", actionItemHandler.Delete)

		// Schedules
		auth.Get("/schedules", scheduleHandler.List)
		auth.Post("/schedules", scheduleHandler.Create)
		auth.Get("/schedules/{id}", scheduleHandler.Get)
		auth.Get("/schedules/{id}/revisions", scheduleHandler.ListRevisions)
		auth.Get("/schedules/{id}/assignees", scheduleHandler.ListAssignees)
		auth.Patch("/schedules/{id}/assignees", scheduleHandler.Assign)
		auth.Post("/schedules/{id}:complete", scheduleHandler.Complete)
		auth.Post("/schedules/{id}:reopen", scheduleHandler.Reopen)
		auth.Post("/schedules/{id}:skip", scheduleHandler.Skip)
		auth.Post("/schedules/{id}:restore", scheduleHandler.Restore)
		auth.Post("/schedules/{id}:reschedule", scheduleHandler.Reschedule)
		auth.Patch("/schedules/{id}", scheduleHandler.Update)
		auth.Put("/schedules/{id}/frequency", scheduleHandler.UpdateFrequency)
		auth.Delete("/schedules/{id}", scheduleHandler.Delete)

		// Shared Tag catalog and Task associations
		auth.Get("/tags", tagHandler.List)
		auth.Post("/tags", tagHandler.Create)
		auth.Get("/tags/{id}", tagHandler.Get)
		auth.Patch("/tags/{id}", tagHandler.Rename)
		auth.Delete("/tags/{id}", tagHandler.Delete)
		auth.Post("/tasks/{id}/tags:add", taskTagHandler.AddToTask)
		auth.Post("/tasks/{id}/tags:remove", taskTagHandler.RemoveFromTask)
		auth.Post("/schedules/{id}/tags:add", scheduleHandler.AddTag)
		auth.Post("/schedules/{id}/tags:remove", scheduleHandler.RemoveTag)
	})

	srv := &http.Server{
		Addr:              conf.ServerAddr,
		Handler:           otelhttp.NewHandler(r, metadata.Service),
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          log.New(serverLogWriter{logger: logger}, "", 0),
	}

	logger.Info(context.Background(), "server starting", "address", conf.ServerAddr, "swagger_url", swaggerURL(conf.ServerAddr))
	operation = "serve_http"
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
	case <-shutdownCtx.Done():
		operation = "shutdown_http"
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		logger.Info(context.Background(), "server stopped")
	}
	return nil
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
