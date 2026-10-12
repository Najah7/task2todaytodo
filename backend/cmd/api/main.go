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
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	emailadapter "github.com/Najah7/task2todaytodo/internal/port/adapter/email"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/port/rest/middleware"
	"github.com/Najah7/task2todaytodo/internal/settings"
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
	settings, err := settings.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, append([]any{"API initialization failed", "operation", "load_configuration"}, telemetry.ErrorFields(err)...)...)
		os.Exit(1)
	}
	provider, err := telemetry.Setup(context.Background(), settings.Application, settings.Logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, append([]any{"API initialization failed", "operation", "configure_telemetry"}, telemetry.ErrorFields(err)...)...)
		os.Exit(1)
	}
	logger := telemetry.NewLogger(os.Stdout, settings.Application, settings.Logger)
	var mailSender emailadapter.Sender
	if settings.Email.Provider == "ses" {
		mailSender, err = emailadapter.NewSES(context.Background(), settings.Email.Region, settings.Email.FromAddress, settings.Email.FromName)
	} else {
		mailSender = emailadapter.NewSMTP(settings.Email.SMTPHost, settings.Email.SMTPPort, settings.Email.FromAddress, settings.Email.FromName)
	}
	if err != nil {
		logger.Error(context.Background(), "API initialization failed", append([]any{"operation", "configure_email_sender"}, telemetry.ErrorFields(err)...)...)
		telemetry.Shutdown(provider, logger)
		os.Exit(1)
	}
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error(context.Background(), "OpenTelemetry error", telemetry.ErrorFields(err)...)
	}))
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	idIssuer := adapter.NewIDIssuer()
	app, closeApp, err := application.New(settings.Database, idIssuer, logger, application.AuthEmailSettings{
		Sender: emailadapter.AuthLinkSender{Provider: mailSender}, InvitationSender: emailadapter.ProjectInvitationSender{Provider: mailSender}, FrontendURL: settings.Email.FrontendBaseURL,
	})
	if err != nil {
		logger.Error(context.Background(), "API initialization failed", append([]any{"operation", "initialize_application"}, telemetry.ErrorFields(err)...)...)
		stop()
		telemetry.Shutdown(provider, logger)
		os.Exit(1)
	}
	r := chi.NewRouter()
	r.Use(middleware.StripTrailingSlash)
	r.Use(middleware.NewRequestLogging(logger, idIssuer.NewUUID))

	handler, err := rest.NewHandler(app.UseCase, idIssuer.NewULID, settings.Secrets.PageTokenKey)
	if err != nil {
		logger.Error(context.Background(), "API initialization failed", append([]any{"operation", "initialize_http_handler"}, telemetry.ErrorFields(err)...)...)
		var closeErr error
		application.Close(closeApp, &closeErr)
		if closeErr != nil {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "shutdown_application"}, telemetry.ErrorFields(closeErr)...)...)
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
		optionalAuth := middleware.NewOptionalAuthMiddleware(app.UseCase.PersonalAccessToken.Authenticate)
		r.With(optionalAuth).Post("/invitations/accept", handler.ProjectInvitationAcceptance.Accept)
		r.Post("/invitations/preview", handler.ProjectMember.PreviewInvitation)
		r.Post("/email-verification/resend", handler.EmailAuth.ResendVerification)
		r.Post("/email-verification/confirm", handler.EmailAuth.ConfirmVerification)
		r.Post("/password-reset/request", handler.EmailAuth.RequestPasswordReset)
		r.Post("/password-reset/confirm", handler.EmailAuth.ConfirmPasswordReset)
		auth := r.With(middleware.NewAuthMiddleware(app.UseCase.PersonalAccessToken.Authenticate))

		// Admin
		auth.Route("/admin", func(admin chi.Router) {
			// Admin Jobs
			admin.Post("/jobs/backfill-schedules:run", handler.Admin.RunBackfill)
			admin.Post("/jobs/prefill-allocations:run", handler.Admin.RunAllocationPrefill)

			// Admin Operations
			admin.Get("/operations", handler.Admin.ListOperations)
			admin.Get("/operations/{id}", handler.Admin.GetOperation)
		})

		// Personal Access Tokens
		auth.Delete("/personal-access-token:revoke", handler.PersonalAccessToken.Revoke)

		// Users
		auth.Get("/users/me", handler.User.Get)
		auth.Patch("/users/me", handler.User.UpdateBasicInfo)
		auth.Patch("/users/me/timezone", handler.User.UpdateTimezone)
		auth.Patch("/users/me/sleep", handler.User.UpdateSleepSettings)
		auth.Patch("/users/me/minimum-gap", handler.User.UpdateMinimumGap)
		auth.Patch("/users/me/password", handler.User.UpdatePassword)

		// Role & Permissions
		auth.Get("/roles", handler.Role.List)
		auth.Get("/roles/capabilities", handler.ProjectMember.RoleCapabilities)
		auth.Get("/permissions", handler.Role.ListPermissions)
		auth.Get("/timezones", handler.Timezone.List)

		// Today's generated execution plan
		auth.Get("/today-todo", handler.Todos.GetToday)
		auth.Post("/today-todo:regenerate", handler.Todos.Regenerate)
		auth.Post("/today-todo/action-items/{actionItemId}", handler.Todos.AddActionItem)
		auth.Post("/today-todo/tasks/{taskId}", handler.Todos.AddTask)
		auth.Post("/today-todo:reorder", handler.Todos.Reorder)
		auth.Post("/today-todo/items/{itemId}:complete", handler.Todos.CompleteFrame)
		auth.Post("/today-todo/items/{itemId}:reopen", handler.Todos.ReopenFrame)
		auth.Post("/today-todo/items/{itemId}:remove", handler.Todos.RemoveActionItem)
		auth.Post("/today-todo/schedules/{itemId}:hide", handler.Todos.HideSchedule)
		auth.Post("/today-todo/schedules/{itemId}:restore", handler.Todos.RestoreSchedule)

		// Inbox
		auth.Get("/inbox", handler.Inbox.List)

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
		auth.Patch("/projects/{id}/members/{user_id}", handler.ProjectMember.UpsertMember)
		auth.Delete("/projects/{id}/members/{user_id}", handler.ProjectMember.DeleteMember)
		auth.Post("/projects/{id}/invitations", handler.ProjectMember.CreateInvitation)
		auth.Post("/projects/{id}/invitations/{invitation_id}/resend", handler.ProjectMember.ResendInvitation)
		auth.Delete("/projects/{id}/invitations/{invitation_id}", handler.ProjectMember.RevokeInvitation)

		// Project Tasks
		auth.Get("/projects/{id}/tasks", handler.Project.ListTasks)
		auth.Post("/projects/{id}/tasks", handler.Project.CreateTask)
		auth.Post("/projects/{id}/tasks:add", handler.Project.AddTask)
		auth.Post("/projects/{id}/tasks:remove", handler.Project.RemoveTask)

		// Project Schedules
		auth.Get("/projects/{id}/schedules", handler.Schedule.ListProjectOccurrences)
		auth.Post("/projects/{id}/schedules", handler.Schedule.CreateProjectRule)

		// Project time allocations
		auth.Get("/allocation-weeks/{week_start}", handler.Allocation.GetWeek)
		auth.Get("/allocation-rules", handler.Allocation.GetRules)
		auth.Put("/allocation-rules", handler.Allocation.PutRules)
		auth.Get("/allocation-plans", handler.Allocation.GetPlans)
		auth.Put("/allocation-plans", handler.Allocation.PutPlans)
		auth.Post("/allocations", handler.Allocation.Create)
		auth.Patch("/allocations/{rule_id}/{occurrence_date}", handler.Allocation.Update)
		auth.Delete("/allocations/{rule_id}/{occurrence_date}", handler.Allocation.Delete)
		auth.Put("/daily-sleep-exceptions/{local_date}", handler.Allocation.PutSleepException)
		auth.Delete("/daily-sleep-exceptions/{local_date}", handler.Allocation.DeleteSleepException)

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

		// Schedule rules
		auth.Post("/schedule-rules", handler.Schedule.CreateRule)
		auth.Get("/schedule-rules/{id}", handler.Schedule.GetRule)
		auth.Get("/schedule-rules/{id}/revisions", handler.Schedule.ListRuleRevisions)
		auth.Patch("/schedule-rules/{id}", handler.Schedule.UpdateRule)
		auth.Delete("/schedule-rules/{id}/following/{occurrence_date}", handler.Schedule.DeleteRuleFollowing)
		auth.Get("/schedule-rules/{id}/assignees", handler.Schedule.ListAssignees)

		// Schedules
		auth.Get("/schedules", handler.Schedule.ListOccurrences)
		auth.Get("/schedules/{rule_id}/{occurrence_date}", handler.Schedule.GetOccurrence)
		auth.Get("/schedules/{rule_id}/{occurrence_date}/revisions", handler.Schedule.ListOccurrenceRevisions)
		auth.Patch("/schedules/{rule_id}/{occurrence_date}", handler.Schedule.UpdateOccurrence)
		auth.Delete("/schedules/{rule_id}/{occurrence_date}", handler.Schedule.DeleteOccurrence)
		auth.Post("/schedules/{rule_id}/{occurrence_date}:complete", handler.Schedule.CompleteOccurrence)
		auth.Post("/schedules/{rule_id}/{occurrence_date}:reopen", handler.Schedule.ReopenOccurrence)
		auth.Post("/schedules/{rule_id}/{occurrence_date}:skip", handler.Schedule.SkipOccurrence)
		auth.Post("/schedules/{rule_id}/{occurrence_date}:restore", handler.Schedule.RestoreOccurrence)
		auth.Post("/schedules/{rule_id}/{occurrence_date}:reschedule", handler.Schedule.RescheduleOccurrence)

	})

	srv := &http.Server{
		Addr:              settings.Server.Addr,
		Handler:           otelhttp.NewHandler(r, settings.Application.Service),
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          log.New(serverLogWriter{logger: logger}, "", 0),
	}

	logger.Info(context.Background(), "server starting", "address", settings.Server.Addr, "swagger_url", swaggerURL(settings.Server.Addr))
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	exitCode := 0
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "serve_http"}, telemetry.ErrorFields(err)...)...)
			exitCode = 1
		}
	case <-shutdownCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr := srv.Shutdown(ctx)
		cancel()
		if shutdownErr != nil {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "shutdown_http"}, telemetry.ErrorFields(shutdownErr)...)...)
			exitCode = 1
			if err := srv.Close(); err != nil {
				logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "close_http"}, telemetry.ErrorFields(err)...)...)
				exitCode = 1
			}
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "serve_http"}, telemetry.ErrorFields(err)...)...)
			exitCode = 1
		} else if shutdownErr == nil {
			logger.Info(context.Background(), "server stopped")
		}
	}

	var closeErr error
	application.Close(closeApp, &closeErr)
	if closeErr != nil {
		logger.Error(context.Background(), "API stopped with error", append([]any{"operation", "shutdown_application"}, telemetry.ErrorFields(closeErr)...)...)
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

type serverLogWriter struct{ logger telemetry.Logger }

func (w serverLogWriter) Write(p []byte) (int, error) {
	w.logger.Error(context.Background(), "HTTP server error", "operation", "serve_http", "source", "net/http")
	return len(p), nil
}

var _ io.Writer = serverLogWriter{}
