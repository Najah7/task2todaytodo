package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
)

type projectProgressSourceFake struct {
	sources dao.ProjectProgressSources
	ids     []string
	calls   int
	err     error
}

func (f *projectProgressSourceFake) ReadProjectProgressSources(_ context.Context, ids []string, _ time.Time) (dao.ProjectProgressSources, error) {
	f.calls++
	f.ids = append([]string(nil), ids...)
	return f.sources, f.err
}
