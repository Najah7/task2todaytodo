package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

// Recurring ActionItems are virtual occurrences generated from their series
// rule. This compatibility type intentionally performs no persistence.
//
// Deprecated: callers should rely on list expansion instead.
type EnsureActionItemOccurrencesUseCase struct{}

func NewEnsureActionItemOccurrencesUseCase(_ shared.ID) *EnsureActionItemOccurrencesUseCase {
	return &EnsureActionItemOccurrencesUseCase{}
}

func (*EnsureActionItemOccurrencesUseCase) Execute(context.Context, Repositories, domain.UserID, domain.ActionItem, time.Time) error {
	return nil
}
