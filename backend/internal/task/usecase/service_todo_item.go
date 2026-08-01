package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/shared"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

type TodoItemCreateParams struct {
	UserID        domain.UserID
	TaskID        domain.TaskID
	Title         string
	Description   string
	DueDate       time.Time
	Position      *int
	IntervalWeeks *int
	Frequencies   []string
}

type TodoItemService struct {
	repo TodoItemRepository
}

func NewTodoItemService(repo TodoItemRepository) *TodoItemService {
	return &TodoItemService{
		repo: repo,
	}
}

func (s *TodoItemService) Create(ctx context.Context, idGen func() string, params TodoItemCreateParams) (domain.TodoItem, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}

	position := 0
	appendToTail := params.Position == nil
	if params.Position != nil {
		position = *params.Position
	}

	intervalWeeks := domain.OnceIntervalWeeks
	if params.IntervalWeeks != nil {
		intervalWeeks = *params.IntervalWeeks
	}

	frequencies, err := newTaskFrequenciesFromStrings(params.Frequencies)
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}

	item, err := domain.NewTodoItemWithDetails(
		domain.TodoItemID(id),
		params.TaskID,
		params.Title,
		params.Description,
		params.DueDate,
		false,
		position,
		intervalWeeks,
		frequencies,
	)
	if err != nil {
		return domain.NewZeroTodoItem(), err
	}

	return s.repo.CreateForOwnedTask(ctx, params.UserID, item, appendToTail)
}

func (s *TodoItemService) Check(ctx context.Context, userID domain.UserID, taskID domain.TaskID, todoItemID domain.TodoItemID) error {
	return s.repo.CheckForOwnedTask(ctx, userID, taskID, todoItemID)
}

func (s *TodoItemService) Uncheck(ctx context.Context, userID domain.UserID, taskID domain.TaskID, todoItemID domain.TodoItemID) error {
	return s.repo.UncheckForOwnedTask(ctx, userID, taskID, todoItemID)
}

func (s *TodoItemService) Delete(ctx context.Context, userID domain.UserID, taskID domain.TaskID, todoItemID domain.TodoItemID) error {
	return s.repo.DeleteForOwnedTask(ctx, userID, taskID, todoItemID)
}

func newTaskFrequenciesFromStrings(values []string) (domain.TaskFrequencies, error) {
	frequencies := make(domain.TaskFrequencies, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}

		frequency, err := domain.NewTaskFrequency(value)
		if err != nil {
			return nil, err
		}
		frequencies = append(frequencies, frequency)
	}
	return frequencies, nil
}
