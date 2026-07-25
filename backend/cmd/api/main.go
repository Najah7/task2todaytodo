package main

import (
	"fmt"
	"net/http"

	_ "github.com/Najah7/task2schedule/docs"
	"github.com/Najah7/task2schedule/internal/adapters"
	"github.com/Najah7/task2schedule/internal/application"
	"github.com/Najah7/task2schedule/internal/handlers"
	"github.com/Najah7/task2schedule/internal/middlewares"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title						task2schedule API
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
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	fmt.Println("Server is running on http://localhost:8080/swagger/index.html")

	srv.ListenAndServe()
}
