package application

import (
	"context"
	"errors"

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

type Store struct {
	Users           *authrepo.UserRepository
	AccessTokens    *authrepo.AccessTokenRepository
	Projects        *taskrepo.ProjectRepository
	Tasks           *taskrepo.TaskRepository
	TodoItems       *taskrepo.TodoItemRepository
	TaskSchedules   *taskrepo.TaskScheduleRepository
	ProjectTypes    *taskrepo.ProjectTypeRepository
	TaskFrequencies *taskrepo.TaskFrequencyRepository
	TaskPriorities  *taskrepo.TaskPriorityRepository
	TaskStatuses    *taskrepo.TaskStatusRepository
}

func newStore(pool *pgxpool.Pool) Store {
	userRepo := authrepo.NewUserRepository(pool)
	accessTokenRepo := authrepo.NewAccessTokenRepository(pool)
	projectRepo := taskrepo.NewProjectRepository(pool)
	taskRepo := taskrepo.NewTaskRepository(pool)
	todoItemRepo := taskrepo.NewTodoItemRepository(pool)
	taskScheduleRepo := taskrepo.NewTaskScheduleRepository(pool)
	projectTypeRepo := taskrepo.NewProjectTypeRepository(pool)
	taskFrequencyRepo := taskrepo.NewTaskFrequencyRepository(pool)
	taskPriorityRepo := taskrepo.NewTaskPriorityRepository(pool)
	taskStatusRepo := taskrepo.NewTaskStatusRepository(pool)

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

func newService(store Store) Service {
	userService := authusecase.NewUserService(store.Users)
	accessTokenService := authusecase.NewAccessTokenService(store.AccessTokens)
	projectService := taskusecase.NewProjectService(store.Projects)
	taskService := taskusecase.NewTaskService(store.Tasks, store.Projects)
	todoItemService := taskusecase.NewTodoItemService(store.TodoItems)
	taskScheduleService := taskusecase.NewTaskScheduleService(store.TaskSchedules)
	projectTypeService := taskusecase.NewProjectTypeService(store.ProjectTypes)
	taskFrequencyService := taskusecase.NewTaskFrequencyService(store.TaskFrequencies)
	taskPriorityService := taskusecase.NewTaskPriorityService(store.TaskPriorities)
	taskStatusService := taskusecase.NewTaskStatusService(store.TaskStatuses)

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
