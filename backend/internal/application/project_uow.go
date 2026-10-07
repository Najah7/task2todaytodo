package application

import (
	"context"

	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleDomain "github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleRepository "github.com/Najah7/task2todaytodo/internal/application/schedule/repository"
	taskDomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskRepository "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectUOW struct {
	pool      *pgxpool.Pool
	projects  ProjectStore
	tasks     TaskStore
	schedules ScheduleStore
}

func NewProjectUOW(pool *pgxpool.Pool, projects ProjectStore, tasks TaskStore, schedules ScheduleStore) projectusecase.UnitOfWork {
	return &ProjectUOW{pool: pool, projects: projects, tasks: tasks, schedules: schedules}
}

func (uow *ProjectUOW) Do(ctx context.Context, fn func(context.Context, projectusecase.Repository, projectusecase.ProjectChildrenDeleter) error) error {
	return RunInTx(ctx, uow.pool, func(tx pgx.Tx) error {
		return fn(ctx, uow.projects.WithTx(tx).Projects, projectChildrenAdapter{
			tasks:     uow.tasks.WithTx(tx).Tasks,
			schedules: uow.schedules.WithTx(tx).Schedules,
		})
	})
}

type projectChildrenAdapter struct {
	tasks     *taskRepository.TaskRepository
	schedules *scheduleRepository.ScheduleRepository
}

func (adapter projectChildrenAdapter) DeleteProjectTasks(ctx context.Context, actorID, projectID string) error {
	return adapter.tasks.DeleteProjectTasksByActor(ctx, actorID, projectID)
}

func (adapter projectChildrenAdapter) DeleteProjectSchedules(ctx context.Context, actorID, projectID string) error {
	return adapter.schedules.DeleteProjectSchedulesByActor(ctx, scheduleDomain.UserID(actorID), scheduleDomain.ProjectID(projectID))
}

func (adapter projectChildrenAdapter) ReassignTasksAfterProjectMemberRemoval(ctx context.Context, actorID, projectID, memberID string) error {
	return adapter.tasks.ReassignProjectMemberTasks(ctx, taskDomain.UserID(actorID), taskDomain.ProjectID(projectID), taskDomain.UserID(memberID))
}

func (adapter projectChildrenAdapter) ReassignSchedulesAfterProjectMemberRemoval(ctx context.Context, actorID, projectID, memberID string) error {
	return adapter.schedules.ReassignProjectMemberSchedules(ctx, scheduleDomain.UserID(actorID), scheduleDomain.ProjectID(projectID), scheduleDomain.UserID(memberID))
}

var _ projectusecase.UnitOfWork = (*ProjectUOW)(nil)
var _ projectusecase.ProjectChildrenDeleter = projectChildrenAdapter{}
