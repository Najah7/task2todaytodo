package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared/status"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type projectOptionsRepository interface {
	ListProjectTypes(context.Context) ([]dao.ProjectType, error)
	ListProjectPriorities(context.Context) ([]dao.Priority, error)
	ListProjectStatuses(context.Context) ([]dao.ProjectStatusOption, error)
}

type ListProjectOptionsUseCase struct {
	repo   projectOptionsRepository
	logger logging.Logger
}

func NewListProjectOptionsUseCase(repo projectOptionsRepository, logger logging.Logger) *ListProjectOptionsUseCase {
	return &ListProjectOptionsUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *ListProjectOptionsUseCase) Execute(ctx context.Context) (output ProjectOptions, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectOptionsUseCase.Execute", err) }()
	types, err := uc.repo.ListProjectTypes(ctx)
	if err != nil {
		return ProjectOptions{}, err
	}
	priorities, err := uc.repo.ListProjectPriorities(ctx)
	if err != nil {
		return ProjectOptions{}, err
	}
	storedStatuses, err := uc.repo.ListProjectStatuses(ctx)
	if err != nil {
		return ProjectOptions{}, err
	}
	statusByValue := make(map[string]dao.ProjectStatusOption, len(storedStatuses))
	for _, item := range storedStatuses {
		statusByValue[item.Value] = item
	}
	output = ProjectOptions{
		Types: make([]ProjectOption, 0, len(types)), Priorities: make([]ProjectOption, 0, len(priorities)),
		Statuses: make([]ProjectOption, 0, len(status.All())),
	}
	for _, item := range types {
		output.Types = append(output.Types, ProjectOption{Value: item.Value, Label: item.Label, LabelJp: item.LabelJp})
	}
	for _, item := range priorities {
		output.Priorities = append(output.Priorities, ProjectOption{Value: item.Value, Label: item.Label, LabelJp: item.LabelJp, Weight: item.Weight})
	}
	for _, item := range status.All() {
		stored, ok := statusByValue[item.Value]
		if ok {
			output.Statuses = append(output.Statuses, ProjectOption{Value: stored.Value, Label: stored.Label, LabelJp: stored.LabelJp})
		}
	}
	return output, nil
}
