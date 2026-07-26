package application

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"

	authrepo "github.com/Najah7/task2todaytodo/internal/auth/repository"
	authusecase "github.com/Najah7/task2todaytodo/internal/auth/usecase"
	taskrepo "github.com/Najah7/task2todaytodo/internal/task/repository"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type config struct {
}

type logger interface{}

func newPool(ctx context.Context) (*pgxpool.Pool, error) {
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

	return pool, nil
}

type authStore struct {
	Users        *authrepo.UserRepository
	AccessTokens *authrepo.AccessTokenRepository
}

func newAuthStore(pool *pgxpool.Pool) authStore {
	return authStore{
		Users:        authrepo.NewUserRepository(pool),
		AccessTokens: authrepo.NewAccessTokenRepository(pool),
	}
}

func (s authStore) WithTx(tx pgx.Tx) authStore {
	return authStore{
		Users:        s.Users.WithTx(tx),
		AccessTokens: s.AccessTokens.WithTx(tx),
	}
}

type taskStore struct {
	Projects        *taskrepo.ProjectRepository
	Tasks           *taskrepo.TaskRepository
	TodoItems       *taskrepo.TodoItemRepository
	TodoLists       *taskrepo.TodoListRepository
	TaskSchedules   *taskrepo.TaskScheduleRepository
	ProjectTypes    *taskrepo.ProjectTypeRepository
	TaskFrequencies *taskrepo.TaskFrequencyRepository
	TaskPriorities  *taskrepo.TaskPriorityRepository
	TaskStatuses    *taskrepo.TaskStatusRepository
}

func newTaskStore(pool *pgxpool.Pool) taskStore {
	return taskStore{
		Projects:        taskrepo.NewProjectRepository(pool),
		Tasks:           taskrepo.NewTaskRepository(pool),
		TodoItems:       taskrepo.NewTodoItemRepository(pool),
		TodoLists:       taskrepo.NewTodoListRepository(pool),
		TaskSchedules:   taskrepo.NewTaskScheduleRepository(pool),
		ProjectTypes:    taskrepo.NewProjectTypeRepository(pool),
		TaskFrequencies: taskrepo.NewTaskFrequencyRepository(pool),
		TaskPriorities:  taskrepo.NewTaskPriorityRepository(pool),
		TaskStatuses:    taskrepo.NewTaskStatusRepository(pool),
	}
}

func (s taskStore) WithTx(tx pgx.Tx) taskStore {
	return taskStore{
		Projects:        s.Projects.WithTx(tx),
		Tasks:           s.Tasks.WithTx(tx),
		TodoItems:       s.TodoItems.WithTx(tx),
		TodoLists:       s.TodoLists.WithTx(tx),
		TaskSchedules:   s.TaskSchedules.WithTx(tx),
		ProjectTypes:    s.ProjectTypes.WithTx(tx),
		TaskFrequencies: s.TaskFrequencies.WithTx(tx),
		TaskPriorities:  s.TaskPriorities.WithTx(tx),
		TaskStatuses:    s.TaskStatuses.WithTx(tx),
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
		Path: "/" + getenv("POSTGRES_DB", "task2todaytodo"),
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
	User          *authusecase.UserService
	AccessToken   *authusecase.AccessTokenService
	Project       *taskusecase.ProjectService
	Task          *taskusecase.TaskService
	TodoItem      *taskusecase.TodoItemService
	TaskSchedule  *taskusecase.TaskScheduleService
	ProjectType   *taskusecase.ProjectTypeService
	TaskFrequency *taskusecase.TaskFrequencyService
	TaskPriority  *taskusecase.TaskPriorityService
	TaskStatus    *taskusecase.TaskStatusService
}

func newService(authStore authStore, taskStore taskStore) Service {
	userService := authusecase.NewUserService(authStore.Users)
	accessTokenService := authusecase.NewAccessTokenService(authStore.AccessTokens)
	projectService := taskusecase.NewProjectService(taskStore.Projects)
	taskService := taskusecase.NewTaskService(taskStore.Tasks, taskStore.Projects, taskStore.TodoItems)
	todoItemService := taskusecase.NewTodoItemService(taskStore.TodoItems)
	taskScheduleService := taskusecase.NewTaskScheduleService(taskStore.TaskSchedules)
	projectTypeService := taskusecase.NewProjectTypeService(taskStore.ProjectTypes)
	taskFrequencyService := taskusecase.NewTaskFrequencyService(taskStore.TaskFrequencies)
	taskPriorityService := taskusecase.NewTaskPriorityService(taskStore.TaskPriorities)
	taskStatusService := taskusecase.NewTaskStatusService(taskStore.TaskStatuses)

	return Service{
		User:          userService,
		AccessToken:   accessTokenService,
		Project:       projectService,
		Task:          taskService,
		TodoItem:      todoItemService,
		TaskSchedule:  taskScheduleService,
		ProjectType:   projectTypeService,
		TaskFrequency: taskFrequencyService,
		TaskPriority:  taskPriorityService,
		TaskStatus:    taskStatusService,
	}
}

type UnitOfWork struct {
	Auth authusecase.UnitOfWork
	Task taskusecase.UnitOfWork
}

func newUnitOfWork(pool *pgxpool.Pool, authStore authStore, taskStore taskStore) UnitOfWork {
	return UnitOfWork{
		Auth: newAuthUnitOfWork(pool, authStore),
		Task: newTaskUnitOfWork(pool, taskStore),
	}
}

type Application struct {
	config     config
	logger     logger
	Service    Service
	UnitOfWork UnitOfWork
}

func New() *Application {
	ctx := context.Background()

	pool, err := newPool(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to create connection pool: %w", err))
	}

	authStore := newAuthStore(pool)
	taskStore := newTaskStore(pool)
	service := newService(authStore, taskStore)
	unitOfWork := newUnitOfWork(pool, authStore, taskStore)

	return &Application{
		config:     config{},
		logger:     nil,
		Service:    service,
		UnitOfWork: unitOfWork,
	}
}
