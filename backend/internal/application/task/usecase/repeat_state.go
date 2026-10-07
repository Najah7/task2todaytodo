package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	calendar "github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

const (
	repeatStateOneOff  = "one_off"
	repeatStateActive  = "active"
	repeatStateStopped = "stopped"
)

type todoItemRecurrenceWriter interface {
	SetTodoItemRecurrence(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, time.Time, int, []dao.TaskFrequency) error
	StopTodoItemRecurrence(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) error
}

type todoSkippedOccurrenceStore interface {
	ListTodoItemSkippedOccurrences(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) ([]int64, error)
	SetTodoItemSkippedOccurrence(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, time.Time, bool) error
}

type todoOccurrenceProjectionReader interface {
	ListByTaskForOccurrenceProjection(context.Context, domain.UserID, domain.TaskID) ([]dao.TodoItem, error)
}

type todoOccurrenceCommandReader interface {
	ListByTaskForOccurrenceCommand(context.Context, domain.UserID, domain.TaskID, shared.Capability) ([]dao.TodoItem, error)
	GetForCommand(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, shared.Capability) (dao.TodoItem, error)
}

type todoSkippedOccurrenceCommandStore interface {
	ListTodoItemSkippedOccurrencesForCapability(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, shared.Capability) ([]int64, error)
}

type todoSeriesTemplateWriter interface {
	UpdateTodoItemSeriesTemplate(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, domain.TodoItemID, domain.TodoItem) (dao.TodoItem, error)
}

func localToday(now time.Time, location *time.Location) time.Time {
	return calendar.CalendarDate(now, location)
}

func todoItemIsRecurring(root dao.TodoItem) bool {
	if root.RepeatState != "" {
		return root.RepeatState != repeatStateOneOff
	}
	return root.IntervalWeeks > 0
}
