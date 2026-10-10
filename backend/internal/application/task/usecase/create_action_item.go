package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	calendar "github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CreateActionItemInput struct {
	ID               domain.ActionItemID
	UserID           domain.UserID
	TaskID           domain.TaskID
	Title            string
	Description      string
	DueDate          time.Time
	IntervalWeeks    int
	Frequencies      []string
	EstimatedMinutes *int
	Priority         string
}

type CreateActionItemUseCase struct {
	uow       UOW
	timezones UserTimezoneReader
	clock     func() time.Time
	logger    logging.Logger
}

func NewCreateActionItemUseCase(uow UOW, timezones UserTimezoneReader, logger logging.Logger) *CreateActionItemUseCase {
	return &CreateActionItemUseCase{logger: logging.OrNop(logger), uow: uow, timezones: timezones, clock: time.Now}
}

func (uc *CreateActionItemUseCase) Execute(ctx context.Context, input CreateActionItemInput) (output dao.ActionItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateActionItemUseCase.Execute", err) }()

	var created dao.ActionItem
	asOf := uc.clock()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, input.UserID, input.TaskID, asOf, shared.ActionItemCreate(), func(task dao.Task, _ taskProjectMutationSnapshot) error {
			timezone, err := uc.timezones.GetTimezone(ctx, string(input.UserID))
			if err != nil {
				return err
			}
			if _, err := time.LoadLocation(timezone); err != nil {
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
			priorityValue := input.Priority
			if priorityValue == "" {
				priorityValue = task.Priority.Value
			}
			priority := domain.TaskPriority{}
			if priorityValue != "" {
				priority, err = domain.NewTaskPriority(priorityValue)
				if err != nil {
					return err
				}
			}

			item, err := domain.NewActionItemWithDetails(
				input.ID,
				input.TaskID,
				input.Title,
				input.Description,
				input.DueDate,
				false,
				0,
				input.IntervalWeeks,
				frequencies,
				domain.ActionItemPlanning{EstimatedMinutes: input.EstimatedMinutes, Priority: priority},
			)
			if err != nil {
				return err
			}

			firstDate := recurrenceDate(input.DueDate, asOf, timezone)
			item, err = item.WithRecurrence(domain.RecurrenceMetadata{
				SeriesID:       string(input.ID),
				OccurrenceDate: firstDate,
				Timezone:       timezone,
			})
			if err != nil {
				return err
			}

			created, err = repos.ActionItems().CreateForOwnedTask(ctx, input.UserID, item, true)
			return err
		})
	})
	if err != nil {
		return dao.ActionItem{}, err
	}
	return created, nil

}

func recurrenceDate(dueDate, now time.Time, timezone string) time.Time {
	date := dueDate
	if date.IsZero() {
		location, err := time.LoadLocation(timezone)
		if err == nil {
			date = now.In(location)
		} else {
			date = now
		}
	}
	return calendar.NormalizeCalendarDate(date)
}
