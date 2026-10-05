package domain

import (
	"errors"
	"strings"
	"time"

	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
)

var (
	ErrTodoItemIDEmpty           = errors.New("todo item ID cannot be empty")
	ErrTodoItemTaskIDEmpty       = errors.New("todo item task ID cannot be empty")
	ErrTodoItemTitleEmpty        = errors.New("todo item title cannot be empty")
	ErrTodoItemPositionLess      = errors.New("todo item position must be greater than or equal to 0")
	ErrTodoItemIntervalWeeksLess = errors.New("todo item interval weeks must be greater than or equal to 0")
)

type TodoItemID string

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
	// SeriesID points to the original row. The original row is its own series root.
	SeriesID       TodoItemID
	OccurrenceDate time.Time
	Timezone       string
	IsException    bool
	Deleted        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
		IntervalWeeks: OnceIntervalWeeks,
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
	recurrence ...RecurrenceMetadata,
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
	if len(recurrence) > 0 {
		item.SeriesID = TodoItemID(recurrence[0].SeriesID)
		item.OccurrenceDate = recurrence[0].OccurrenceDate
		item.Timezone = recurrence[0].Timezone
		item.IsException = recurrence[0].IsException
		item.Deleted = recurrence[0].Deleted
	} else {
		item.SeriesID = id
		item.OccurrenceDate = dueDate
		if item.OccurrenceDate.IsZero() {
			item.OccurrenceDate = createdAt
		}
		item.Timezone = "UTC"
	}
	if err := item.Validate(); err != nil {
		return NewZeroTodoItem(), err
	}
	if len(recurrence) > 0 {
		return NewTodoItemWithRecurrence(item)
	}

	return item, nil
}

func NewZeroTodoItem() TodoItem {
	return TodoItem{}
}

// NewTodoItemWithRecurrence restores a row with recurrence lineage metadata.
func NewTodoItemWithRecurrence(item TodoItem) (TodoItem, error) {
	if item.SeriesID == "" {
		item.SeriesID = item.ID
	}
	item.OccurrenceDate = tasktime.NormalizeCalendarDate(item.OccurrenceDate)
	if err := item.Validate(); err != nil {
		return NewZeroTodoItem(), err
	}
	if item.OccurrenceDate.IsZero() || strings.TrimSpace(item.Timezone) == "" {
		return NewZeroTodoItem(), ErrRecurrenceMetadataInvalid
	}
	if _, err := time.LoadLocation(item.Timezone); err != nil {
		return NewZeroTodoItem(), ErrRecurrenceTimezoneInvalid
	}
	return item, nil
}

func (i TodoItem) WithRecurrence(metadata RecurrenceMetadata) (TodoItem, error) {
	i.SeriesID = TodoItemID(metadata.SeriesID)
	i.OccurrenceDate = metadata.OccurrenceDate
	i.Timezone = metadata.Timezone
	i.IsException = metadata.IsException
	i.Deleted = metadata.Deleted
	return NewTodoItemWithRecurrence(i)
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

func (i TodoItem) IsEveryFourWeeks() bool {
	return i.IntervalWeeks == FourWeekIntervalWeeks
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
