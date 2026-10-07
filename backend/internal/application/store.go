package application

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	Auth     AuthStore
	Project  ProjectStore
	Task     TaskStore
	Schedule ScheduleStore
	Tag      TagStore
}

func NewStore(pool *pgxpool.Pool) Store {
	return Store{
		Auth:     newAuthStore(pool),
		Project:  newProjectStore(pool),
		Task:     newTaskStore(pool),
		Schedule: newScheduleStore(pool),
		Tag:      newTagStore(pool),
	}
}
