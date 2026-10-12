package application

import (
	"context"
	"time"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
)

type allocationProjectLifecycleAdapter struct {
	repository *allocationrepository.AllocationRepository
}

func (adapter allocationProjectLifecycleAdapter) PauseProjectAllocations(ctx context.Context, actorID, projectID string, at time.Time) error {
	return allocationusecase.PauseProjectAllocations(ctx, adapter.repository, actorID, projectID, at)
}

func (adapter allocationProjectLifecycleAdapter) ResumeProjectAllocations(ctx context.Context, actorID, projectID string, at time.Time) error {
	return allocationusecase.ResumeProjectAllocations(ctx, adapter.repository, adapter.repository, actorID, projectID, at)
}

var _ interface {
	PauseProjectAllocations(context.Context, string, string, time.Time) error
	ResumeProjectAllocations(context.Context, string, string, time.Time) error
} = allocationProjectLifecycleAdapter{}
