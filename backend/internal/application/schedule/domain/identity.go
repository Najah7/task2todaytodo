package domain

import "errors"

var ErrScheduleNotFound = errors.New("schedule not found")

type ScheduleID string
type UserID string
type ProjectID string
