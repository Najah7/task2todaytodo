package usecase

import (
	"context"
	"errors"
	"sort"

	"github.com/Najah7/task2todaytodo/internal/application/metric/dao"
)

const OverdueProjectLimit = 5

var ErrInvalidOverdueActor = errors.New("invalid overdue metrics actor")

type OverdueWorkReader interface {
	ReadOverdueWork(context.Context, string) (dao.OverdueWork, error)
}

type OverdueWorkUseCase struct {
	reader OverdueWorkReader
}

func NewOverdueWorkUseCase(reader OverdueWorkReader) *OverdueWorkUseCase {
	return &OverdueWorkUseCase{reader: reader}
}

func (uc *OverdueWorkUseCase) Execute(ctx context.Context, actorID string) (dao.OverdueWork, error) {
	if actorID == "" {
		return dao.OverdueWork{}, ErrInvalidOverdueActor
	}
	if uc.reader == nil {
		return dao.OverdueWork{}, ErrReaderUnavailable
	}
	result, err := uc.reader.ReadOverdueWork(ctx, actorID)
	if err != nil {
		return dao.OverdueWork{}, err
	}
	if result.Projects == nil {
		result.Projects = []dao.OverdueProject{}
	}
	sort.Slice(result.Projects, func(i, j int) bool {
		if result.Projects[i].Count != result.Projects[j].Count {
			return result.Projects[i].Count > result.Projects[j].Count
		}
		left, right := result.Projects[i].ProjectTitle, result.Projects[j].ProjectTitle
		if left == nil || right == nil {
			return left != nil
		}
		return *left < *right
	})
	if len(result.Projects) > OverdueProjectLimit {
		for _, project := range result.Projects[OverdueProjectLimit:] {
			result.OtherCount += project.Count
		}
		result.Projects = result.Projects[:OverdueProjectLimit]
	}
	return result, nil
}
