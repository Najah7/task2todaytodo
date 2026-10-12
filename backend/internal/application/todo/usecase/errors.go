package usecase

import "errors"

var (
	ErrNotFound           = errors.New("todo item not found")
	ErrRevisionConflict   = errors.New("todo revision does not match")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrActionItemNotFound = errors.New("action item not found")
	ErrTaskNotFound       = errors.New("task not found")
	ErrFrameNotFound      = errors.New("todo frame not found")
	ErrFrameCompleted     = errors.New("completed todo frame cannot be changed")
	ErrActionItemStarted  = errors.New("started action item cannot be changed")
	ErrTaskStarted        = errors.New("started task cannot be changed")
	ErrScheduleCompleted  = errors.New("completed schedule cannot be hidden")
	ErrAllDayBlocksAction = errors.New("visible all-day schedule blocks action items")
)
