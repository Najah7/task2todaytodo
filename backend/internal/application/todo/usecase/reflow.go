package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ReflowUseCase struct {
	uow    UOW
	newID  func() string
	logger telemetry.Logger
}

func NewReflowUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *ReflowUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &ReflowUseCase{uow: uow, newID: newID, logger: telemetry.OrNop(logger)}
}

func (u *ReflowUseCase) Execute(ctx context.Context, userID string, expectedRevision int32, orderedItemIDs []string) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "ReflowUseCase.Execute", err) }()
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, saved, _, in, err := loadPlanningState(ctx, userID, day.Date, repos)
		if err != nil {
			return err
		}
		if todo.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		order, err := requestedOrder(orderedItemIDs, saved)
		if err != nil {
			return err
		}
		plan, err := domain.Reflow(domain.ReflowInput{Input: in, Order: order})
		if err != nil {
			return err
		}
		if err := persistPlan(ctx, todo.ID, saved, plan, u.newID, repos); err != nil {
			return err
		}
		if _, err := repos.Todos().BumpRevision(ctx, todo.ID, todo.Revision); err != nil {
			return err
		}
		todo.Revision++
		result = todo
		return nil
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.reflow", userID, result.ID, "")
	}
	return result, err
}

func loadPlanningState(ctx context.Context, userID string, todoDate time.Time, repos Repositories) (dao.Todo, []dao.Item, dao.PlanningSources, domain.Input, error) {
	todo, err := repos.Todos().GetByUserDate(ctx, userID, todoDate)
	if err != nil {
		return dao.Todo{}, nil, dao.PlanningSources{}, domain.Input{}, err
	}
	saved, err := repos.Items().ListByTodoID(ctx, todo.ID)
	if err != nil {
		return dao.Todo{}, nil, dao.PlanningSources{}, domain.Input{}, err
	}
	sources, err := repos.Sources().ReadPlanningSources(ctx, userID, todoDate, todo.Timezone)
	if err != nil {
		return dao.Todo{}, nil, dao.PlanningSources{}, domain.Input{}, err
	}
	// Reflow operates on the persisted plan's calendar. A timezone update is
	// applied only by full regeneration, so a manual change must retain the
	// existing day and the Schedule times already saved in that plan.
	sources.Timezone = todo.Timezone
	refs := workSourceRefs(saved, sources.History)
	actionSources, err := repos.Sources().ReadWorkSources(ctx, userID, todo.Date, todo.Timezone, refs)
	if err != nil {
		return dao.Todo{}, nil, dao.PlanningSources{}, domain.Input{}, err
	}
	sourceByID := make(map[string]dao.WorkSource, len(actionSources))
	for _, source := range actionSources {
		sourceByID[sourceKey(source.Kind, source.ID)] = source
	}
	scheduleByKey := make(map[string]dao.ScheduleOccurrence, len(sources.Schedules))
	for _, occurrence := range sources.Schedules {
		scheduleByKey[scheduleKey(occurrence.RuleID, occurrence.OccurrenceDate)] = occurrence
	}
	for index := range saved {
		item := &saved[index]
		if item.Kind != string(domain.FrameSchedule) {
			continue
		}
		if occurrence, ok := scheduleByKey[scheduleKey(item.ScheduleRuleID, item.OccurrenceDate)]; ok {
			item.Completed = occurrence.Completed
			if item.ScheduledStart != nil && item.ScheduledEnd != nil {
				occurrence.StartAt = *item.ScheduledStart
				occurrence.EndAt = *item.ScheduledEnd
			}
			occurrence.Hidden = item.Hidden
			scheduleByKey[scheduleKey(item.ScheduleRuleID, item.OccurrenceDate)] = occurrence
		}
	}
	for index := range sources.Schedules {
		if occurrence, ok := scheduleByKey[scheduleKey(sources.Schedules[index].RuleID, sources.Schedules[index].OccurrenceDate)]; ok {
			sources.Schedules[index] = occurrence
		}
	}
	candidateIDs := make(map[string]bool, len(sources.Candidates))
	for _, candidate := range sources.Candidates {
		candidateIDs[sourceKey(candidate.Kind, candidate.ID)] = true
	}
	for _, item := range saved {
		if !domain.FrameKind(item.Kind).IsWork() || candidateIDs[sourceKey(item.Kind, itemSourceID(item))] {
			continue
		}
		key := sourceKey(item.Kind, itemSourceID(item))
		source, ok := sourceByID[key]
		if !ok {
			continue
		}
		sources.Candidates = append(sources.Candidates, source)
		candidateIDs[key] = true
	}
	in, err := plannerInput(todoDate, sources, saved)
	if err != nil {
		return dao.Todo{}, nil, dao.PlanningSources{}, domain.Input{}, err
	}
	for index := range in.Schedules {
		key := in.Schedules[index].ID
		for savedIndex := range saved {
			item := &saved[savedIndex]
			if item.Kind != string(domain.FrameSchedule) || scheduleKey(item.ScheduleRuleID, item.OccurrenceDate) != key {
				continue
			}
			in.Schedules[index].Hidden = item.Hidden
			in.Schedules[index].Completed = item.Completed
			break
		}
	}
	return todo, saved, sources, in, nil
}

func ensureTodayTodoInTx(ctx context.Context, userID string, day dao.CurrentDay, repos Repositories, regenerator *RegenerateTodayUseCase, newID func() string) (dao.Todo, error) {
	todo, err := repos.Todos().GetByUserDate(ctx, userID, day.Date)
	if err == nil {
		return todo, nil
	}
	if err != ErrNotFound {
		return dao.Todo{}, err
	}
	model, err := domain.NewTodo(newID(), userID, day.Date, day.Timezone)
	if err != nil {
		return dao.Todo{}, err
	}
	todo, err = repos.Todos().Create(ctx, model)
	if err != nil {
		return dao.Todo{}, err
	}
	return regenerator.generateExistingTodayInTx(ctx, userID, day.Date, day.Timezone, todo, repos, false, false)
}

func requestedOrder(ids []string, saved []dao.Item) ([]domain.OrderedItem, error) {
	byID := make(map[string]dao.Item, len(saved))
	for _, item := range saved {
		byID[item.ID] = item
	}
	if len(ids) == 0 {
		return nil, domain.ErrInvalidReflow
	}
	seen := make(map[string]bool, len(ids))
	order := make([]domain.OrderedItem, 0, len(ids))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok || seen[id] {
			return nil, domain.ErrInvalidReflow
		}
		seen[id] = true
		if domain.FrameKind(item.Kind).IsWork() {
			if item.PlannedMinutes == nil {
				return nil, domain.ErrInvalidReflow
			}
			order = append(order, domain.OrderedItem{Kind: domain.FrameKind(item.Kind), FrameID: item.ID, SourceID: itemSourceID(item), PlannedMinutes: *item.PlannedMinutes})
		} else {
			order = append(order, domain.OrderedItem{Kind: domain.FrameSchedule, ScheduleID: scheduleKey(item.ScheduleRuleID, item.OccurrenceDate)})
		}
	}
	for _, item := range saved {
		if !item.Hidden && !seen[item.ID] {
			return nil, domain.ErrInvalidReflow
		}
	}
	var fixedSaved, fixedRequested []string
	for _, item := range saved {
		if !item.Hidden && (item.Kind == string(domain.FrameSchedule) || item.Completed) {
			fixedSaved = append(fixedSaved, item.ID)
		}
	}
	for _, id := range ids {
		item := byID[id]
		if item.Kind == string(domain.FrameSchedule) || item.Completed {
			fixedRequested = append(fixedRequested, id)
		}
	}
	if !equalStrings(fixedSaved, fixedRequested) {
		return nil, domain.ErrInvalidReflow
	}
	return order, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func persistPlan(ctx context.Context, todoID string, saved []dao.Item, plan domain.Plan, newID func() string, repos Repositories) error {
	items, err := planTodoItems(todoID, saved, plan, newID)
	if err != nil {
		return err
	}
	return repos.Items().ReplacePlanPreservingCompletedWork(ctx, todoID, items)
}

func planTodoItems(todoID string, saved []dao.Item, plan domain.Plan, newID func() string) ([]domain.TodoItem, error) {
	existingSchedules := make(map[string]string)
	hiddenRows := make(map[string]domain.TodoItem)
	for _, savedItem := range saved {
		if savedItem.Kind != string(domain.FrameSchedule) {
			continue
		}
		key := scheduleKey(savedItem.ScheduleRuleID, savedItem.OccurrenceDate)
		existingSchedules[key] = savedItem.ID
		if savedItem.Hidden {
			model, err := restoredItem(savedItem)
			if err != nil {
				return nil, err
			}
			hiddenRows[key] = model
		}
	}
	items := make([]domain.TodoItem, 0, len(plan.Frames)+len(plan.PreservedCompletedFrames)+len(hiddenRows))
	seen := make(map[string]bool)
	plannedSchedules := make(map[string]bool)
	for _, frame := range plan.Frames {
		if frame.Kind == domain.FrameSchedule {
			key := scheduleKey(frame.RuleID, frame.OccurrenceDate)
			frame.ID = existingSchedules[key]
			plannedSchedules[key] = true
		}
		if frame.ID != "" && frame.Completed && seen[frame.ID] {
			continue
		}
		item, err := itemFromFrame(todoID, frame, newID())
		if err != nil {
			return nil, err
		}
		if item.Completed && item.Kind.IsWork() {
			seen[item.ID] = true
		}
		items = append(items, item)
	}
	for _, frame := range plan.PreservedCompletedFrames {
		if !frame.Kind.IsWork() || seen[frame.ID] {
			continue
		}
		frame.Position = len(items)
		item, err := itemFromFrame(todoID, frame, newID())
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		seen[frame.ID] = true
	}
	for _, key := range sortedKeys(hiddenRows) {
		if plannedSchedules[key] {
			continue
		}
		item := hiddenRows[key]
		item.Position = len(items)
		items = append(items, item)
	}
	for index := range items {
		items[index].Position = index
	}
	return items, nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func restoredItem(item dao.Item) (domain.TodoItem, error) {
	model := domain.TodoItem{ID: item.ID, TodoID: item.TodoID, Kind: domain.FrameKind(item.Kind), ActionItemID: item.ActionItemID, TaskID: item.TaskID,
		ScheduleRuleID: item.ScheduleRuleID, OccurrenceDate: item.OccurrenceDate, PlannedMinutes: item.PlannedMinutes,
		Start: item.ScheduledStart, End: item.ScheduledEnd, Completed: item.Completed, Hidden: item.Hidden, Position: int(item.Position)}
	return domain.RestoreTodoItem(model)
}

func itemRef(item dao.Item) dao.WorkSourceRef {
	return dao.WorkSourceRef{Kind: item.Kind, ID: itemSourceID(item)}
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func sourceByRef(sources []dao.WorkSource, ref dao.WorkSourceRef) (dao.WorkSource, bool) {
	for _, source := range sources {
		if source.ID == ref.ID && source.Kind == ref.Kind {
			return source, true
		}
	}
	return dao.WorkSource{}, false
}

func itemSourceID(item dao.Item) string {
	if item.Kind == string(domain.FrameTask) {
		return item.TaskID
	}
	if item.Kind == string(domain.FrameActionItem) {
		return item.ActionItemID
	}
	return ""
}

func sourceKey(kind, id string) string { return kind + "\x00" + id }

func workSourceRefs(saved []dao.Item, history []dao.HistoryFrame) []dao.WorkSourceRef {
	refs := make([]dao.WorkSourceRef, 0, len(saved)+len(history))
	seen := make(map[string]bool)
	add := func(kind, id string) {
		if id == "" || !domain.FrameKind(kind).IsWork() {
			return
		}
		key := sourceKey(kind, id)
		if !seen[key] {
			seen[key] = true
			refs = append(refs, dao.WorkSourceRef{Kind: kind, ID: id})
		}
	}
	for _, item := range saved {
		add(item.Kind, itemSourceID(item))
	}
	for _, frame := range history {
		add(frame.Kind, itemHistorySourceID(frame))
	}
	return refs
}

func itemHistorySourceID(frame dao.HistoryFrame) string {
	if frame.Kind == string(domain.FrameTask) {
		return frame.TaskID
	}
	return frame.ActionItemID
}

func plannerInputForDay(day dao.CurrentDay, sources dao.PlanningSources, saved []dao.Item) (domain.Input, error) {
	in, err := plannerInput(day.Date, sources, saved)
	if err != nil {
		return domain.Input{}, fmt.Errorf("build planner input: %w", err)
	}
	return in, nil
}
