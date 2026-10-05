package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

// Recurring TodoItems are virtual occurrences generated from their series
// rule. This compatibility type intentionally performs no persistence.
//
// Deprecated: callers should rely on list expansion instead.
type EnsureTodoItemOccurrencesUseCase struct{}

func NewEnsureTodoItemOccurrencesUseCase(_ shared.ID) *EnsureTodoItemOccurrencesUseCase {
	return &EnsureTodoItemOccurrencesUseCase{}
}

func (*EnsureTodoItemOccurrencesUseCase) Execute(context.Context, Repositories, domain.UserID, domain.TodoItem, time.Time) error {
	return nil
}
