package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CreateTaskScheduleInput struct {
	ID            domain.TaskScheduleID
	UserID        domain.UserID
	TaskID        domain.TaskID
	Title         string
	Description   string
	Location      string
	StartAt       time.Time
	EndAt         time.Time
	IntervalWeeks int
	Frequencies   []string
}

type CreateTaskScheduleUseCase struct {
	uow       UOW
	timezones UserTimezoneReader
	logger    logging.Logger
}

func NewCreateTaskScheduleUseCase(uow UOW, timezones UserTimezoneReader, logger logging.Logger) *CreateTaskScheduleUseCase {
	return &CreateTaskScheduleUseCase{logger: logging.OrNop(logger), uow: uow, timezones: timezones}
}

func (uc *CreateTaskScheduleUseCase) Execute(ctx context.Context, input CreateTaskScheduleInput) (output dao.TaskSchedule, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateTaskScheduleUseCase.Execute", err) }()

	var created dao.TaskSchedule
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, input.UserID, input.TaskID, asOf, shared.TaskScheduleCreate(), func() error {
			timezone, err := uc.timezones.GetTimezone(ctx, input.UserID)
			if err != nil {
				return err
			}
			location, err := time.LoadLocation(timezone)
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

			root, err := domain.NewTaskScheduleWithDetails(
				input.ID,
				input.TaskID,
				input.Title,
				input.Description,
				input.Location,
				input.IntervalWeeks,
				frequencies,
				input.StartAt,
				input.EndAt,
			)
			if err != nil {
				return err
			}

			startLocal := input.StartAt.In(location)
			firstDate := tasktime.NormalizeCalendarDate(startLocal)
			root, err = root.WithRecurrence(domain.RecurrenceMetadata{
				SeriesID:       string(input.ID),
				OccurrenceDate: firstDate,
				Timezone:       timezone,
			})
			if err != nil {
				return err
			}
			created, err = repos.TaskSchedules().CreateByTaskAndUserID(ctx, input.UserID, root)
			return err
		})
	})
	if err != nil {
		return dao.TaskSchedule{}, err
	}
	return created, nil

}
