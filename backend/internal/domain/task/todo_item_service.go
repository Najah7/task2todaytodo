package task

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
)

var (
	ErrTodoItemTaskNotFound     = errors.New("todo item task not found")
	ErrTodoItemNotFound         = errors.New("todo item not found")
	ErrTodoItemPositionConflict = errors.New("todo item position already exists")
)

type TodoItemCreateParams struct {
	UserID        UserID
	TaskID        TaskID
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

func (s *TodoItemService) Create(ctx context.Context, idGen func() string, params TodoItemCreateParams) (TodoItem, error) {
	id, err := shared.NewID(idGen())
	if err != nil {
		return NewZeroTodoItem(), err
	}

	position := 0
	appendToTail := params.Position == nil
	if params.Position != nil {
		position = *params.Position
	}

	intervalWeeks := OnceIntervalWeeks
	if params.IntervalWeeks != nil {
		intervalWeeks = *params.IntervalWeeks
	}

	frequencies, err := newTaskFrequenciesFromStrings(params.Frequencies)
	if err != nil {
		return NewZeroTodoItem(), err
	}

	item, err := NewTodoItemWithDetails(
		TodoItemID(id),
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
		return NewZeroTodoItem(), err
	}

	return s.repo.CreateForOwnedTask(ctx, params.UserID, item, appendToTail)
}

func (s *TodoItemService) Check(ctx context.Context, userID UserID, taskID TaskID, todoItemID TodoItemID) error {
	return s.repo.CheckForOwnedTask(ctx, userID, taskID, todoItemID)
}

func (s *TodoItemService) Uncheck(ctx context.Context, userID UserID, taskID TaskID, todoItemID TodoItemID) error {
	return s.repo.UncheckForOwnedTask(ctx, userID, taskID, todoItemID)
}

func (s *TodoItemService) Delete(ctx context.Context, userID UserID, taskID TaskID, todoItemID TodoItemID) error {
	return s.repo.DeleteForOwnedTask(ctx, userID, taskID, todoItemID)
}

func newTaskFrequenciesFromStrings(values []string) (TaskFrequencies, error) {
	frequencies := make(TaskFrequencies, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}

		frequency, err := NewTaskFrequency(value)
		if err != nil {
			return nil, err
		}
		frequencies = append(frequencies, frequency)
	}
	return frequencies, nil
}
