package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListSchedulesUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewListSchedulesUseCase(uow UOW, logger logging.Logger, clocks ...func() time.Time) *ListSchedulesUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &ListSchedulesUseCase{uow: uow, now: now, logger: logging.OrNop(logger)}
}

func (uc *ListSchedulesUseCase) ExecutePage(ctx context.Context, actorID domain.UserID, request CursorPageRequest) (output CursorPage[dao.Schedule], err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "ListSchedulesUseCase.ExecutePage", err) }()
	return listSchedulePage(ctx, uc.uow, uc.now, actorID, "", request)
}

type ListProjectSchedulesUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewListProjectSchedulesUseCase(uow UOW, logger logging.Logger, clocks ...func() time.Time) *ListProjectSchedulesUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &ListProjectSchedulesUseCase{uow: uow, now: now, logger: logging.OrNop(logger)}
}

func (uc *ListProjectSchedulesUseCase) ExecutePage(ctx context.Context, actorID domain.UserID, projectID domain.ProjectID, request CursorPageRequest) (output CursorPage[dao.Schedule], err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "ListProjectSchedulesUseCase.ExecutePage", err) }()
	return listSchedulePage(ctx, uc.uow, uc.now, actorID, string(projectID), request)
}

func listSchedulePage(ctx context.Context, uow UOW, now func() time.Time, actorID domain.UserID, projectID string, request CursorPageRequest) (CursorPage[dao.Schedule], error) {
	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.Schedule]{}, err
	}
	var rows []dao.Schedule
	skipped := make(map[string]map[string]bool)
	if err := uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		if projectID == "" {
			rows, err = repos.Schedules().ListByAssigneeUserID(ctx, actorID)
		} else {
			rows, err = repos.Schedules().ListByProjectAndUserID(ctx, actorID, domain.ProjectID(projectID))
		}
		if err != nil {
			return err
		}
		for _, schedule := range rows {
			if schedule.ID != schedule.SeriesID {
				continue
			}
			dates, err := repos.Schedules().ListSkippedOccurrencesForPermission(ctx, actorID, domain.ScheduleID(schedule.ID), shared.ScheduleRead())
			if err != nil {
				return err
			}
			set := make(map[string]bool, len(dates))
			for _, date := range dates {
				set[time.Unix(date, 0).UTC().Format("2006-01-02")] = true
			}
			skipped[schedule.SeriesID] = set
		}
		return nil
	}); err != nil {
		return CursorPage[dao.Schedule]{}, err
	}
	asOf, err := listReferenceTime(request, now())
	if err != nil {
		return CursorPage[dao.Schedule]{}, err
	}
	rows, err = expandScheduleRowsWithSkipped(rows, request, asOf, skipped)
	if err != nil {
		return CursorPage[dao.Schedule]{}, err
	}
	filtered := rows[:0]
	for _, schedule := range rows {
		if afterScheduleAnchor(schedule, request.Anchor) {
			filtered = append(filtered, schedule)
		}
	}
	selected, more := pagination.Window(filtered, request.Size)
	page := CursorPage[dao.Schedule]{Items: selected}
	if more && len(selected) > 0 {
		last := selected[len(selected)-1]
		page.Next = &CursorAnchor{At: last.CursorStartAt, ID: last.ID, SeriesID: last.SeriesID, Date: last.OccurrenceDate, OccurrenceDate: last.OccurrenceDate, AsOf: asOf.Format(time.RFC3339Nano)}
	}
	return page, nil
}
