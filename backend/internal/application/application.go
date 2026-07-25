package application

import (
	"context"

	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/Najah7/task2schedule/internal/domain/auth"
	"github.com/Najah7/task2schedule/internal/domain/task"
	"github.com/Najah7/task2schedule/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
)

type config struct {
}

type logger interface{}

type Store struct {
	Users           *repositories.UserRepository
	AccessTokens    *repositories.AccessTokenRepository
	ProjectTypes    *repositories.ProjectTypeRepository
	TaskFrequencies *repositories.TaskFrequencyRepository
	TaskPriorities  *repositories.TaskPriorityRepository
	TaskStatuses    *repositories.TaskStatusRepository
}

func NewStore(ctx context.Context) Store {
	cfg, err := pgxpool.ParseConfig(postgresConnectionString())
	if err != nil {
		panic(fmt.Errorf("failed to parse connection string: %w", err))
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(fmt.Errorf("failed to create connection pool: %w", err))
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		panic(fmt.Errorf("ping postgres: %w", err))
	}

	userRepo := repositories.NewUserRepository(pool)
	accessTokenRepo := repositories.NewAccessTokenRepository(pool)
	projectTypeRepo := repositories.NewProjectTypeRepository(pool)
	taskFrequencyRepo := repositories.NewTaskFrequencyRepository(pool)
	taskPriorityRepo := repositories.NewTaskPriorityRepository(pool)
	taskStatusRepo := repositories.NewTaskStatusRepository(pool)

	return Store{
		Users:           userRepo,
		AccessTokens:    accessTokenRepo,
		ProjectTypes:    projectTypeRepo,
		TaskFrequencies: taskFrequencyRepo,
		TaskPriorities:  taskPriorityRepo,
		TaskStatuses:    taskStatusRepo,
	}
}

func postgresConnectionString() string {
	postgresURL := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			getenv("POSTGRES_USER", "changeme"),
			getenv("POSTGRES_PASSWORD", "changeme"),
		),
		Host: net.JoinHostPort(
			getenv("POSTGRES_HOST", "localhost"),
			getenv("POSTGRES_PORT", "5432"),
		),
		Path: "/" + getenv("POSTGRES_DB", "task2schedule"),
	}

	query := postgresURL.Query()
	query.Set("sslmode", getenv("POSTGRES_SSLMODE", "disable"))
	postgresURL.RawQuery = query.Encode()

	return postgresURL.String()
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

type Service struct {
	User          *auth.UserService
	AccessToken   *auth.AccessTokenService
	ProjectType   *task.ProjectTypeService
	TaskFrequency *task.TaskFrequencyService
	TaskPriority  *task.TaskPriorityService
	TaskStatus    *task.TaskStatusService
}

func NewService(store Store) Service {
	userService := auth.NewUserService(store.Users)
	accessTokenService := auth.NewAccessTokenService(store.AccessTokens)
	projectTypeService := task.NewProjectTypeService(store.ProjectTypes)
	taskFrequencyService := task.NewTaskFrequencyService(store.TaskFrequencies)
	taskPriorityService := task.NewTaskPriorityService(store.TaskPriorities)
	taskStatusService := task.NewTaskStatusService(store.TaskStatuses)

	return Service{
		User:          userService,
		AccessToken:   accessTokenService,
		ProjectType:   projectTypeService,
		TaskFrequency: taskFrequencyService,
		TaskPriority:  taskPriorityService,
		TaskStatus:    taskStatusService,
	}
}

type Application struct {
	config  config
	logger  logger
	Store   Store
	Service Service
}

func New() *Application {
	ctx := context.Background()
	store := NewStore(ctx)
	service := NewService(store)

	return &Application{
		config:  config{},
		logger:  nil,
		Store:   store,
		Service: service,
	}
}
