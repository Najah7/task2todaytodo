package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpdateActionItemFrequencyInput struct {
	UserID        domain.UserID
	TaskID        domain.TaskID
	ActionItemID  domain.ActionItemID
	IntervalWeeks int
	Frequencies   []string
}

type UpdateActionItemFrequencyUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateActionItemFrequencyUseCase(uow UOW, logger logging.Logger) *UpdateActionItemFrequencyUseCase {
	return &UpdateActionItemFrequencyUseCase{logger: logging.OrNop(logger), uow: uow, now: time.Now}
}

func (uc *UpdateActionItemFrequencyUseCase) Execute(ctx context.Context, input UpdateActionItemFrequencyInput) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateActionItemFrequencyUseCase.Execute", err) }()

	asOf := uc.now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		mutate := func() error {
			root, err := getActionItemForCommand(ctx, repos.ActionItems(), input.UserID, input.TaskID, input.ActionItemID, shared.ActionItemUpdate())
			if err != nil {
				return err
			}
			if root.ID != string(input.ActionItemID) || root.SeriesID != root.ID || root.TaskID != string(input.TaskID) {
				return ErrActionItemNotFound
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
			if _, err := domain.NewActionItemWithDetails(domain.ActionItemID(root.ID), input.TaskID, root.Title, root.Description, due, root.Completed, root.Position, input.IntervalWeeks, frequencies); err != nil {
				return err
			}
			writer, ok := repos.ActionItems().(actionItemRecurrenceWriter)
			if !ok {
				return ErrOccurrenceInactive
			}
			if input.IntervalWeeks == domain.OnceIntervalWeeks {
				if !actionItemIsRecurring(root) {
					return nil
				}
				return writer.StopActionItemRecurrence(ctx, input.UserID, input.TaskID, input.ActionItemID)
			}
			today := localToday(asOf, location)
			return writer.SetActionItemRecurrence(ctx, input.UserID, input.TaskID, input.ActionItemID, today, input.IntervalWeeks, frequenciesDAO(frequencies))
		}
		return withTaskProgressMutationForPermission(ctx, repos, input.UserID, input.TaskID, asOf, shared.ActionItemUpdate(), mutate)
	})

}
