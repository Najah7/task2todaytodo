package domain

import (
	"errors"
	"math"
	"strings"
	"time"
)

var (
	ErrActionItemIDEmpty                 = errors.New("action item ID cannot be empty")
	ErrActionItemTaskIDEmpty             = errors.New("action item task ID cannot be empty")
	ErrActionItemTitleEmpty              = errors.New("action item title cannot be empty")
	ErrActionItemPositionLess            = errors.New("action item position must be greater than or equal to 0")
	ErrActionItemEstimatedMinutesInvalid = errors.New("action item estimated minutes must be between 1 and 2147483647")
	ErrActionItemActualMinutesInvalid    = errors.New("action item actual minutes must be between 0 and 2147483647")
)

type ActionItemID string

type ActionItem struct {
	ID               ActionItemID
	TaskID           TaskID
	Title            string
	Description      string
	DueDate          time.Time
	EstimatedMinutes *int
	ActualMinutes    *int
	Priority         TaskPriority
	Completed        bool
	Position         int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewActionItem(id ActionItemID, taskID TaskID, title string, estimatedMinutes int) (ActionItem, error) {
	item := ActionItem{ID: id, TaskID: taskID, Title: title, EstimatedMinutes: &estimatedMinutes}
	return item, item.Validate()
}

func NewActionItemWithDetails(
	id ActionItemID,
	taskID TaskID,
	title string,
	description string,
	dueDate time.Time,
	completed bool,
	position int,
	planning ...ActionItemPlanning,
) (ActionItem, error) {
	item := ActionItem{
		ID:          id,
		TaskID:      taskID,
		Title:       title,
		Description: description,
		DueDate:     dueDate,
		Completed:   completed,
		Position:    position,
	}
	if len(planning) > 0 {
		item.EstimatedMinutes = copyActionItemMinutes(planning[0].EstimatedMinutes)
		item.ActualMinutes = copyActionItemMinutes(planning[0].ActualMinutes)
		item.Priority = planning[0].Priority
	}
	return item, item.Validate()
}

type ActionItemPlanning struct {
	EstimatedMinutes *int
	ActualMinutes    *int
	Priority         TaskPriority
}

func copyActionItemMinutes(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func NewExistingActionItem(
	id ActionItemID,
	taskID TaskID,
	title string,
	description string,
	dueDate time.Time,
	completed bool,
	position int,
	createdAt time.Time,
	updatedAt time.Time,
	planning ...ActionItemPlanning,
) (ActionItem, error) {
	item, err := NewActionItemWithDetails(id, taskID, title, description, dueDate, completed, position, planning...)
	if err != nil {
		return NewZeroActionItem(), err
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func NewZeroActionItem() ActionItem {
	return ActionItem{}
}

func (i ActionItem) IsZero() bool {
	return i.ID == ""
}

func (i ActionItem) Validate() error {
	if i.ID == "" {
		return ErrActionItemIDEmpty
	}
	if i.TaskID == "" {
		return ErrActionItemTaskIDEmpty
	}
	if strings.TrimSpace(i.Title) == "" {
		return ErrActionItemTitleEmpty
	}
	if i.Position < 0 {
		return ErrActionItemPositionLess
	}
	if i.EstimatedMinutes == nil || *i.EstimatedMinutes < 1 || int64(*i.EstimatedMinutes) > math.MaxInt32 {
		return ErrActionItemEstimatedMinutesInvalid
	}
	if i.ActualMinutes != nil && (*i.ActualMinutes < 0 || int64(*i.ActualMinutes) > math.MaxInt32) {
		return ErrActionItemActualMinutesInvalid
	}
	if i.Priority != (TaskPriority{}) {
		if err := i.Priority.validate(); err != nil {
			return err
		}
	}
	return nil
}

type ActionItems []ActionItem

func (items ActionItems) SortByPosition() ActionItems {
	sorted := make(ActionItems, len(items))
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

func (items ActionItems) HasIncomplete() bool {
	for _, item := range items {
		if !item.Completed {
			return true
		}
	}
	return false
}
