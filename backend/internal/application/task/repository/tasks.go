package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.TaskRepository = TaskRepository{}
var _ usecase.TaskProjectRepository = TaskRepository{}

type TaskRepository struct {
	queries *sqlc.Queries
}

func NewTaskRepository(db sqlc.DBTX) *TaskRepository {
	return &TaskRepository{
		queries: sqlc.New(db),
	}
}

func (r *TaskRepository) WithTx(tx pgx.Tx) *TaskRepository {
	return &TaskRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r TaskRepository) Get(ctx context.Context, id domain.TaskID) (dao.Task, error) {
	record, err := r.queries.GetTask(ctx, string(id))
	if err != nil {
		return dao.Task{}, err
	}
	return recordToTask(record), nil
}

func (r TaskRepository) GetByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error) {
	record, err := r.queries.GetTaskByUserID(ctx, sqlc.GetTaskByUserIDParams{
		ID:       string(id),
		ActorKey: string(userID),
	})
	if err != nil {
		return dao.Task{}, taskRepositoryError(err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) GetProjectByUserID(ctx context.Context, actorID, projectID string) (usecase.TaskProject, error) {
	project, err := r.queries.GetProjectByUserID(ctx, sqlc.GetProjectByUserIDParams{ID: projectID, ActorKey: actorID})
	if err != nil {
		return usecase.TaskProject{}, taskProjectRepositoryError(err)
	}
	return taskProjectRecord(project), nil
}

func (r TaskRepository) GetProjectByUserIDWithPermission(ctx context.Context, actorID, projectID string, capability shared.Capability) (usecase.TaskProject, error) {
	project, err := r.queries.GetProjectByUserIDForPermission(ctx, sqlc.GetProjectByUserIDForPermissionParams{ID: projectID, ActorID: actorID, ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, lookupErr := r.GetProjectByUserID(ctx, actorID, projectID); lookupErr == nil {
				return usecase.TaskProject{}, usecase.ErrPermissionDenied
			}
		}
		return usecase.TaskProject{}, taskProjectRepositoryError(err)
	}
	return taskProjectRecord(project), nil
}

func (r TaskRepository) LockProjectByUserIDWithPermission(ctx context.Context, actorID, projectID string, capability shared.Capability) (usecase.TaskProject, error) {
	project, err := r.queries.LockProjectByUserIDForPermission(ctx, sqlc.LockProjectByUserIDForPermissionParams{ID: projectID, UserID: actorID, ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, lookupErr := r.GetProjectByUserID(ctx, actorID, projectID); lookupErr == nil {
				return usecase.TaskProject{}, usecase.ErrPermissionDenied
			}
		}
		return usecase.TaskProject{}, taskProjectRepositoryError(err)
	}
	return taskProjectRecord(project), nil
}

func taskProjectRecord(project sqlc.Project) usecase.TaskProject {
	return usecase.TaskProject{ID: project.ID, OwnerID: project.UserID, DefaultPriority: project.Priority}
}

func (r TaskRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, capability shared.Capability) (dao.Task, error) {
	record, err := r.queries.GetTaskByUserIDForPermission(ctx, sqlc.GetTaskByUserIDForPermissionParams{
		ID: string(id), ActorID: string(userID), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, readErr := r.GetByUserID(ctx, userID, id); readErr == nil {
				return dao.Task{}, usecase.ErrPermissionDenied
			}
		}
		return dao.Task{}, taskRepositoryError(err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) LockByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error) {
	record, err := r.queries.LockTaskByUserID(ctx, sqlc.LockTaskByUserIDParams{ID: string(id), UserID: string(userID)})
	if err != nil {
		return dao.Task{}, taskRepositoryError(err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, capability shared.Capability) (dao.Task, error) {
	record, err := r.queries.LockTaskByUserIDForPermission(ctx, sqlc.LockTaskByUserIDForPermissionParams{
		ID: string(id), UserID: string(userID), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, readErr := r.GetByUserID(ctx, userID, id); readErr == nil {
				return dao.Task{}, usecase.ErrPermissionDenied
			}
		}
		return dao.Task{}, taskRepositoryError(err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) HasPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, capability shared.Capability) (bool, error) {
	allowed, err := r.queries.HasTaskPermission(ctx, sqlc.HasTaskPermissionParams{
		TaskID: string(id), ActorID: string(userID), ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		return false, taskRepositoryError(err)
	}
	return allowed, nil
}

func (r TaskRepository) SetStatusByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID, status domain.TaskStatus) error {
	current, err := r.GetByUserID(ctx, userID, id)
	if err != nil {
		return err
	}
	return r.SetStatusByUserIDWithPermission(ctx, userID, id, status, current.Revision, shared.TaskUpdate())
}

func (r TaskRepository) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, status domain.TaskStatus, expectedRevision int32, capability shared.Capability) error {
	_, err := r.queries.UpdateTaskStatusByUserID(ctx, sqlc.UpdateTaskStatusByUserIDParams{
		ID: string(id), UserID: string(userID), Status: taskStatusString(status), ExpectedRevision: expectedRevision,
		ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
	if err != nil {
		return r.taskWriteError(ctx, userID, id, capability, err)
	}
	return nil
}

func (r TaskRepository) ReadTaskProgressSources(ctx context.Context, taskIDs []string, asOf time.Time) (dao.TaskProgressSources, error) {
	sources := dao.TaskProgressSources{
		Counts:   make(map[string]dao.TaskProgressCounts, len(taskIDs)),
		Statuses: make(map[string]dao.TaskStatus, len(taskIDs)),
	}
	if len(taskIDs) == 0 {
		return sources, nil
	}
	rows, err := r.queries.ReadTaskProgressSources(ctx, sqlc.ReadTaskProgressSourcesParams{
		AsOf: pgtype.Timestamptz{Time: asOf, Valid: true}, TaskIds: taskIDs, ProjectIds: nil,
	})
	if err != nil {
		return dao.TaskProgressSources{}, err
	}
	for _, row := range rows {
		sources.Statuses[row.TaskID] = dao.TaskStatus{Value: row.Status}
		sources.Counts[row.TaskID] = dao.TaskProgressCounts{
			Total:     int(row.ActionItemTotal),
			Completed: int(row.ActionItemCompleted),
		}
		var actionItemRoots []progressRootRecord
		if err := json.Unmarshal(row.ActionItemRoots, &actionItemRoots); err != nil {
			return dao.TaskProgressSources{}, err
		}
		for _, root := range actionItemRoots {
			value, err := root.toDAO(row.TaskID)
			if err != nil {
				return dao.TaskProgressSources{}, err
			}
			sources.ActionItemRoots = append(sources.ActionItemRoots, value)
		}
	}
	return sources, nil
}

type progressRootRecord struct {
	SeriesID             string   `json:"series_id"`
	OccurrenceDate       string   `json:"occurrence_date"`
	Timezone             string   `json:"timezone"`
	IntervalWeeks        int      `json:"interval_weeks"`
	FrequencyAnchorDate  *string  `json:"frequency_anchor_date"`
	Frequencies          []string `json:"frequencies"`
	OccurrenceSavedToday bool     `json:"occurrence_saved_today"`
}

func (root progressRootRecord) toDAO(taskID string) (dao.ProgressRecurrence, error) {
	value := dao.ProgressRecurrence{
		TaskID: taskID, SeriesID: root.SeriesID, OccurrenceDate: root.OccurrenceDate,
		Timezone: root.Timezone, RepeatState: "active", IntervalWeeks: root.IntervalWeeks,
		Frequencies: taskFrequenciesDAO(root.Frequencies), OccurrenceSavedToday: root.OccurrenceSavedToday,
	}
	if root.FrequencyAnchorDate != nil {
		date, err := time.Parse("2006-01-02", *root.FrequencyAnchorDate)
		if err != nil {
			return dao.ProgressRecurrence{}, err
		}
		value.FrequencyAnchorDate = date.UTC().Unix()
	}
	return value, nil
}

func (r TaskRepository) ListByUserIDCursor(ctx context.Context, userID domain.UserID, limit int, anchor *usecase.CursorAnchor) ([]dao.Task, error) {
	arg := sqlc.ListTasksByUserIDCursorPageParams{UserID: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		at, err := time.Parse(time.RFC3339Nano, anchor.At)
		if err != nil {
			return nil, usecase.ErrInvalidTaskPage
		}
		arg.CursorAt = pgtype.Timestamptz{Time: at, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListTasksByUserIDCursorPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records), nil
}

func (r TaskRepository) ListUsersWithActiveRecurrences(ctx context.Context) ([]domain.UserID, error) {
	ids, err := r.queries.ListUsersWithActiveRecurrences(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]domain.UserID, 0, len(ids))
	for _, id := range ids {
		users = append(users, domain.UserID(id))
	}
	return users, nil
}

func (r TaskRepository) ListByProjectAndUserIDCursor(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, limit int, anchor *usecase.CursorAnchor) ([]dao.Task, error) {
	arg := sqlc.ListProjectTasksByUserIDCursorPageParams{ProjectID: stringToPgText(string(projectID)), UserID: string(userID), PageLimit: int32(limit)}
	if anchor != nil {
		at, err := time.Parse(time.RFC3339Nano, anchor.At)
		if err != nil {
			return nil, usecase.ErrInvalidTaskPage
		}
		arg.CursorAt = pgtype.Timestamptz{Time: at, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	records, err := r.queries.ListProjectTasksByUserIDCursorPage(ctx, arg)
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records), nil
}

func (r TaskRepository) AssignToProjectByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error) {
	record, err := r.queries.AssignTaskToProjectByUserID(ctx, sqlc.AssignTaskToProjectByUserIDParams{
		TaskID:           string(taskID),
		UserID:           string(userID),
		ProjectID:        string(projectID),
		ExpectedRevision: expectedRevision,
	})
	if err != nil {
		return dao.Task{}, r.taskWriteError(ctx, userID, taskID, shared.TaskUpdate(), err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) RemoveFromProjectByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error) {
	record, err := r.queries.RemoveTaskFromProjectByUserID(ctx, sqlc.RemoveTaskFromProjectByUserIDParams{
		TaskID:           string(taskID),
		UserID:           string(userID),
		ProjectID:        stringToPgText(string(projectID)),
		ExpectedRevision: expectedRevision,
	})
	if err != nil {
		return dao.Task{}, r.taskWriteError(ctx, userID, taskID, shared.TaskUpdate(), err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) GetDetailsByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.TaskDetails, error) {
	task, err := r.GetByUserID(ctx, userID, id)
	if err != nil {
		return dao.TaskDetails{}, err
	}

	itemRecords, err := r.queries.ListActionItemsByTaskForUser(ctx, sqlc.ListActionItemsByTaskForUserParams{
		TaskID: string(id),
		UserID: string(userID),
	})
	if err != nil {
		return dao.TaskDetails{}, err
	}
	items := recordsToActionItemsForTaskRows(itemRecords)

	return dao.TaskDetails{
		Task:        task,
		ActionItems: items,
	}, nil
}

func (r TaskRepository) GetByFrequency(ctx context.Context, frequency domain.TaskFrequency) ([]dao.Task, error) {
	records, err := r.queries.GetTaskByFrequency(ctx, frequency.String())
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records), nil
}

func (r TaskRepository) GetByPriority(ctx context.Context, priority domain.TaskPriority) ([]dao.Task, error) {
	records, err := r.queries.GetTaskByPriority(ctx, priority.String())
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records), nil
}

func (r TaskRepository) GetByProject(ctx context.Context, projectID domain.ProjectID) ([]dao.Task, error) {
	records, err := r.queries.GetTaskByProject(ctx, string(projectID))
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records), nil
}

func (r TaskRepository) GetByStatus(ctx context.Context, status domain.TaskStatus) ([]dao.Task, error) {
	records, err := r.queries.GetTaskByStatus(ctx, status.String())
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records), nil
}

func (r TaskRepository) GetByTag(ctx context.Context, tagID string) ([]dao.Task, error) {
	records, err := r.queries.GetTaskByTag(ctx, tagID)
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records), nil
}

func (r TaskRepository) Create(ctx context.Context, task domain.Task) (dao.Task, error) {
	record, err := r.queries.CreateTask(ctx, sqlc.CreateTaskParams{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		ProjectID:        stringToPgText(string(task.ProjectID)),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
		DueDate:          timeToPgDate(task.DueDate),
		Priority:         taskPriorityString(task.Priority),
		Status:           taskStatusString(task.Status),
	})
	if err != nil {
		return dao.Task{}, err
	}
	return recordToTask(record), nil
}

func (r TaskRepository) CreateInProject(ctx context.Context, actorID domain.UserID, task domain.Task) (dao.Task, error) {
	record, err := r.queries.CreateTaskInProject(ctx, sqlc.CreateTaskInProjectParams{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		ProjectID:        string(task.ProjectID),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
		DueDate:          timeToPgDate(task.DueDate),
		Priority:         taskPriorityString(task.Priority),
		Status:           taskStatusString(task.Status),
		ActorID:          string(actorID),
	})
	if err != nil {
		return dao.Task{}, taskProjectRepositoryError(err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) Update(ctx context.Context, task domain.Task) (dao.Task, error) {
	record, err := r.queries.UpdateTask(ctx, sqlc.UpdateTaskParams{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		ProjectID:        stringToPgText(string(task.ProjectID)),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
		DueDate:          timeToPgDate(task.DueDate),
		Priority:         taskPriorityString(task.Priority),
		Status:           taskStatusString(task.Status),
	})
	if err != nil {
		return dao.Task{}, err
	}
	return recordToTask(record), nil
}

func (r TaskRepository) UpdateByUserID(ctx context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error) {
	record, err := r.queries.UpdateTaskByUserID(ctx, sqlc.UpdateTaskByUserIDParams{
		ID:               string(task.ID),
		ExpectedRevision: expectedRevision,
		UserID:           string(userID),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		DueDate:          timeToPgDate(task.DueDate),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
	})
	if err != nil {
		return dao.Task{}, r.taskWriteError(ctx, userID, task.ID, shared.TaskUpdate(), err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) Delete(ctx context.Context, id domain.TaskID) error {
	return r.queries.DeleteTask(ctx, string(id))
}

func (r TaskRepository) DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID, expectedRevision int32) error {
	_, err := r.queries.DeleteTaskByUserID(ctx, sqlc.DeleteTaskByUserIDParams{
		ID:               string(id),
		UserID:           string(userID),
		ExpectedRevision: expectedRevision,
	})
	if err != nil {
		return r.taskWriteError(ctx, userID, id, shared.TaskDelete(), err)
	}
	return nil
}

func (r TaskRepository) taskWriteError(ctx context.Context, userID domain.UserID, taskID domain.TaskID, capability shared.Capability, err error) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	allowed, accessErr := r.HasPermission(ctx, userID, taskID, capability)
	if accessErr != nil {
		return accessErr
	}
	if !allowed {
		return usecase.ErrPermissionDenied
	}
	if _, getErr := r.GetByUserIDWithPermission(ctx, userID, taskID, capability); getErr != nil {
		return taskRepositoryError(getErr)
	}
	return usecase.ErrRevisionConflict
}

func (r TaskRepository) UpdateTaskAssigneeByActor(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID, expectedRevision int32) (dao.Task, error) {
	record, err := r.queries.UpdateTaskAssigneeByActor(ctx, sqlc.UpdateTaskAssigneeByActorParams{
		TaskID: string(taskID), ActorID: string(actorID), AssigneeID: string(assigneeID), ExpectedRevision: expectedRevision,
	})
	if err != nil {
		return dao.Task{}, r.taskWriteError(ctx, actorID, taskID, shared.AssignmentUpdate(), err)
	}
	return recordToTask(record), nil
}

func (r TaskRepository) LockProjectForTaskAssignment(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) error {
	projectID, err := r.queries.TaskProjectForAssigneeChange(ctx, sqlc.TaskProjectForAssigneeChangeParams{
		TaskID: string(taskID), ActorID: string(actorID),
	})
	if err != nil {
		return r.taskWriteError(ctx, actorID, taskID, shared.AssignmentUpdate(), err)
	}
	var lockedProjectID string
	if projectID.Valid {
		lockedProjectID, err = r.queries.LockTaskProjectForAssigneeChange(ctx, sqlc.LockTaskProjectForAssigneeChangeParams{
			TaskID: string(taskID), ActorID: string(actorID),
		})
		if err != nil {
			return r.taskWriteError(ctx, actorID, taskID, shared.AssignmentUpdate(), err)
		}
	}
	_, err = r.queries.LockTaskForAssigneeChange(ctx, sqlc.LockTaskForAssigneeChangeParams{
		TaskID: string(taskID), ActorID: string(actorID),
	})
	if err != nil {
		return r.taskWriteError(ctx, actorID, taskID, shared.AssignmentUpdate(), err)
	}
	currentProjectID, err := r.queries.TaskProjectForAssigneeChange(ctx, sqlc.TaskProjectForAssigneeChangeParams{
		TaskID: string(taskID), ActorID: string(actorID),
	})
	if err != nil {
		return r.taskWriteError(ctx, actorID, taskID, shared.AssignmentUpdate(), err)
	}
	if currentProjectID.Valid != projectID.Valid || (currentProjectID.Valid && currentProjectID.String != lockedProjectID) {
		return usecase.ErrRevisionConflict
	}
	return nil
}

func (r TaskRepository) IsEligibleTaskAssignee(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID) (bool, error) {
	return r.queries.IsEligibleTaskAssignee(ctx, sqlc.IsEligibleTaskAssigneeParams{
		TaskID: string(taskID), ActorID: string(actorID), AssigneeID: string(assigneeID),
	})
}

func (r TaskRepository) ListEligibleTaskAssignees(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) ([]dao.TaskAssignee, error) {
	if _, err := r.GetByUserIDWithPermission(ctx, actorID, taskID, shared.AssignmentUpdate()); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListEligibleTaskAssignees(ctx, sqlc.ListEligibleTaskAssigneesParams{
		TaskID: string(taskID), ActorID: string(actorID),
	})
	if err != nil {
		return nil, taskRepositoryError(err)
	}
	assignees := make([]dao.TaskAssignee, 0, len(rows))
	for _, row := range rows {
		assignees = append(assignees, dao.TaskAssignee{
			ID: row.ID, FirstName: row.FirstName, LastName: row.LastName,
			Email: row.Email, ProjectOwner: row.IsProjectOwner,
		})
	}
	return assignees, nil
}

func taskRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrTaskNotFound
	}
	return err
}

func taskProjectRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrTaskProjectNotFound
	}
	return err
}
