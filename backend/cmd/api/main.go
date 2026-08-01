package main

import (
	"fmt"
	"net/http"

	_ "github.com/Najah7/task2todaytodo/docs"
	"github.com/Najah7/task2todaytodo/internal/shared/adapters"
	"github.com/Najah7/task2todaytodo/internal/application"
	authhandlers "github.com/Najah7/task2todaytodo/internal/auth/handlers"
	authmiddlewares "github.com/Najah7/task2todaytodo/internal/auth/middlewares"
	sharedmiddlewares "github.com/Najah7/task2todaytodo/internal/shared/middlewares"
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	taskhandlers "github.com/Najah7/task2todaytodo/internal/task/handlers"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title						task2todaytodo API
// @version					1.0
// @description				Task scheduling API server.
// @host						localhost:8080
// @BasePath					/api
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	r := chi.NewRouter()
	r.Use(sharedmiddlewares.StripTrailingSlash)

	app := application.New()

	ulidGen := adapters.NewULIDGenerator()

	userHandler := authhandlers.NewUserHandler(app.Service.User, ulidGen)
	accessTokenHandler := authhandlers.NewAccessTokenHandler(app.Service.AccessToken, app.Service.User)
	projectHandler := taskhandlers.NewProjectHandler(app.Service.Project, ulidGen)
	taskHandler := taskhandlers.NewTaskHandler(app.Service.Task, ulidGen)
	todoItemHandler := taskhandlers.NewTodoItemHandler(app.Service.TodoItem, ulidGen)
	taskScheduleHandler := taskhandlers.NewTaskScheduleHandler(app.Service.TaskSchedule, ulidGen)
	projectTypeHandler := taskhandlers.NewProjectTypeHandler(app.Service.ProjectType)
	taskFrequencyHandler := taskhandlers.NewTaskFrequencyHandler(app.Service.TaskFrequency)
	taskPriorityHandler := taskhandlers.NewTaskPriorityHandler(app.Service.TaskPriority)
	taskStatusHandler := taskhandlers.NewTaskStatusHandler(app.Service.TaskStatus)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	r.Route("/monitor", func(r chi.Router) {
		r.Get("/health", sharedhandlers.HealthCheckHandler)
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/users", userHandler.Create)
		r.Post("/access-tokens", accessTokenHandler.Generate)
		r.Get("/projects/types", projectTypeHandler.List)
		r.Get("/tasks/frequencies", taskFrequencyHandler.List)
		r.Get("/tasks/priorities", taskPriorityHandler.List)
		r.Get("/tasks/statuses", taskStatusHandler.List)

		authRoutes := r.With(authmiddlewares.AuthMiddleware(*app.Service.AccessToken))
		authRoutes.Get("/users/me", userHandler.Get)
		authRoutes.Patch("/users/me", userHandler.UpdateBasicInfo)
		authRoutes.Patch("/users/me/password", userHandler.UpdatePassword)
		authRoutes.Delete("/access-token/current", accessTokenHandler.Revoke)

		authRoutes.Get("/projects/{project_id}", projectHandler.Get)
		authRoutes.Post("/projects", projectHandler.Create)
		authRoutes.Patch("/projects/{project_id}", projectHandler.Update)
		authRoutes.Patch("/projects/{project_id}/schedule", projectHandler.UpdateSchedule)
		authRoutes.Patch("/projects/{project_id}/priority", projectHandler.UpdatePriority)
		authRoutes.Delete("/projects/{project_id}", projectHandler.Delete)
		authRoutes.Post("/projects/{project_id}/tasks", taskHandler.CreateInProject)

		authRoutes.Get("/tasks/{task_id}", taskHandler.Get)
		authRoutes.Post("/tasks", taskHandler.Create)
		authRoutes.Patch("/tasks/{task_id}", taskHandler.Update)
		authRoutes.Patch("/tasks/{task_id}/status", taskHandler.UpdateStatus)
		authRoutes.Patch("/tasks/{task_id}/priority", taskHandler.UpdatePriority)
		authRoutes.Patch("/tasks/{task_id}/estimation", taskHandler.UpdateEstimation)
		authRoutes.Delete("/tasks/{task_id}", taskHandler.Delete)

		authRoutes.Post("/tasks/{task_id}/todo-items", todoItemHandler.Create)
		authRoutes.Post("/tasks/{task_id}/todo-items/{todo_item_id}:checked", todoItemHandler.Check)
		authRoutes.Post("/tasks/{task_id}/todo-items/{todo_item_id}:unchecked", todoItemHandler.Uncheck)
		authRoutes.Delete("/tasks/{task_id}/todo-items/{todo_item_id}", todoItemHandler.Delete)

		authRoutes.Post("/tasks/{task_id}/schedules", taskScheduleHandler.Create)
		authRoutes.Patch("/tasks/{task_id}/schedules/{task_schedule_id}", taskScheduleHandler.Update)
		authRoutes.Delete("/tasks/{task_id}/schedules/{task_schedule_id}", taskScheduleHandler.Delete)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	fmt.Println("Server is running on http://localhost:8080/swagger/index.html")

	srv.ListenAndServe()
}
