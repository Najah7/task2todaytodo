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

type UpdateScheduleFrequencyInput struct {
	UserID        domain.UserID
	ScheduleID    domain.ScheduleID
	IntervalWeeks int
	Frequencies   []string
}

type UpdateScheduleFrequencyUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateScheduleFrequencyUseCase(uow UOW, logger logging.Logger) *UpdateScheduleFrequencyUseCase {
	return &UpdateScheduleFrequencyUseCase{uow: uow, now: time.Now, logger: logging.OrNop(logger)}
}

func (uc *UpdateScheduleFrequencyUseCase) Execute(ctx context.Context, input UpdateScheduleFrequencyInput) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "UpdateScheduleFrequencyUseCase.Execute", err) }()
	asOf := uc.now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMutation(ctx, repos, input.UserID, input.ScheduleID, asOf, shared.ScheduleUpdate(), func(root dao.Schedule, _ scheduleProjectMutationSnapshots) error {
			if root.ID != string(input.ScheduleID) || root.SeriesID != root.ID {
				return domain.ErrScheduleNotFound
			}
			location, err := time.LoadLocation(root.Timezone)
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
			if _, err := domain.NewScheduleWithDetails(domain.ScheduleID(root.ID), domain.UserID(root.UserID), domain.ProjectID(root.ProjectID), domain.UserID(root.AssigneeID), root.Title, root.Description, root.Location,
				input.IntervalWeeks, frequencies, time.Unix(root.StartAt, 0), time.Unix(root.EndAt, 0)); err != nil {
				return err
			}
			writer, err := requireRecurrenceWriter(repos.Schedules())
			if err != nil {
				return err
			}
			if input.IntervalWeeks == domain.OnceIntervalWeeks {
				if !scheduleIsRecurring(root) {
					return nil
				}
				return writer.StopRecurrenceByUserID(ctx, input.UserID, input.ScheduleID)
			}
			today := localToday(asOf, location)
			return writer.SetRecurrenceByUserID(ctx, input.UserID, input.ScheduleID, today, input.IntervalWeeks, frequenciesDAO(frequencies))
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.update_frequency", input.UserID, input.ScheduleID)
	}
	return err
}
