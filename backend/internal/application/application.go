package application

import (
	"context"
	"errors"

	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/Najah7/task2todaytodo/internal/domain/auth"
	"github.com/Najah7/task2todaytodo/internal/domain/task"
	"github.com/Najah7/task2todaytodo/internal/repositories"
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

type Store struct {
	Users           *repositories.UserRepository
	AccessTokens    *repositories.AccessTokenRepository
	Projects        *repositories.ProjectRepository
	Tasks           *repositories.TaskRepository
	TodoItems       *repositories.TodoItemRepository
	TaskSchedules   *repositories.TaskScheduleRepository
	ProjectTypes    *repositories.ProjectTypeRepository
	TaskFrequencies *repositories.TaskFrequencyRepository
	TaskPriorities  *repositories.TaskPriorityRepository
	TaskStatuses    *repositories.TaskStatusRepository
}

func newStore(pool *pgxpool.Pool) Store {
	userRepo := repositories.NewUserRepository(pool)
	accessTokenRepo := repositories.NewAccessTokenRepository(pool)
	projectRepo := repositories.NewProjectRepository(pool)
	taskRepo := repositories.NewTaskRepository(pool)
	todoItemRepo := repositories.NewTodoItemRepository(pool)
	taskScheduleRepo := repositories.NewTaskScheduleRepository(pool)
	projectTypeRepo := repositories.NewProjectTypeRepository(pool)
	taskFrequencyRepo := repositories.NewTaskFrequencyRepository(pool)
	taskPriorityRepo := repositories.NewTaskPriorityRepository(pool)
	taskStatusRepo := repositories.NewTaskStatusRepository(pool)

	return Store{
		Users:           userRepo,
		AccessTokens:    accessTokenRepo,
		Projects:        projectRepo,
		Tasks:           taskRepo,
		TodoItems:       todoItemRepo,
		TaskSchedules:   taskScheduleRepo,
		ProjectTypes:    projectTypeRepo,
		TaskFrequencies: taskFrequencyRepo,
		TaskPriorities:  taskPriorityRepo,
		TaskStatuses:    taskStatusRepo,
	}
}

func (s Store) WithTx(tx pgx.Tx) Store {
	return Store{
		Users:           s.Users.WithTx(tx),
		AccessTokens:    s.AccessTokens.WithTx(tx),
		Projects:        s.Projects.WithTx(tx),
		Tasks:           s.Tasks.WithTx(tx),
		TodoItems:       s.TodoItems.WithTx(tx),
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

type Transaction struct {
	pool  *pgxpool.Pool
	store Store
}

func newTransaction(pool *pgxpool.Pool, store Store) *Transaction {
	return &Transaction{
		pool:  pool,
		store: store,
	}
}

func (t *Transaction) Run(
	ctx context.Context,
	fn func(ctx context.Context, store Store) error,
) error {
	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback(ctx)
			panic(recovered)
		}
	}()

	if err := fn(ctx, t.store.WithTx(tx)); err != nil {
		rollbackErr := tx.Rollback(ctx)

		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(
				err,
				fmt.Errorf("rollback transaction: %w", rollbackErr),
			)
		}

		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

type Service struct {
	User          *auth.UserService
	AccessToken   *auth.AccessTokenService
	Project       *task.ProjectService
	Task          *task.TaskService
	TodoItem      *task.TodoItemService
	TaskSchedule  *task.TaskScheduleService
	ProjectType   *task.ProjectTypeService
	TaskFrequency *task.TaskFrequencyService
	TaskPriority  *task.TaskPriorityService
	TaskStatus    *task.TaskStatusService
}

func newService(store Store) Service {
	userService := auth.NewUserService(store.Users)
	accessTokenService := auth.NewAccessTokenService(store.AccessTokens)
	projectService := task.NewProjectService(store.Projects)
	taskService := task.NewTaskService(store.Tasks, store.Projects)
	todoItemService := task.NewTodoItemService(store.TodoItems)
	taskScheduleService := task.NewTaskScheduleService(store.TaskSchedules)
	projectTypeService := task.NewProjectTypeService(store.ProjectTypes)
	taskFrequencyService := task.NewTaskFrequencyService(store.TaskFrequencies)
	taskPriorityService := task.NewTaskPriorityService(store.TaskPriorities)
	taskStatusService := task.NewTaskStatusService(store.TaskStatuses)

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

type Application struct {
	config  config
	logger  logger
	Store   Store
	Service Service
	Tx      *Transaction
}

func New() *Application {
	ctx := context.Background()

	pool, err := newPool(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to create connection pool: %w", err))
	}

	store := newStore(pool)
	service := newService(store)

	return &Application{
		config:  config{},
		logger:  nil,
		Store:   store,
		Service: service,
		Tx:      newTransaction(pool, store),
	}
}
