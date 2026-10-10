package domain

import (
	"errors"
	"strings"
	"time"

	calendar "github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

var (
	ErrActionItemIDEmpty                 = errors.New("action item ID cannot be empty")
	ErrActionItemTaskIDEmpty             = errors.New("action item task ID cannot be empty")
	ErrActionItemTitleEmpty              = errors.New("action item title cannot be empty")
	ErrActionItemPositionLess            = errors.New("action item position must be greater than or equal to 0")
	ErrActionItemIntervalWeeksLess       = errors.New("action item interval weeks must be greater than or equal to 0")
	ErrActionItemEstimatedMinutesInvalid = errors.New("action item estimated minutes must be greater than or equal to 0")
)

type ActionItemID string

type ActionItem struct {
	ID               ActionItemID
	TaskID           TaskID
	Title            string
	Description      string
	DueDate          time.Time
	EstimatedMinutes *int
	Priority         TaskPriority
	Completed        bool
	Position         int
	IntervalWeeks    int
	Frequencies      TaskFrequencies
	// SeriesID points to the original row. The original row is its own series root.
	SeriesID       ActionItemID
	OccurrenceDate time.Time
	Timezone       string
	IsException    bool
	Deleted        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewActionItem(
	id ActionItemID,
	taskID TaskID,
	title string,
) (ActionItem, error) {
	item := ActionItem{
		ID:            id,
		TaskID:        taskID,
		Title:         title,
		IntervalWeeks: OnceIntervalWeeks,
	}
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
	intervalWeeks int,
	frequencies TaskFrequencies,
	planning ...ActionItemPlanning,
) (ActionItem, error) {
	item := ActionItem{
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
	if len(planning) > 0 {
		item.EstimatedMinutes = copyActionItemMinutes(planning[0].EstimatedMinutes)
		item.Priority = planning[0].Priority
	}
	return item, item.Validate()
}

type ActionItemPlanning struct {
	EstimatedMinutes *int
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
	intervalWeeks int,
	frequencies TaskFrequencies,
	createdAt time.Time,
	updatedAt time.Time,
	recurrence ...RecurrenceMetadata,
) (ActionItem, error) {
	return NewExistingActionItemWithPlanning(id, taskID, title, description, dueDate, completed, position, intervalWeeks, frequencies, createdAt, updatedAt, ActionItemPlanning{}, recurrence...)
}

func NewExistingActionItemWithPlanning(
	id ActionItemID,
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
	planning ActionItemPlanning,
	recurrence ...RecurrenceMetadata,
) (ActionItem, error) {
	item := ActionItem{
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
	item.EstimatedMinutes = copyActionItemMinutes(planning.EstimatedMinutes)
	item.Priority = planning.Priority
	if len(recurrence) > 0 {
		item.SeriesID = ActionItemID(recurrence[0].SeriesID)
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
		return NewZeroActionItem(), err
	}
	if len(recurrence) > 0 {
		return NewActionItemWithRecurrence(item)
	}

	return item, nil
}

func NewZeroActionItem() ActionItem {
	return ActionItem{}
}

// NewActionItemWithRecurrence restores a row with recurrence lineage metadata.
func NewActionItemWithRecurrence(item ActionItem) (ActionItem, error) {
	if item.SeriesID == "" {
		item.SeriesID = item.ID
	}
	item.OccurrenceDate = calendar.NormalizeCalendarDate(item.OccurrenceDate)
	if err := item.Validate(); err != nil {
		return NewZeroActionItem(), err
	}
	if item.OccurrenceDate.IsZero() || strings.TrimSpace(item.Timezone) == "" {
		return NewZeroActionItem(), ErrRecurrenceMetadataInvalid
	}
	if _, err := time.LoadLocation(item.Timezone); err != nil {
		return NewZeroActionItem(), ErrRecurrenceTimezoneInvalid
	}
	return item, nil
}

func (i ActionItem) WithRecurrence(metadata RecurrenceMetadata) (ActionItem, error) {
	i.SeriesID = ActionItemID(metadata.SeriesID)
	i.OccurrenceDate = metadata.OccurrenceDate
	i.Timezone = metadata.Timezone
	i.IsException = metadata.IsException
	i.Deleted = metadata.Deleted
	return NewActionItemWithRecurrence(i)
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
	if i.IntervalWeeks < 0 {
		return ErrActionItemIntervalWeeksLess
	}
	if i.EstimatedMinutes != nil && *i.EstimatedMinutes < 0 {
		return ErrActionItemEstimatedMinutesInvalid
	}
	if i.Priority != (TaskPriority{}) {
		if err := i.Priority.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (i ActionItem) IsWeekly() bool {
	return i.IntervalWeeks == WeeklyIntervalWeeks
}

func (i ActionItem) IsEveryWeekday() bool {
	return i.Frequencies.IsWeekday() && i.IsWeekly()
}

func (i ActionItem) IsEveryWeekend() bool {
	return i.Frequencies.IsWeekend() && i.IsWeekly()
}

func (i ActionItem) IsBiWeekly() bool {
	return i.IntervalWeeks == BiWeeklyIntervalWeeks
}

func (i ActionItem) IsEveryFourWeeks() bool {
	return i.IntervalWeeks == FourWeekIntervalWeeks
}

func (i ActionItem) IsQuarterly() bool {
	return i.IntervalWeeks == QuarterlyIntervalWeeks
}

func (i ActionItem) IsSemiAnnually() bool {
	return i.IntervalWeeks == SemiAnnualIntervalWeeks
}

func (i ActionItem) IsAnnually() bool {
	return i.IntervalWeeks == AnnualIntervalWeeks
}

func (i ActionItem) IsOnce() bool {
	return i.IntervalWeeks == 0
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
