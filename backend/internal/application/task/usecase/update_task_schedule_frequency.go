package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpdateTaskScheduleFrequencyInput struct {
	UserID         domain.UserID
	TaskID         domain.TaskID
	TaskScheduleID domain.TaskScheduleID
	IntervalWeeks  int
	Frequencies    []string
}

type UpdateTaskScheduleFrequencyUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateTaskScheduleFrequencyUseCase(uow UOW, logger logging.Logger) *UpdateTaskScheduleFrequencyUseCase {
	return &UpdateTaskScheduleFrequencyUseCase{logger: logging.OrNop(logger), uow: uow, now: time.Now}
}

func (uc *UpdateTaskScheduleFrequencyUseCase) Execute(ctx context.Context, input UpdateTaskScheduleFrequencyInput) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateTaskScheduleFrequencyUseCase.Execute", err) }()

	asOf := uc.now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		mutate := func() error {
			root, err := getTaskScheduleForCommand(ctx, repos.TaskSchedules(), input.UserID, input.TaskID, input.TaskScheduleID, shared.TaskScheduleUpdate())
			if err != nil {
				return err
			}
			if root.ID != string(input.TaskScheduleID) || root.SeriesID != root.ID || root.TaskID != string(input.TaskID) {
				return domain.ErrTaskScheduleNotFound
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
			if _, err := domain.NewTaskScheduleWithDetails(domain.TaskScheduleID(root.ID), input.TaskID, root.Title, root.Description, root.Location, input.IntervalWeeks, frequencies, time.Unix(root.StartAt, 0), time.Unix(root.EndAt, 0)); err != nil {
				return err
			}
			writer, ok := repos.TaskSchedules().(taskScheduleRecurrenceWriter)
			if !ok {
				return ErrOccurrenceInactive
			}
			if input.IntervalWeeks == domain.OnceIntervalWeeks {
				if !taskScheduleIsRecurring(root) {
					return nil
				}
				return writer.StopTaskScheduleRecurrence(ctx, input.UserID, input.TaskID, input.TaskScheduleID)
			}
			today := localToday(asOf, location)
			return writer.SetTaskScheduleRecurrence(ctx, input.UserID, input.TaskID, input.TaskScheduleID, today, input.IntervalWeeks, frequenciesDAO(frequencies))
		}
		return withTaskProgressMutationForPermission(ctx, repos, input.UserID, input.TaskID, asOf, shared.TaskScheduleUpdate(), mutate)
	})

}
