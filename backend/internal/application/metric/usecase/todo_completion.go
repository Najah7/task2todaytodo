package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/metric/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var (
	ErrInvalidDays       = errors.New("invalid metrics days")
	ErrInvalidActor      = errors.New("invalid metrics actor")
	ErrReaderUnavailable = errors.New("metrics reader unavailable")
	ErrPaidPlanRequired  = errors.New("paid plan required")
)

type TodoCompletionReader interface {
	ReadTodoCompletion(context.Context, string, int) (dao.TodoCompletion, error)
}

type TodoCompletionUseCase struct {
	reader TodoCompletionReader
	access shared.PlanAccessReader
}

func NewTodoCompletionUseCase(reader TodoCompletionReader, access ...shared.PlanAccessReader) *TodoCompletionUseCase {
	uc := &TodoCompletionUseCase{reader: reader}
	if len(access) > 0 {
		uc.access = access[0]
	}
	return uc
}

func (uc *TodoCompletionUseCase) Execute(ctx context.Context, actorID string, days int) (dao.TodoCompletion, error) {
	if actorID == "" {
		return dao.TodoCompletion{}, ErrInvalidActor
	}
	if uc.reader == nil {
		return dao.TodoCompletion{}, ErrReaderUnavailable
	}
	if days != 7 && days != 30 {
		return dao.TodoCompletion{}, ErrInvalidDays
	}
	if uc.access == nil {
		return dao.TodoCompletion{}, ErrPaidPlanRequired
	}
	plan, err := uc.access.GetPlanAccess(ctx, actorID)
	if err != nil {
		return dao.TodoCompletion{}, err
	}
	if !plan.IsPaid() {
		return dao.TodoCompletion{}, ErrPaidPlanRequired
	}
	result, err := uc.reader.ReadTodoCompletion(ctx, actorID, days)
	if err != nil {
		return dao.TodoCompletion{}, err
	}
	result.TotalPlannedItems = 0
	result.CompletedItems = 0
	for index := range result.Days {
		day := &result.Days[index]
		result.TotalPlannedItems += day.PlannedItems
		result.CompletedItems += day.CompletedItems
		day.CompletionRatio = completionRatio(day.CompletedItems, day.PlannedItems)
	}
	result.CompletionRatio = completionRatio(result.CompletedItems, result.TotalPlannedItems)
	return result, nil
}

func completionRatio(completed, planned int32) *float64 {
	if planned == 0 {
		return nil
	}
	value := float64(completed) / float64(planned)
	return &value
}
