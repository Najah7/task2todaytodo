package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

var ErrTaskPatchRequiredFieldNull = errors.New("required task field cannot be null")

type UpdateTaskInput struct {
	UserID                 domain.UserID
	TaskID                 domain.TaskID
	ExpectedRevision       int32
	Title                  PatchField[string]
	Description            PatchField[string]
	DueDate                PatchField[time.Time]
	ManualEstimatedMinutes PatchField[int]
	ActualMinutes          PatchField[int]
	Priority               PatchField[string]
	ProjectID              PatchField[string]
}

type TaskUpdateFieldError struct {
	Path string
	Err  error
}

func (e TaskUpdateFieldError) Error() string { return e.Err.Error() }
func (e TaskUpdateFieldError) Unwrap() error { return e.Err }

type UpdateTaskUseCase struct {
	uow    UOW
	logger logging.Logger
	now    func() time.Time
}

func NewUpdateTaskUseCase(uow UOW, logger logging.Logger) *UpdateTaskUseCase {
	return &UpdateTaskUseCase{uow: uow, logger: logging.OrNop(logger), now: time.Now}
}

func (uc *UpdateTaskUseCase) Execute(ctx context.Context, in UpdateTaskInput) (result dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateTaskUseCase.Execute", err) }()
	asOf := uc.now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var candidate, current dao.Task
		lockedProjects := map[string]TaskProject{}
		beforeByProject := map[string]shared.ProjectWorkState{}
		if in.ProjectID.Present {
			candidate, err = repos.Tasks().GetByUserIDWithPermission(ctx, in.UserID, in.TaskID, shared.TaskUpdate())
			if err != nil {
				return err
			}
			if candidate.Revision != in.ExpectedRevision {
				return ErrRevisionConflict
			}
			targetProjectID := ""
			if in.ProjectID.Value != nil {
				if strings.TrimSpace(*in.ProjectID.Value) == "" {
					return TaskUpdateFieldError{"project_id", ErrTaskProjectNotFound}
				}
				targetProjectID = *in.ProjectID.Value
			}
			locks := []struct {
				id         string
				capability shared.Capability
			}{}
			if candidate.ProjectID != "" {
				locks = append(locks, struct {
					id         string
					capability shared.Capability
				}{candidate.ProjectID, shared.TaskUpdate()})
			}
			if targetProjectID != "" && targetProjectID != candidate.ProjectID {
				locks = append(locks, struct {
					id         string
					capability shared.Capability
				}{targetProjectID, shared.TaskCreate()})
			}
			sort.Slice(locks, func(i, j int) bool { return locks[i].id < locks[j].id })
			for _, lock := range locks {
				project, lockErr := repos.TaskProjects().LockProjectByUserIDWithPermission(ctx, string(in.UserID), lock.id, lock.capability)
				if lockErr != nil {
					if lock.id == targetProjectID && errors.Is(lockErr, ErrTaskProjectNotFound) {
						return TaskUpdateFieldError{"project_id", lockErr}
					}
					return lockErr
				}
				lockedProjects[project.ID] = project
			}
			current, err = repos.Tasks().LockByUserIDWithPermission(ctx, in.UserID, in.TaskID, shared.TaskUpdate())
			if err != nil {
				return err
			}
			if current.Revision != in.ExpectedRevision || current.ProjectID != candidate.ProjectID {
				return ErrRevisionConflict
			}
		} else {
			current, err = lockTaskForMutation(ctx, repos, in.UserID, in.TaskID, shared.TaskUpdate())
			if err != nil {
				return err
			}
			if current.Revision != in.ExpectedRevision {
				return ErrRevisionConflict
			}
		}
		before := taskProjectMutationSnapshot{ID: current.ProjectID}
		if in.ProjectID.Present && (current.ProjectID != "" || len(lockedProjects) > 0) {
			if repos.ProjectLifecycle() == nil {
				return ErrProjectLifecycleUnavailable
			}
			if current.ProjectID != "" {
				before.State, err = repos.ProjectLifecycle().CaptureWorkState(ctx, current.ProjectID, asOf)
				if err != nil {
					return err
				}
				beforeByProject[current.ProjectID] = before.State
			}
		}
		task, err := taskFromDAO(current)
		if err != nil {
			return err
		}
		if in.Title.Present {
			if in.Title.Value == nil {
				return TaskUpdateFieldError{"title", ErrTaskPatchRequiredFieldNull}
			}
			task.Title = *in.Title.Value
		}
		if in.Description.Present {
			task.Description = ""
			if in.Description.Value != nil {
				task.Description = *in.Description.Value
			}
		}
		if in.DueDate.Present {
			task.DueDate = time.Time{}
			if in.DueDate.Value != nil {
				task.DueDate = *in.DueDate.Value
			}
		}
		if in.ManualEstimatedMinutes.Present {
			if in.ManualEstimatedMinutes.Value != nil && *in.ManualEstimatedMinutes.Value < 0 {
				return TaskUpdateFieldError{"manual_estimated_minutes", domain.ErrTaskEstimatedMinutesInvalid}
			}
			task.ManualEstimatedMinutes = copyOptionalInt(in.ManualEstimatedMinutes.Value)
		}
		if in.ActualMinutes.Present {
			if in.ActualMinutes.Value != nil && *in.ActualMinutes.Value < 0 {
				return TaskUpdateFieldError{"actual_minutes", domain.ErrTaskActualMinutesInvalid}
			}
			task.ActualMinutes = copyOptionalInt(in.ActualMinutes.Value)
		}
		if in.Priority.Present {
			if in.Priority.Value == nil {
				return TaskUpdateFieldError{"priority", domain.ErrTaskPriorityEmpty}
			}
			p, e := domain.NewTaskPriority(*in.Priority.Value)
			if e != nil {
				return TaskUpdateFieldError{"priority", e}
			}
			task.Priority = p
		}
		if in.ProjectID.Present {
			projectChanged := false
			task.ProjectID = ""
			if in.ProjectID.Value != nil && *in.ProjectID.Value != "" {
				p, exists := lockedProjects[*in.ProjectID.Value]
				if !exists {
					return TaskUpdateFieldError{"project_id", ErrTaskProjectNotFound}
				}
				if p.OwnerID != current.UserID {
					return TaskUpdateFieldError{"project_id", ErrTaskProjectNotFound}
				}
				task.ProjectID = domain.ProjectID(p.ID)
				if _, exists := beforeByProject[p.ID]; !exists {
					state, e := repos.ProjectLifecycle().CaptureWorkState(ctx, p.ID, asOf)
					if e != nil {
						return e
					}
					beforeByProject[p.ID] = state
				}
			} else {
				task.ProjectID = ""
			}
			projectChanged = string(task.ProjectID) != current.ProjectID
			if projectChanged {
				task.AssigneeID = domain.UserID(current.UserID)
			}
		}
		validated, err := domain.NewExistingTask(task.ID, task.UserID, task.ProjectID, task.Title, task.Description, task.DueDate, task.ManualEstimatedMinutes, task.ActualMinutes, task.Progress, task.Priority, task.Status, task.AssigneeID, task.CreatedAt, task.UpdatedAt)
		if err != nil {
			return err
		}
		result, err = repos.Tasks().UpdateByUserID(ctx, in.UserID, validated, in.ExpectedRevision)
		if err != nil {
			return err
		}
		result.CanUpdate = true
		if result.ProjectID != "" {
			if project, ok := lockedProjects[result.ProjectID]; ok {
				result.ProjectName = project.Title
			}
		}
		rows, err := EnrichTasksInRepositories(ctx, repos, in.UserID, []dao.Task{result}, asOf)
		if err != nil {
			return err
		}
		result = rows[0]
		for projectID, prior := range beforeByProject {
			if err = repos.ProjectLifecycle().ReconcileWorkState(ctx, string(in.UserID), projectID, prior, asOf); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

func taskFromDAO(task dao.Task) (domain.Task, error) {
	priority, err := domain.NewTaskPriority(task.Priority.Value)
	if err != nil {
		return domain.Task{}, err
	}
	status, err := domain.NewTaskStatus(task.Status.Value)
	if err != nil {
		return domain.Task{}, err
	}
	var dueDate time.Time
	if task.DueDate != 0 {
		dueDate = time.Unix(task.DueDate, 0)
	}
	return domain.NewExistingTask(
		domain.TaskID(task.ID), domain.UserID(task.UserID), domain.ProjectID(task.ProjectID),
		task.Title, task.Description, dueDate, copyOptionalInt(task.ManualEstimatedMinutes),
		copyOptionalInt(task.ActualMinutes), task.Progress, priority, status,
		domain.UserID(task.AssigneeID), time.Unix(task.CreatedAt, 0), time.Unix(task.UpdatedAt, 0),
	)
}

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
