package domain

import "time"

type ScheduleInsertion struct {
	ItemID string
	Start  time.Time
}

// InsertSchedulesByStart removes every pending target before applying the
// insertion order. This prevents an unprocessed target's stale display
// position from acting as an anchor for another target.
func InsertSchedulesByStart(items []TodoItem, insertions []ScheduleInsertion) ([]TodoItem, error) {
	ordered := append([]TodoItem(nil), items...)
	pending := make(map[string]ScheduleInsertion, len(insertions))
	for _, insertion := range insertions {
		if insertion.ItemID == "" {
			return nil, ErrInvalidTodoItem
		}
		if _, exists := pending[insertion.ItemID]; exists {
			return nil, ErrInvalidTodoItem
		}
		pending[insertion.ItemID] = insertion
	}
	targets := make(map[string]TodoItem, len(pending))
	remaining := make([]TodoItem, 0, len(items))
	for _, item := range ordered {
		insertion, exists := pending[item.ID]
		if !exists {
			remaining = append(remaining, item)
			continue
		}
		if item.Kind != FrameSchedule || item.Hidden || item.Start == nil {
			return nil, ErrInvalidTodoItem
		}
		item.Start = &insertion.Start
		targets[item.ID] = item
		delete(pending, item.ID)
	}
	if len(pending) != 0 {
		return nil, ErrInvalidTodoItem
	}
	for index := len(insertions) - 1; index >= 0; index-- {
		insertion := insertions[index]
		target := targets[insertion.ItemID]
		target.Position = len(remaining)
		remaining = append(remaining, target)
		var err error
		remaining, err = InsertScheduleByStart(remaining, insertion.ItemID, insertion.Start)
		if err != nil {
			return nil, err
		}
	}
	return remaining, nil
}

// InsertScheduleByStart places a visible Schedule before the earliest visible
// item whose saved start is at or after the Schedule start. Untimed work never
// anchors the insertion. Other visible items retain their relative order.
func InsertScheduleByStart(items []TodoItem, scheduleID string, start time.Time) ([]TodoItem, error) {
	ordered := append([]TodoItem(nil), items...)
	var target *TodoItem
	for index := range ordered {
		if ordered[index].ID == scheduleID {
			copy := ordered[index]
			target = &copy
			break
		}
	}
	if target == nil || target.Kind != FrameSchedule || target.Hidden || target.Start == nil {
		return nil, ErrInvalidTodoItem
	}
	target.Start = &start
	visible := make([]TodoItem, 0, len(ordered))
	hidden := make([]TodoItem, 0)
	for _, item := range ordered {
		if item.Hidden {
			hidden = append(hidden, item)
			continue
		}
		if item.ID != scheduleID {
			visible = append(visible, item)
		}
	}
	anchor := -1
	var anchorTime time.Time
	for index, item := range visible {
		if item.Start == nil || item.Start.Before(start) {
			continue
		}
		if anchor < 0 || item.Start.Before(anchorTime) || (item.Start.Equal(anchorTime) && item.Position < visible[anchor].Position) {
			anchor, anchorTime = index, *item.Start
		}
	}
	insertAt := len(visible)
	if anchor >= 0 {
		insertAt = anchor
	}
	visible = append(visible, TodoItem{})
	copy(visible[insertAt+1:], visible[insertAt:])
	visible[insertAt] = *target
	result := append(visible, hidden...)
	for index := range result {
		result[index].Position = index
	}
	return result, nil
}
