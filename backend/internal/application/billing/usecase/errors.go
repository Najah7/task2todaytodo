package usecase

import "errors"

var (
	ErrDisabled       = errors.New("billing is not configured")
	ErrConflict       = errors.New("billing operation conflicts with current subscription")
	ErrNotFound       = errors.New("billing account not found")
	ErrInvalidPlan    = errors.New("billing plan is invalid")
	ErrPaymentPending = errors.New("plan change is awaiting payment")
)
