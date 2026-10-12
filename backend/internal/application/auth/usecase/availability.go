package usecase

import (
	"context"
	"errors"
)

var ErrAvailabilityValidationUnavailable = errors.New("allocation availability validation unavailable")

// UserAvailabilityValidator checks proposed profile settings against future
// allocation windows. The application root binds this port to the same UOW.
type UserAvailabilityValidator interface {
	LockUser(ctx context.Context, userID string) error
	ValidateUserProfile(ctx context.Context, userID, timezone, wakeUpTime, bedtime string) error
}
