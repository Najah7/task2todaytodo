package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
)

// SyncScheduleSourcesInTx keeps persisted Schedule rows in an existing Todo in
// sync with the source schedule set. The caller owns the surrounding UOW and
// must lock the Todo user before invoking it.
func SyncScheduleSourcesInTx(ctx context.Context, repos Repositories, todo dao.Todo, schedules []dao.ScheduleOccurrence, newID func() string) error {
	items, err := repos.Items().ListByTodoID(ctx, todo.ID)
	if err != nil {
		return err
	}
	byKey := make(map[string]dao.ScheduleOccurrence, len(schedules))
	for _, source := range schedules {
		byKey[scheduleSourceKey(source.RuleID, source.OccurrenceDate)] = source
	}
	itemByKey := make(map[string]dao.Item)
	startChanged := make(map[string]bool)
	modified := false
	for _, item := range items {
		if item.Kind != string(domain.FrameSchedule) {
			continue
		}
		key := scheduleSourceKey(item.ScheduleRuleID, item.OccurrenceDate)
		source, exists := byKey[key]
		if !exists {
			if err := repos.Items().DeleteByID(ctx, todo.ID, item.ID); err != nil {
				return err
			}
			modified = true
			continue
		}
		itemByKey[key] = item
		if item.ScheduledStart == nil || item.ScheduledEnd == nil {
			return domain.ErrInvalidTodoItem
		}
		startChanged[key] = !item.Hidden && !item.ScheduledStart.Equal(source.StartAt)
		if !item.ScheduledStart.Equal(source.StartAt) || !item.ScheduledEnd.Equal(source.EndAt) {
			model, err := todoItemModel(item)
			if err != nil {
				return err
			}
			model.Start, model.End = &source.StartAt, &source.EndAt
			if _, err := repos.Items().Save(ctx, model); err != nil {
				return err
			}
			modified = true
		}
	}
	nextPosition := len(items)
	for key, source := range byKey {
		if _, exists := itemByKey[key]; exists {
			continue
		}
		id := newID()
		model, err := domain.NewScheduleTodoItem(id, todo.ID, source.RuleID, source.OccurrenceDate, source.StartAt, source.EndAt, source.Completed, false, nextPosition)
		if err != nil {
			return err
		}
		if _, err := repos.Items().Insert(ctx, model); err != nil {
			return err
		}
		nextPosition++
		itemByKey[key] = dao.Item{ID: id, TodoID: todo.ID, Kind: string(domain.FrameSchedule), ScheduleRuleID: source.RuleID,
			OccurrenceDate: source.OccurrenceDate, ScheduledStart: &source.StartAt, ScheduledEnd: &source.EndAt}
		startChanged[key] = true
		modified = true
	}
	if !modified {
		return nil
	}
	items, err = repos.Items().ListByTodoID(ctx, todo.ID)
	if err != nil {
		return err
	}
	models := make([]domain.TodoItem, 0, len(items))
	for _, item := range items {
		model, err := todoItemModel(item)
		if err != nil {
			return err
		}
		models = append(models, model)
	}
	sort.SliceStable(models, func(i, j int) bool { return models[i].Position < models[j].Position })
	insertions := make([]dao.ScheduleOccurrence, 0, len(startChanged))
	for key := range startChanged {
		if startChanged[key] {
			insertions = append(insertions, byKey[key])
		}
	}
	sort.Slice(insertions, func(i, j int) bool {
		if !insertions[i].StartAt.Equal(insertions[j].StartAt) {
			return insertions[i].StartAt.Before(insertions[j].StartAt)
		}
		if insertions[i].RuleID != insertions[j].RuleID {
			return insertions[i].RuleID < insertions[j].RuleID
		}
		return insertions[i].OccurrenceDate.Before(insertions[j].OccurrenceDate)
	})
	domainInsertions := make([]domain.ScheduleInsertion, 0, len(insertions))
	for _, source := range insertions {
		key := scheduleSourceKey(source.RuleID, source.OccurrenceDate)
		item := itemByKey[key]
		domainInsertions = append(domainInsertions, domain.ScheduleInsertion{ItemID: item.ID, Start: source.StartAt})
	}
	if len(domainInsertions) != 0 {
		models, err = domain.InsertSchedulesByStart(models, domainInsertions)
		if err != nil {
			return err
		}
	}
	for index := range models {
		models[index].Position = index
	}
	if err := repos.Items().SavePositions(ctx, todo.ID, models); err != nil {
		return err
	}
	_, err = repos.Todos().BumpRevision(ctx, todo.ID, todo.Revision)
	return err
}

func todoItemModel(item dao.Item) (domain.TodoItem, error) {
	return domain.RestoreTodoItem(domain.TodoItem{ID: item.ID, TodoID: item.TodoID, Kind: domain.FrameKind(item.Kind),
		ActionItemID: item.ActionItemID, TaskID: item.TaskID, ScheduleRuleID: item.ScheduleRuleID, OccurrenceDate: item.OccurrenceDate,
		PlannedMinutes: item.PlannedMinutes, Start: item.ScheduledStart, End: item.ScheduledEnd, Completed: item.Completed, Hidden: item.Hidden, Position: int(item.Position)})
}

func scheduleSourceKey(ruleID string, occurrenceDate time.Time) string {
	return ruleID + "/" + occurrenceDate.Format("2006-01-02")
}
