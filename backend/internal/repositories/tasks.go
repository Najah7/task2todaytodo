package repositories

import (
	"context"
	"errors"

	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
	"github.com/jackc/pgx/v5"
)

var _ domain.TaskRepository = TaskRepository{}

type TaskRepository struct {
	queries *sqlc.Queries
}

func NewTaskRepository(db sqlc.DBTX) *TaskRepository {
	return &TaskRepository{
		queries: sqlc.New(db),
	}
}

func (r TaskRepository) Get(ctx context.Context, id domain.TaskID) (domain.Task, error) {
	record, err := r.queries.GetTask(ctx, string(id))
	if err != nil {
		return domain.NewZeroTask(), err
	}
	return recordToTask(record)
}

func (r TaskRepository) GetByUser(ctx context.Context, userID domain.UserID, id domain.TaskID) (domain.Task, error) {
	record, err := r.queries.GetTaskByUser(ctx, sqlc.GetTaskByUserParams{
		ID:     string(id),
		UserID: string(userID),
	})
	if err != nil {
		return domain.NewZeroTask(), taskRepositoryError(err)
	}
	return recordToTask(record)
}

func (r TaskRepository) GetAggregateByUser(ctx context.Context, userID domain.UserID, id domain.TaskID) (domain.TaskAggregate, error) {
	task, err := r.GetByUser(ctx, userID, id)
	if err != nil {
		return domain.TaskAggregate{}, err
	}

	itemRecords, err := r.queries.ListTodoItemsByTaskForUser(ctx, sqlc.ListTodoItemsByTaskForUserParams{
		TaskID: string(id),
		UserID: string(userID),
	})
	if err != nil {
		return domain.TaskAggregate{}, err
	}
	items, err := recordsToTodoItemsForTaskRows(itemRecords)
	if err != nil {
		return domain.TaskAggregate{}, err
	}

	scheduleRecords, err := r.queries.ListTaskSchedulesByTaskForUser(ctx, sqlc.ListTaskSchedulesByTaskForUserParams{
		TaskID: string(id),
		UserID: string(userID),
	})
	if err != nil {
		return domain.TaskAggregate{}, err
	}
	schedules, err := recordsToTaskSchedulesForTaskRows(scheduleRecords)
	if err != nil {
		return domain.TaskAggregate{}, err
	}

	return domain.TaskAggregate{
		Task:          task,
		TodoItems:     items,
		TaskSchedules: schedules,
	}, nil
}

func (r TaskRepository) GetByFrequency(ctx context.Context, frequency domain.TaskFrequency) ([]domain.Task, error) {
	records, err := r.queries.GetTaskByFrequency(ctx, frequency.String())
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records)
}

func (r TaskRepository) GetByPriority(ctx context.Context, priority domain.TaskPriority) ([]domain.Task, error) {
	records, err := r.queries.GetTaskByPriority(ctx, priority.String())
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records)
}

func (r TaskRepository) GetByProject(ctx context.Context, projectID domain.ProjectID) ([]domain.Task, error) {
	records, err := r.queries.GetTaskByProject(ctx, string(projectID))
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records)
}

func (r TaskRepository) GetByProjectType(ctx context.Context, projectType domain.ProjectType) ([]domain.Task, error) {
	records, err := r.queries.GetTaskByProjectType(ctx, projectType.String())
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records)
}

func (r TaskRepository) GetByStatus(ctx context.Context, status domain.TaskStatus) ([]domain.Task, error) {
	records, err := r.queries.GetTaskByStatus(ctx, status.String())
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records)
}

func (r TaskRepository) GetByTag(ctx context.Context, tagID string) ([]domain.Task, error) {
	records, err := r.queries.GetTaskByTag(ctx, tagID)
	if err != nil {
		return nil, err
	}
	return recordsToTasks(records)
}

func (r TaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	record, err := r.queries.CreateTask(ctx, sqlc.CreateTaskParams{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		ProjectID:        stringToPgText(string(task.ProjectID)),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
		Progress:         int16(task.Progress),
		DueDate:          timeToPgDate(task.DueDate),
		Priority:         taskPriorityString(task.Priority),
		Status:           taskStatusString(task.Status),
	})
	if err != nil {
		return domain.NewZeroTask(), err
	}
	return recordToTask(record)
}

func (r TaskRepository) CreateInProject(ctx context.Context, task domain.Task) (domain.Task, error) {
	record, err := r.queries.CreateTaskInProject(ctx, sqlc.CreateTaskInProjectParams{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		ProjectID:        string(task.ProjectID),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
		Progress:         int16(task.Progress),
		DueDate:          timeToPgDate(task.DueDate),
		Priority:         taskPriorityString(task.Priority),
		Status:           taskStatusString(task.Status),
	})
	if err != nil {
		return domain.NewZeroTask(), taskProjectRepositoryError(err)
	}
	return recordToTask(record)
}

func (r TaskRepository) Update(ctx context.Context, task domain.Task) (domain.Task, error) {
	record, err := r.queries.UpdateTask(ctx, sqlc.UpdateTaskParams{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		ProjectID:        stringToPgText(string(task.ProjectID)),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
		Progress:         int16(task.Progress),
		DueDate:          timeToPgDate(task.DueDate),
		Priority:         taskPriorityString(task.Priority),
		Status:           taskStatusString(task.Status),
	})
	if err != nil {
		return domain.NewZeroTask(), err
	}
	return recordToTask(record)
}

func (r TaskRepository) UpdateByUser(ctx context.Context, task domain.Task) (domain.Task, error) {
	record, err := r.queries.UpdateTaskByUser(ctx, sqlc.UpdateTaskByUserParams{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		ProjectID:        stringToPgText(string(task.ProjectID)),
		Title:            task.Title,
		Description:      stringToPgText(task.Description),
		EstimatedMinutes: intPointerToPgInt(task.EstimatedMinutes),
		ActualMinutes:    intPointerToPgInt(task.ActualMinutes),
		Progress:         int16(task.Progress),
		DueDate:          timeToPgDate(task.DueDate),
		Priority:         taskPriorityString(task.Priority),
		Status:           taskStatusString(task.Status),
	})
	if err != nil {
		return domain.NewZeroTask(), taskRepositoryError(err)
	}
	return recordToTask(record)
}

func (r TaskRepository) Delete(ctx context.Context, id domain.TaskID) error {
	return r.queries.DeleteTask(ctx, string(id))
}

func (r TaskRepository) DeleteByUser(ctx context.Context, userID domain.UserID, id domain.TaskID) error {
	_, err := r.queries.DeleteTaskByUser(ctx, sqlc.DeleteTaskByUserParams{
		ID:     string(id),
		UserID: string(userID),
	})
	return taskRepositoryError(err)
}

func taskRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTaskNotFound
	}
	return err
}

func taskProjectRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTaskProjectNotFound
	}
	return err
}
