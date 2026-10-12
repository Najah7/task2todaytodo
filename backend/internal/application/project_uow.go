package application

import (
	"context"
	"time"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
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
		allocationRepository := allocationrepository.NewAllocationRepository(uow.pool).WithTx(tx)
		return fn(ctx, uow.projects.WithTx(tx).Projects, projectChildrenAdapter{
			tasks:       uow.tasks.WithTx(tx).Tasks,
			schedules:   uow.schedules.WithTx(tx).Schedules,
			allocations: allocationProjectLifecycleAdapter{repository: allocationRepository},
		})
	})
}

type projectChildrenAdapter struct {
	tasks       *taskRepository.TaskRepository
	schedules   *scheduleRepository.ScheduleRepository
	allocations allocationProjectLifecycleAdapter
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

func (adapter projectChildrenAdapter) PauseProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error {
	return adapter.schedules.PauseProjectRules(ctx, actorID, projectID, at)
}

func (adapter projectChildrenAdapter) ResumeProjectRules(ctx context.Context, actorID, projectID string, at time.Time) error {
	return adapter.schedules.ResumeProjectRules(ctx, actorID, projectID, at)
}

func (adapter projectChildrenAdapter) PauseProjectAllocations(ctx context.Context, actorID, projectID string, at time.Time) error {
	return adapter.allocations.PauseProjectAllocations(ctx, actorID, projectID, at)
}

func (adapter projectChildrenAdapter) ResumeProjectAllocations(ctx context.Context, actorID, projectID string, at time.Time) error {
	return adapter.allocations.ResumeProjectAllocations(ctx, actorID, projectID, at)
}

var _ projectusecase.UnitOfWork = (*ProjectUOW)(nil)
var _ projectusecase.ProjectChildrenDeleter = projectChildrenAdapter{}
