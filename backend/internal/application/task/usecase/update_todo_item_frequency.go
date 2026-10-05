package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpdateTodoItemFrequencyInput struct {
	UserID        domain.UserID
	TaskID        domain.TaskID
	TodoItemID    domain.TodoItemID
	IntervalWeeks int
	Frequencies   []string
}

type UpdateTodoItemFrequencyUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateTodoItemFrequencyUseCase(uow UOW, logger logging.Logger) *UpdateTodoItemFrequencyUseCase {
	return &UpdateTodoItemFrequencyUseCase{logger: logging.OrNop(logger), uow: uow, now: time.Now}
}

func (uc *UpdateTodoItemFrequencyUseCase) Execute(ctx context.Context, input UpdateTodoItemFrequencyInput) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateTodoItemFrequencyUseCase.Execute", err) }()

	asOf := uc.now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		mutate := func() error {
			root, err := getTodoItemForCommand(ctx, repos.TodoItems(), input.UserID, input.TaskID, input.TodoItemID, shared.TodoItemUpdate())
			if err != nil {
				return err
			}
			if root.ID != string(input.TodoItemID) || root.SeriesID != root.ID || root.TaskID != string(input.TaskID) {
				return ErrTodoItemNotFound
			}
			location, err := time.LoadLocation(root.Timezone)
			if err != nil {
				return domain.ErrRecurrenceTimezoneInvalid
			}
			frequencies := make(domain.TaskFrequencies, 0, len(input.Frequencies))
			for _, value := range input.Frequencies {
				frequency, err := domain.NewTaskFrequency(value)
				if err != nil {
					return err
				}
				frequencies = append(frequencies, frequency)
			}
			var due time.Time
			if root.DueDate != 0 {
				due = time.Unix(root.DueDate, 0).UTC()
			}
			if _, err := domain.NewTodoItemWithDetails(domain.TodoItemID(root.ID), input.TaskID, root.Title, root.Description, due, root.Completed, root.Position, input.IntervalWeeks, frequencies); err != nil {
				return err
			}
			writer, ok := repos.TodoItems().(todoItemRecurrenceWriter)
			if !ok {
				return ErrOccurrenceInactive
			}
			if input.IntervalWeeks == domain.OnceIntervalWeeks {
				if !todoItemIsRecurring(root) {
					return nil
				}
				return writer.StopTodoItemRecurrence(ctx, input.UserID, input.TaskID, input.TodoItemID)
			}
			today := localToday(asOf, location)
			return writer.SetTodoItemRecurrence(ctx, input.UserID, input.TaskID, input.TodoItemID, today, input.IntervalWeeks, frequenciesDAO(frequencies))
		}
		return withTaskProgressMutationForPermission(ctx, repos, input.UserID, input.TaskID, asOf, shared.TodoItemUpdate(), mutate)
	})

}
