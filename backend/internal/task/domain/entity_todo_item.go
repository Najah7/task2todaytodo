package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/shared"
)

var (
	ErrTodoItemIDEmpty           = errors.New("todo item ID cannot be empty")
	ErrTodoItemTaskIDEmpty       = errors.New("todo item task ID cannot be empty")
	ErrTodoItemTitleEmpty        = errors.New("todo item title cannot be empty")
	ErrTodoItemPositionLess      = errors.New("todo item position must be greater than or equal to 0")
	ErrTodoItemIntervalWeeksLess = errors.New("todo item interval weeks must be greater than or equal to 0")
)

type TodoItemID shared.ID

type TodoItem struct {
	ID            TodoItemID
	TaskID        TaskID
	Title         string
	Description   string
	DueDate       time.Time
	Completed     bool
	Position      int
	IntervalWeeks int
	Frequencies   TaskFrequencies
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
	dueDate time.Time,
	completed bool,
	position int,
	intervalWeeks int,
	frequencies TaskFrequencies,
) (TodoItem, error) {
	item := TodoItem{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		Description:   description,
		DueDate:       dueDate,
		Completed:     completed,
		Position:      position,
		IntervalWeeks: intervalWeeks,
		Frequencies:   frequencies,
	}
	return item, item.Validate()
}

func NewExistingTodoItem(
	id TodoItemID,
	taskID TaskID,
	title string,
	description string,
	dueDate time.Time,
	completed bool,
	position int,
	intervalWeeks int,
	frequencies TaskFrequencies,
	createdAt time.Time,
	updatedAt time.Time,
) (TodoItem, error) {
	item := TodoItem{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		Description:   description,
		DueDate:       dueDate,
		Completed:     completed,
		Position:      position,
		IntervalWeeks: intervalWeeks,
		Frequencies:   frequencies,
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
	if i.IntervalWeeks < 0 {
		return ErrTodoItemIntervalWeeksLess
	}

	return nil
}

func (i TodoItem) IsWeekly() bool {
	return i.IntervalWeeks == WeeklyIntervalWeeks
}

func (i TodoItem) IsEveryWeekday() bool {
	return i.Frequencies.IsWeekday() && i.IsWeekly()
}

func (i TodoItem) IsEveryWeekend() bool {
	return i.Frequencies.IsWeekend() && i.IsWeekly()
}

func (i TodoItem) IsBiWeekly() bool {
	return i.IntervalWeeks == BiWeeklyIntervalWeeks
}

func (i TodoItem) IsMonthly() bool {
	return i.IntervalWeeks == MonthlyIntervalWeeks
}

func (i TodoItem) IsQuarterly() bool {
	return i.IntervalWeeks == QuarterlyIntervalWeeks
}

func (i TodoItem) IsSemiAnnually() bool {
	return i.IntervalWeeks == SemiAnnualIntervalWeeks
}

func (i TodoItem) IsAnnually() bool {
	return i.IntervalWeeks == AnnualIntervalWeeks
}

func (i TodoItem) IsOnce() bool {
	return i.IntervalWeeks == 0
}

type TodoItems []TodoItem

func (items TodoItems) SortByPosition() TodoItems {
	sorted := make(TodoItems, len(items))
	copy(sorted, items)

	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].Position > sorted[j+1].Position {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}

func (items TodoItems) HasIncomplete() bool {
	for _, item := range items {
		if !item.Completed {
			return true
		}
	}
	return false
}
