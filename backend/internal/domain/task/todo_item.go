package task

import (
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2schedule/internal/domain/shared"
)

var (
	ErrTodoItemIDEmpty           = errors.New("todo item ID cannot be empty")
	ErrTodoItemTaskIDEmpty       = errors.New("todo item task ID cannot be empty")
	ErrTodoItemTitleEmpty        = errors.New("todo item title cannot be empty")
	ErrTodoItemPositionLess      = errors.New("todo item position must be greater than or equal to 0")
	ErrTodoItemIntervalWeeksLess = errors.New("todo item interval weeks must be greater than or equal to 1")
)

type TodoItemID shared.ID

type TodoItem struct {
	ID            TodoItemID
	TaskID        TaskID
	Title         string
	Description   string
	Completed     bool
	Position      int
	IntervalWeeks int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewTodoItem(
	id TodoItemID,
	taskID TaskID,
	title string,
) (TodoItem, error) {
	item := TodoItem{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		IntervalWeeks: 1,
	}
	return item, item.Validate()
}

func NewTodoItemWithDetails(
	id TodoItemID,
	taskID TaskID,
	title string,
	description string,
	completed bool,
	position int,
	intervalWeeks int,
) (TodoItem, error) {
	item := TodoItem{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		Description:   description,
		Completed:     completed,
		Position:      position,
		IntervalWeeks: intervalWeeks,
	}
	return item, item.Validate()
}

func NewExistingTodoItem(
	id TodoItemID,
	taskID TaskID,
	title string,
	description string,
	completed bool,
	position int,
	intervalWeeks int,
	createdAt time.Time,
	updatedAt time.Time,
) (TodoItem, error) {
	item := TodoItem{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		Description:   description,
		Completed:     completed,
		Position:      position,
		IntervalWeeks: intervalWeeks,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	if err := item.Validate(); err != nil {
		return NewZeroTodoItem(), err
	}

	return item, nil
}

func NewZeroTodoItem() TodoItem {
	return TodoItem{}
}

func (i TodoItem) IsZero() bool {
	return i.ID == ""
}

func (i TodoItem) Validate() error {
	if i.ID == "" {
		return ErrTodoItemIDEmpty
	}
	if i.TaskID == "" {
		return ErrTodoItemTaskIDEmpty
	}
	if strings.TrimSpace(i.Title) == "" {
		return ErrTodoItemTitleEmpty
	}
	if i.Position < 0 {
		return ErrTodoItemPositionLess
	}
	if i.IntervalWeeks < 1 {
		return ErrTodoItemIntervalWeeksLess
	}

	return nil
}
