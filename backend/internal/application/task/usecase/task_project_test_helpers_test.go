package usecase

import "github.com/Najah7/task2todaytodo/internal/application/task/dao"

type legacyProjectFixture struct {
	ID       string
	UserID   string
	Priority dao.Priority
}
