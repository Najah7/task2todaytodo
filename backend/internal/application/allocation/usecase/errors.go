package usecase

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrConflict           = errors.New("allocation availability conflict")
	ErrPreconditionFailed = errors.New("allocation revision mismatch")
	ErrNotFound           = errors.New("allocation not found")
	ErrProjectUnavailable = errors.New("project unavailable")
	ErrInvalidRequest     = errors.New("invalid allocation request")
	ErrNoConflict         = errors.New("no allocation conflict found")
)

type ConflictError struct {
	Kind           string
	RuleID         string
	OccurrenceDate time.Time
	ProjectID      string
	StartAt        time.Time
	EndAt          time.Time
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("%s conflicts with rule %s on %s", e.Kind, e.RuleID, e.OccurrenceDate.Format("2006-01-02"))
}

func (e ConflictError) Unwrap() error { return ErrConflict }
