package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
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

type taskScheduleRecurrenceWriter interface {
	SetTaskScheduleRecurrence(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID, time.Time, int, []dao.TaskFrequency) error
	StopTaskScheduleRecurrence(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID) error
}

type todoSkippedOccurrenceStore interface {
	ListTodoItemSkippedOccurrences(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) ([]int64, error)
	SetTodoItemSkippedOccurrence(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, time.Time, bool) error
}

type scheduleSkippedOccurrenceStore interface {
	ListTaskScheduleSkippedOccurrences(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID) ([]int64, error)
	SetTaskScheduleSkippedOccurrence(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID, time.Time, bool) error
}

type todoOccurrenceProjectionReader interface {
	ListByTaskForOccurrenceProjection(context.Context, domain.UserID, domain.TaskID) ([]dao.TodoItem, error)
}

type scheduleOccurrenceProjectionReader interface {
	ListByTaskForOccurrenceProjection(context.Context, domain.UserID, domain.TaskID) ([]dao.TaskSchedule, error)
}

type todoOccurrenceCommandReader interface {
	ListByTaskForOccurrenceCommand(context.Context, domain.UserID, domain.TaskID, shared.Capability) ([]dao.TodoItem, error)
	GetForCommand(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, shared.Capability) (dao.TodoItem, error)
}

type scheduleOccurrenceCommandReader interface {
	ListByTaskForOccurrenceCommand(context.Context, domain.UserID, domain.TaskID, shared.Capability) ([]dao.TaskSchedule, error)
	GetForCommand(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID, shared.Capability) (dao.TaskSchedule, error)
}

type todoSkippedOccurrenceCommandStore interface {
	ListTodoItemSkippedOccurrencesForCapability(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, shared.Capability) ([]int64, error)
}

type scheduleSkippedOccurrenceCommandStore interface {
	ListTaskScheduleSkippedOccurrencesForCapability(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID, shared.Capability) ([]int64, error)
}

type todoSeriesTemplateWriter interface {
	UpdateTodoItemSeriesTemplate(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, domain.TodoItemID, domain.TodoItem) (dao.TodoItem, error)
}

type scheduleSeriesTemplateWriter interface {
	UpdateTaskScheduleSeriesTemplate(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID, domain.TaskScheduleID, domain.TaskSchedule) (dao.TaskSchedule, error)
}

func localToday(now time.Time, location *time.Location) time.Time {
	return tasktime.CalendarDate(now, location)
}

func todoItemIsRecurring(root dao.TodoItem) bool {
	if root.RepeatState != "" {
		return root.RepeatState != repeatStateOneOff
	}
	return root.IntervalWeeks > 0
}

func taskScheduleIsRecurring(root dao.TaskSchedule) bool {
	if root.RepeatState != "" {
		return root.RepeatState != repeatStateOneOff
	}
	return root.IntervalWeeks > 0
}
