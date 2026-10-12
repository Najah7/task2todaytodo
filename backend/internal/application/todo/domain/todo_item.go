package domain

import (
	"errors"
	"time"
)

var ErrInvalidTodoItem = errors.New("invalid todo item")

type TodoItem struct {
	ID             string
	TodoID         string
	Kind           FrameKind
	ActionItemID   string
	TaskID         string
	ScheduleRuleID string
	OccurrenceDate time.Time
	PlannedMinutes *int
	Start          *time.Time
	End            *time.Time
	Completed      bool
	Hidden         bool
	Position       int
}

func NewActionTodoItem(id, todoID, actionItemID string, plannedMinutes int, start, end *time.Time, completed bool, position int) (TodoItem, error) {
	item := TodoItem{ID: id, TodoID: todoID, Kind: FrameActionItem, ActionItemID: actionItemID,
		PlannedMinutes: &plannedMinutes, Start: start, End: end, Completed: completed, Position: position}
	return item, item.Validate()
}

func NewTaskTodoItem(id, todoID, taskID string, plannedMinutes int, start, end *time.Time, completed bool, position int) (TodoItem, error) {
	item := TodoItem{ID: id, TodoID: todoID, Kind: FrameTask, TaskID: taskID,
		PlannedMinutes: &plannedMinutes, Start: start, End: end, Completed: completed, Position: position}
	return item, item.Validate()
}

func NewScheduleTodoItem(id, todoID, ruleID string, occurrenceDate, start, end time.Time, completed, hidden bool, position int) (TodoItem, error) {
	item := TodoItem{ID: id, TodoID: todoID, Kind: FrameSchedule, ScheduleRuleID: ruleID,
		OccurrenceDate: dateOnly(occurrenceDate), Start: &start, End: &end,
		Completed: completed, Hidden: hidden, Position: position}
	return item, item.Validate()
}

// RestoreTodoItem validates persisted state without applying input transforms.
func RestoreTodoItem(item TodoItem) (TodoItem, error) {
	return item, item.Validate()
}

func (item TodoItem) Validate() error {
	if item.ID == "" || item.TodoID == "" || item.Position < 0 {
		return ErrInvalidTodoItem
	}
	switch item.Kind {
	case FrameActionItem:
		if item.ActionItemID == "" || item.TaskID != "" || item.ScheduleRuleID != "" || !item.OccurrenceDate.IsZero() ||
			item.PlannedMinutes == nil || *item.PlannedMinutes < 1 || item.Hidden {
			return ErrInvalidTodoItem
		}
	case FrameTask:
		if item.TaskID == "" || item.ActionItemID != "" || item.ScheduleRuleID != "" || !item.OccurrenceDate.IsZero() ||
			item.PlannedMinutes == nil || *item.PlannedMinutes < 1 || item.Hidden {
			return ErrInvalidTodoItem
		}
	case FrameSchedule:
		if item.ActionItemID != "" || item.TaskID != "" || item.ScheduleRuleID == "" || item.OccurrenceDate.IsZero() ||
			item.PlannedMinutes != nil || item.Start == nil || item.End == nil || !item.End.After(*item.Start) {
			return ErrInvalidTodoItem
		}
	default:
		return ErrInvalidTodoItem
	}
	if (item.Start == nil) != (item.End == nil) || (item.Start != nil && !item.End.After(*item.Start)) {
		return ErrInvalidTodoItem
	}
	return nil
}

func (item TodoItem) SetCompleted(completed bool) TodoItem {
	item.Completed = completed
	return item
}

func (item TodoItem) SetHidden(hidden bool) (TodoItem, error) {
	if item.Kind != FrameSchedule || (hidden && item.Completed) {
		return TodoItem{}, ErrInvalidTodoItem
	}
	item.Hidden = hidden
	return item, item.Validate()
}
