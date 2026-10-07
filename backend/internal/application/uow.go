package application

import (
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UOW struct {
	Auth     authusecase.UOW
	Project  projectusecase.UnitOfWork
	Task     taskusecase.UOW
	Schedule scheduleusecase.UOW
}

func NewUOW(pool *pgxpool.Pool, authStore AuthStore, projectStore ProjectStore, taskStore TaskStore, scheduleStore ScheduleStore) UOW {
	return UOW{
		Auth:     NewAuthUOW(pool, authStore),
		Project:  NewProjectUOW(pool, projectStore, taskStore, scheduleStore),
		Task:     NewTaskUOW(pool, taskStore),
		Schedule: NewScheduleUOW(pool, scheduleStore),
	}
}
