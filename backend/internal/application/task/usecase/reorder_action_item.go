package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

var ErrActionItemPositionOutOfRange = errors.New("action item position is out of range")

type ReorderActionItemUseCase struct {
	uow    UOW
	ID     shared.ID
	logger logging.Logger
}

func NewReorderActionItemUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *ReorderActionItemUseCase {
	uc := &ReorderActionItemUseCase{uow: uow, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

func (uc *ReorderActionItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate string, position int) (output dao.ActionItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReorderActionItemUseCase.ExecuteOccurrence", err) }()

	if position < 0 {
		return dao.ActionItem{}, ErrActionItemPositionOutOfRange
	}
	asOf := time.Now()
	var result dao.ActionItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		mutate := func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.ActionItemUpdate())
			if err != nil {
				return err
			}
			state.projectDone = projectBefore.State.Status == "done"
			if projectDoneSuppressesActionItemOccurrence(state) {
				return ErrOccurrenceInactive
			}
			rows, err := listActionItemOccurrenceCommandProjection(ctx, repos.ActionItems(), userID, taskID, shared.ActionItemUpdate())
			if err != nil {
				return err
			}
			rootIDs := make(map[string]bool)
			for _, row := range rows {
				if row.ID == row.SeriesID {
					rootIDs[row.ID] = true
				}
			}
			skipped := make(map[string]map[string]bool)
			for rootID := range rootIDs {
				if store, ok := repos.ActionItems().(actionItemSkippedOccurrenceCommandStore); ok {
					dates, err := store.ListActionItemSkippedOccurrencesForCapability(ctx, userID, taskID, domain.ActionItemID(rootID), shared.ActionItemUpdate())
					if err != nil {
						return err
					}
					skipped[rootID] = make(map[string]bool)
					for _, value := range dates {
						skipped[rootID][time.Unix(value, 0).UTC().Format("2006-01-02")] = true
					}
				} else if store, ok := repos.ActionItems().(actionItemSkippedOccurrenceStore); ok {
					dates, err := store.ListActionItemSkippedOccurrences(ctx, userID, taskID, domain.ActionItemID(rootID))
					if err != nil {
						return err
					}
					skipped[rootID] = make(map[string]bool)
					for _, value := range dates {
						skipped[rootID][time.Unix(value, 0).UTC().Format("2006-01-02")] = true
					}
				}
			}
			request := CursorPageRequest{Size: len(rootIDs) + 1, FromDate: state.date.Format("2006-01-02"), AsOf: asOf}
			visible, err := expandActionItemRowsWithProjectState(rows, request, request.AsOf, false, state.projectDone, skipped)
			if err != nil {
				return err
			}
			day := make([]dao.ActionItem, 0)
			for _, item := range visible {
				if item.OccurrenceDate == state.date.Format("2006-01-02") {
					day = append(day, item)
				}
			}
			if position >= len(day) {
				return ErrActionItemPositionOutOfRange
			}
			target := -1
			for i, item := range day {
				if item.SeriesID == string(seriesID) && item.OccurrenceDate == state.date.Format("2006-01-02") {
					target = i
					break
				}
			}
			if target < 0 {
				return ErrOccurrenceNotFound
			}
			selected := day[target]
			if target < position {
				copy(day[target:position], day[target+1:position+1])
			} else if target > position {
				copy(day[position+1:target+1], day[position:target])
			}
			day[position] = selected
			writer, err := requireActionItemOverride(repos.ActionItems())
			if err != nil {
				return err
			}
			for index, row := range day {
				if row.Position == index {
					if row.SeriesID == string(seriesID) {
						result = row
					}
					continue
				}
				currentState, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, domain.ActionItemID(row.SeriesID), row.OccurrenceDate, asOf, shared.ActionItemUpdate())
				if err != nil {
					return err
				}
				var id domain.ActionItemID
				completed := false
				if currentState.current != nil {
					id = domain.ActionItemID(currentState.current.ID)
					completed = currentState.current.Completed
				} else {
					id, err = generatedActionItemID(uc.ID)
					if err != nil {
						return err
					}
				}
				item, err := actionItemDomainOccurrence(currentState.root, currentState.current, currentState.date, id, completed, asOf)
				if err != nil {
					return err
				}
				item.Position = index
				validated, err := domain.NewActionItemWithRecurrence(item)
				if err != nil {
					return err
				}
				savedID, err := writer.UpsertActionItemOverride(ctx, userID, validated)
				if err != nil {
					return err
				}
				if row.SeriesID == string(seriesID) {
					result = actionItemDAOFromOccurrence(currentState.root, validated, savedID)
				}
			}
			return nil
		}
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), mutate)
	})
	return result, err

}

func (uc *ReorderActionItemUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	actionItemID domain.ActionItemID,
	position int,
) (dao.ActionItem, error) {
	var current dao.ActionItem
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		current, err = getActionItemForCommand(ctx, repos.ActionItems(), userID, taskID, actionItemID, shared.ActionItemUpdate())
		return err
	}); err != nil {
		logUnexpectedTaskFailure(uc.logger, ctx, "action_item.reorder.load", err)
		return dao.ActionItem{}, err
	}
	if current.ID != string(actionItemID) || current.Deleted {
		return dao.ActionItem{}, ErrActionItemNotFound
	}
	return uc.ExecuteOccurrence(ctx, userID, taskID, domain.ActionItemID(current.SeriesID), current.OccurrenceDate, position)
}
