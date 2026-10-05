package usecase

import (
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

func taskFrequenciesFromDAO(values []dao.TaskFrequency) (domain.TaskFrequencies, error) {
	frequencies := make(domain.TaskFrequencies, 0, len(values))
	for _, value := range values {
		frequency, err := domain.NewTaskFrequency(value.Value)
		if err != nil {
			return nil, err
		}
		frequencies = append(frequencies, frequency)
	}
	return frequencies, nil
}
