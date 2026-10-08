package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CreateScheduleInput struct {
	ID            domain.ScheduleID
	UserID        domain.UserID
	ProjectID     domain.ProjectID
	Title         string
	Description   string
	Location      string
	StartAt       time.Time
	EndAt         time.Time
	IntervalWeeks int
	Frequencies   []string
}

type CreateScheduleUseCase struct {
	uow       UOW
	timezones UserTimezoneReader
	logger    logging.Logger
}

func NewCreateScheduleUseCase(uow UOW, timezones UserTimezoneReader, logger logging.Logger) *CreateScheduleUseCase {
	return &CreateScheduleUseCase{uow: uow, timezones: timezones, logger: logging.OrNop(logger)}
}

func (uc *CreateScheduleUseCase) Execute(ctx context.Context, input CreateScheduleInput) (output dao.Schedule, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "CreateScheduleUseCase.Execute", err) }()
	var created dao.Schedule
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		timezone, err := uc.timezones.GetTimezone(ctx, string(input.UserID))
		if err != nil {
			return err
		}
		location, err := time.LoadLocation(timezone)
		if err != nil {
			return recurrence.ErrRecurrenceTimezoneInvalid
		}
		frequencies := make(domain.Frequencies, 0, len(input.Frequencies))
		for _, value := range input.Frequencies {
			frequency, err := recurrence.NewFrequency(value)
			if err != nil {
				return err
			}
			frequencies = append(frequencies, frequency)
		}
		root, err := domain.NewScheduleWithDetails(input.ID, input.UserID, input.ProjectID, input.UserID, input.Title, input.Description, input.Location,
			input.IntervalWeeks, frequencies, input.StartAt, input.EndAt)
		if err != nil {
			return err
		}
		firstDate := time.Date(input.StartAt.In(location).Year(), input.StartAt.In(location).Month(), input.StartAt.In(location).Day(), 0, 0, 0, 0, time.UTC)
		root, err = root.WithRecurrence(recurrence.Metadata{SeriesID: string(input.ID), OccurrenceDate: firstDate, Timezone: timezone})
		if err != nil {
			return err
		}
		if input.ProjectID != "" {
			if err := requireScheduleProjectPermission(ctx, repos.Schedules(), input.UserID, input.ProjectID, shared.ScheduleCreate()); err != nil {
				return err
			}
		}
		before, err := lockAndCaptureScheduleProjects(ctx, repos, []string{string(input.ProjectID)}, asOf)
		if err != nil {
			return err
		}
		if input.ProjectID != "" {
			if err := repos.Schedules().LockProjectForScheduleMutation(ctx, input.ProjectID); err != nil {
				return err
			}
		}
		created, err = repos.Schedules().CreateByUserID(ctx, input.UserID, root)
		if err != nil {
			return err
		}
		return finishScheduleProjectMutation(ctx, repos, input.UserID, before, asOf)
	})
	if err != nil {
		return dao.Schedule{}, err
	}
	logScheduleStateChange(uc.logger, ctx, "schedule.create", input.UserID, input.ID)
	return created, nil
}
