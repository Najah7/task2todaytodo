package main

import (
	"fmt"
	"net/http"

	_ "github.com/Najah7/task2todaytodo/docs"
	"github.com/Najah7/task2todaytodo/internal/adapters"
	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/handlers"
	"github.com/Najah7/task2todaytodo/internal/middlewares"
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
	r.Use(middlewares.StripTrailingSlash)

	app := application.New()

	ulidGen := adapters.NewULIDGenerator()

	userHandler := handlers.NewUserHandler(app.Service.User, ulidGen)
	accessTokenHandler := handlers.NewAccessTokenHandler(app.Service.AccessToken, app.Service.User)
	projectHandler := handlers.NewProjectHandler(app.Service.Project, ulidGen)
	taskHandler := handlers.NewTaskHandler(app.Service.Task, ulidGen)
	todoItemHandler := handlers.NewTodoItemHandler(app.Service.TodoItem, ulidGen)
	taskScheduleHandler := handlers.NewTaskScheduleHandler(app.Service.TaskSchedule, ulidGen)
	projectTypeHandler := handlers.NewProjectTypeHandler(app.Service.ProjectType)
	taskFrequencyHandler := handlers.NewTaskFrequencyHandler(app.Service.TaskFrequency)
	taskPriorityHandler := handlers.NewTaskPriorityHandler(app.Service.TaskPriority)
	taskStatusHandler := handlers.NewTaskStatusHandler(app.Service.TaskStatus)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	r.Route("/monitor", func(r chi.Router) {
		r.Get("/health", handlers.HealthCheckHandler)
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/users", userHandler.Create)
		r.Post("/access-tokens", accessTokenHandler.Generate)
		r.Get("/projects/types", projectTypeHandler.List)
		r.Get("/tasks/frequencies", taskFrequencyHandler.List)
		r.Get("/tasks/priorities", taskPriorityHandler.List)
		r.Get("/tasks/statuses", taskStatusHandler.List)

		authRoutes := r.With(middlewares.AuthMiddleware(*app.Service.AccessToken))
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
