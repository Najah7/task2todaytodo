package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListTodoItemsUseCase struct {
	uow    UOW
	now    func() time.Time
	logger logging.Logger
}

func NewListTodoItemsUseCase(uow UOW, logger logging.Logger, clocks ...func() time.Time) *ListTodoItemsUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &ListTodoItemsUseCase{logger: logging.OrNop(logger), uow: uow, now: now}
}

func (uc *ListTodoItemsUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (output []dao.TodoItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTodoItemsUseCase.Execute", err) }()

	var items []dao.TodoItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		_, err := repos.Tasks().GetByUserIDWithPermission(ctx, userID, taskID, shared.TodoItemRead())
		if err != nil {
			return err
		}

		items, err = repos.TodoItems().ListByTask(ctx, userID, taskID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []dao.TodoItem{}
	}
	return items, nil

}

func (uc *ListTodoItemsUseCase) ExecutePage(ctx context.Context, userID domain.UserID, taskID domain.TaskID, request CursorPageRequest) (output CursorPage[dao.TodoItem], err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTodoItemsUseCase.ExecutePage", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.TodoItem]{}, err
	}
	var items []dao.TodoItem
	var taskStatus string
	var projectDone bool
	skipped := make(map[string]map[string]bool)
	asOf, err := listReferenceTime(request, uc.now())
	if err != nil {
		return CursorPage[dao.TodoItem]{}, err
	}
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		task, err := repos.Tasks().GetByUserIDWithPermission(ctx, userID, taskID, shared.TodoItemRead())
		if err != nil {
			return err
		}
		taskStatus = task.Status.Value
		if task.ProjectID != "" {
			lifecycle := repos.ProjectLifecycle()
			if lifecycle == nil {
				return ErrProjectLifecycleUnavailable
			}
			projectStatus, err := lifecycle.ReadStatus(ctx, task.ProjectID)
			if errors.Is(err, shared.ErrProjectUnavailable) {
				return ErrTaskProjectNotFound
			}
			if err != nil {
				return err
			}
			projectDone = projectStatus == "done"
		}
		items, err = listTodoOccurrenceProjection(ctx, repos.TodoItems(), userID, taskID)
		if err != nil {
			return err
		}
		if store, ok := repos.TodoItems().(todoSkippedOccurrenceStore); ok {
			for _, item := range items {
				if item.ID != item.SeriesID {
					continue
				}
				dates, err := store.ListTodoItemSkippedOccurrences(ctx, userID, taskID, domain.TodoItemID(item.ID))
				if err != nil {
					return err
				}
				set := make(map[string]bool, len(dates))
				for _, date := range dates {
					set[time.Unix(date, 0).UTC().Format("2006-01-02")] = true
				}
				skipped[item.SeriesID] = set
			}
		}
		return nil
	})
	if err != nil {
		return CursorPage[dao.TodoItem]{}, err
	}
	items, err = expandTodoItemRowsWithProjectState(items, request, asOf, taskStatus == "done", projectDone, skipped)
	if err != nil {
		return CursorPage[dao.TodoItem]{}, err
	}
	filtered := items[:0]
	for _, item := range items {
		if afterTodoItemAnchor(item, request.Anchor) {
			filtered = append(filtered, item)
		}
	}
	selected, more := pagination.Window(filtered, request.Size)
	page := CursorPage[dao.TodoItem]{Items: selected}
	if more && len(selected) > 0 {
		last := selected[len(selected)-1]
		page.Next = &CursorAnchor{Position: last.Position, Date: last.OccurrenceDate, OccurrenceDate: last.OccurrenceDate, SeriesID: last.SeriesID, ID: last.ID, AsOf: asOf.Format(time.RFC3339Nano)}
	}
	return page, nil

}
