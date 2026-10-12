package application

import (
	"context"
	"sort"
	"time"

	inboxdao "github.com/Najah7/task2todaytodo/internal/application/inbox/dao"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type inboxCandidateReader struct {
	auth      AuthStore
	tasks     TaskStore
	schedules scheduleusecase.ScheduleUseCases
	inbox     InboxStore
}

func NewInboxUseCases(auth AuthStore, tasks TaskStore, schedules scheduleusecase.ScheduleUseCases, inbox InboxStore, logger telemetry.Logger) inboxusecase.InboxUseCases {
	reader := inboxCandidateReader{auth: auth, tasks: tasks, schedules: schedules, inbox: inbox}
	return inboxusecase.InboxUseCases{List: inboxusecase.NewListUseCase(reader, logger, time.Now)}
}

func (r inboxCandidateReader) Timezone(ctx context.Context, actorID string) (string, error) {
	return r.auth.Users.GetTimezone(ctx, actorID)
}

func (r inboxCandidateReader) ReadCandidates(ctx context.Context, actorID string, snapshot inboxusecase.Snapshot, dateRange inboxusecase.CandidateRange) ([]inboxdao.Item, error) {
	ownerID := taskdomain.UserID(actorID)
	taskRows, err := r.tasks.Tasks.ReadTaskListCandidates(ctx, ownerID, taskusecase.TaskListRequest{
		AsOf: snapshot.AsOf, DueFilter: "all", Status: "all",
	})
	if err != nil {
		return nil, err
	}
	taskRows, err = taskusecase.EnrichTaskList(ctx, r.tasks.ActionItems, ownerID, taskRows, snapshot.AsOf)
	if err != nil {
		return nil, err
	}
	items := make([]inboxdao.Item, 0, len(taskRows))
	projectIDs := make(map[string]struct{})
	for _, task := range taskRows {
		if task.DeletedAt != nil {
			continue
		}
		createdAt := task.CursorCreatedAt
		if createdAt == "" {
			createdAt = time.Unix(task.CreatedAt, 0).UTC().Format(time.RFC3339Nano)
		}
		projectID := task.ProjectID
		if projectID != "" {
			projectIDs[projectID] = struct{}{}
		}
		items = append(items, inboxdao.Item{
			Type: inboxdao.ItemTypeTask, ID: task.ID, Title: task.Title, ProjectID: projectID,
			CreatedAt: createdAt,
			Task: &inboxdao.TaskPayload{
				TaskID: task.ID, Revision: task.Revision, Description: task.Description, AssigneeID: task.AssigneeID, DueDate: task.DueDate,
				RemainingDays: task.RemainingDays, EstimatedMinutes: task.EstimatedMinutes, EstimateSource: task.EstimateSource,
				ActualMinutes: task.ActualMinutes, Progress: task.Progress, Priority: task.Priority.Value, Status: task.Status.Value,
				ActionItemCount: task.ActionItemCount, ActionItemCompletedCount: task.ActionItemCompletedCount,
				CanUpdate: task.CanUpdate, CanAssign: task.CanAssign,
				Started: task.Started, EstimateEditable: task.EstimateEditable,
				CanAddActionItem: task.CanAddActionItem, CanAddToToday: task.CanAddToToday,
			},
		})
	}

	var scheduleRows []dao.ScheduleOccurrence
	var anchor *scheduleusecase.CursorAnchor
	for {
		page, err := r.schedules.ListOccurrences.ExecutePage(ctx, actorID, scheduleusecase.CursorPageRequest{
			Size: 100, From: dateRange.ScheduleFrom, To: dateRange.ScheduleTo, AsOf: snapshot.AsOf, Anchor: anchor,
		})
		if err != nil {
			return nil, err
		}
		scheduleRows = append(scheduleRows, page.Items...)
		if page.Next == nil {
			break
		}
		anchor = page.Next
	}
	ruleIDs := make(map[string]struct{})
	for _, schedule := range scheduleRows {
		ruleIDs[schedule.RuleID] = struct{}{}
		if schedule.ProjectID != "" {
			projectIDs[schedule.ProjectID] = struct{}{}
		}
	}
	ruleMetadata, err := r.inbox.Metadata.ListScheduleRuleMetadata(ctx, sortedKeys(ruleIDs))
	if err != nil {
		return nil, err
	}
	createdAtByRule := make(map[string]string, len(ruleMetadata))
	for _, metadata := range ruleMetadata {
		createdAtByRule[metadata.RuleID] = metadata.CreatedAt
	}
	projectMetadata, err := r.inbox.Metadata.ListProjectMetadata(ctx, actorID, sortedKeys(projectIDs))
	if err != nil {
		return nil, err
	}
	projectNames := make(map[string]string, len(projectMetadata))
	for _, metadata := range projectMetadata {
		projectNames[metadata.ProjectID] = metadata.Name
	}
	for i := range items {
		items[i].ProjectName = projectNames[items[i].ProjectID]
	}
	for _, schedule := range scheduleRows {
		if schedule.AssigneeID != actorID || schedule.Deleted {
			continue
		}
		createdAt := createdAtByRule[schedule.RuleID]
		if createdAt == "" {
			continue
		}
		id := schedule.RuleID + "/" + schedule.OccurrenceDate
		projectID := schedule.ProjectID
		var occurrenceID *string
		if schedule.ID != nil {
			value := *schedule.ID
			occurrenceID = &value
		}
		items = append(items, inboxdao.Item{
			Type: inboxdao.ItemTypeSchedule, ID: id, Title: schedule.Title, ProjectID: projectID,
			ProjectName: projectNames[projectID], CreatedAt: createdAt,
			Schedule: &inboxdao.SchedulePayload{
				OccurrenceID: occurrenceID, RuleID: schedule.RuleID, OccurrenceDate: schedule.OccurrenceDate,
				SourceRevisionID: schedule.SourceRevisionID, AssigneeID: schedule.AssigneeID, Description: schedule.Description,
				Location: schedule.Location, Timezone: schedule.Timezone, AllDay: schedule.AllDay, Completed: schedule.Completed,
				Skipped: schedule.Skipped, StartAt: schedule.StartAt, EndAt: schedule.EndAt, Revision: schedule.Revision,
				CanUpdate: schedule.CanUpdate, CanAssign: schedule.CanAssign, CanUpdateFollowing: schedule.CanUpdateFollowing,
			},
		})
	}
	return items, nil
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
