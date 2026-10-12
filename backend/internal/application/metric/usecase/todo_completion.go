package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/metric/dao"
)

var (
	ErrInvalidDays       = errors.New("invalid metrics days")
	ErrInvalidActor      = errors.New("invalid metrics actor")
	ErrReaderUnavailable = errors.New("metrics reader unavailable")
)

type TodoCompletionReader interface {
	ReadTodoCompletion(context.Context, string, int) (dao.TodoCompletion, error)
}

type TodoCompletionUseCase struct {
	reader TodoCompletionReader
}

func NewTodoCompletionUseCase(reader TodoCompletionReader) *TodoCompletionUseCase {
	return &TodoCompletionUseCase{reader: reader}
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
