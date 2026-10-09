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

type actionItemRecurrenceWriter interface {
	SetActionItemRecurrence(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, time.Time, int, []dao.TaskFrequency) error
	StopActionItemRecurrence(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID) error
}

type actionItemSkippedOccurrenceStore interface {
	ListActionItemSkippedOccurrences(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID) ([]int64, error)
	SetActionItemSkippedOccurrence(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, time.Time, bool) error
}

type actionItemOccurrenceProjectionReader interface {
	ListByTaskForOccurrenceProjection(context.Context, domain.UserID, domain.TaskID) ([]dao.ActionItem, error)
}

type actionItemOccurrenceCommandReader interface {
	ListByTaskForOccurrenceCommand(context.Context, domain.UserID, domain.TaskID, shared.Capability) ([]dao.ActionItem, error)
	GetForCommand(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, shared.Capability) (dao.ActionItem, error)
}

type actionItemSkippedOccurrenceCommandStore interface {
	ListActionItemSkippedOccurrencesForCapability(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, shared.Capability) ([]int64, error)
}

type actionItemSeriesTemplateWriter interface {
	UpdateActionItemSeriesTemplate(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, domain.ActionItemID, domain.ActionItem) (dao.ActionItem, error)
}

func localToday(now time.Time, location *time.Location) time.Time {
	return calendar.CalendarDate(now, location)
}

func actionItemIsRecurring(root dao.ActionItem) bool {
	if root.RepeatState != "" {
		return root.RepeatState != repeatStateOneOff
	}
	return root.IntervalWeeks > 0
}
