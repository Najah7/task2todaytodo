package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListTaskSchedulesUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewListTaskSchedulesUseCase(uow UOW, logger logging.Logger, clocks ...func() time.Time) *ListTaskSchedulesUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &ListTaskSchedulesUseCase{logger: logging.OrNop(logger), uow: uow, now: now}
}

func (uc *ListTaskSchedulesUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (output []dao.TaskSchedule, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTaskSchedulesUseCase.Execute", err) }()

	var schedules []dao.TaskSchedule
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		_, err := repos.Tasks().GetByUserIDWithPermission(ctx, userID, taskID, shared.TaskScheduleRead())
		if err != nil {
			return err
		}

		schedules, err = repos.TaskSchedules().ListByTaskAndUserID(ctx, userID, taskID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if schedules == nil {
		schedules = []dao.TaskSchedule{}
	}
	return schedules, nil

}

func (uc *ListTaskSchedulesUseCase) ExecutePage(ctx context.Context, userID domain.UserID, taskID domain.TaskID, request CursorPageRequest) (output CursorPage[dao.TaskSchedule], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTaskSchedulesUseCase.ExecutePage", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.TaskSchedule]{}, err
	}
	var schedules []dao.TaskSchedule
	var taskStatus string
	skipped := make(map[string]map[string]bool)
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		task, err := repos.Tasks().GetByUserIDWithPermission(ctx, userID, taskID, shared.TaskScheduleRead())
		if err != nil {
			return err
		}
		taskStatus = task.Status.Value
		schedules, err = listTaskScheduleOccurrenceProjection(ctx, repos.TaskSchedules(), userID, taskID)
		if err != nil {
			return err
		}
		if store, ok := repos.TaskSchedules().(scheduleSkippedOccurrenceStore); ok {
			for _, schedule := range schedules {
				if schedule.ID != schedule.SeriesID {
					continue
				}
				dates, err := store.ListTaskScheduleSkippedOccurrences(ctx, userID, taskID, domain.TaskScheduleID(schedule.ID))
				if err != nil {
					return err
				}
				set := make(map[string]bool, len(dates))
				for _, date := range dates {
					set[time.Unix(date, 0).UTC().Format("2006-01-02")] = true
				}
				skipped[schedule.SeriesID] = set
			}
		}
		return nil
	})
	if err != nil {
		return CursorPage[dao.TaskSchedule]{}, err
	}
	asOf, err := listReferenceTime(request, uc.now())
	if err != nil {
		return CursorPage[dao.TaskSchedule]{}, err
	}
	schedules, err = expandTaskScheduleRowsWithSkipped(schedules, request, asOf, taskStatus == "done", skipped)
	if err != nil {
		return CursorPage[dao.TaskSchedule]{}, err
	}
	filtered := schedules[:0]
	for _, schedule := range schedules {
		if afterTaskScheduleAnchor(schedule, request.Anchor) {
			filtered = append(filtered, schedule)
		}
	}
	selected, more := pagination.Window(filtered, request.Size)
	page := CursorPage[dao.TaskSchedule]{Items: selected}
	if more && len(selected) > 0 {
		last := selected[len(selected)-1]
		page.Next = &CursorAnchor{At: last.CursorStartAt, ID: last.ID, SeriesID: last.SeriesID, Date: last.OccurrenceDate, OccurrenceDate: last.OccurrenceDate, AsOf: asOf.Format(time.RFC3339Nano)}
	}
	return page, nil

}
