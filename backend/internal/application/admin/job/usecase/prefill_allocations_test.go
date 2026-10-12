package usecase

import (
	"context"
	"testing"
	"time"
)

type namedJobRunnerFake struct {
	jobs []string
}

func (*namedJobRunnerFake) StartOperation(string, string, string, time.Time) {}
func (r *namedJobRunnerFake) StartJob(jobName, _, _, _ string, _ time.Time) {
	r.jobs = append(r.jobs, jobName)
}

func TestPrefillAllocationsStartsNamedJob(t *testing.T) {
	repo := &operationRepoFake{permission: true}
	runner := &namedJobRunnerFake{}
	uc := NewPrefillAllocationsUseCase(repo, runner, &sequentialIDs{}, fixedClock{now: time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)}, time.Minute)
	op, err := uc.Execute(context.Background(), "admin")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if op.JobName != prefillAllocationsJobName {
		t.Fatalf("operation JobName = %q; want %q", op.JobName, prefillAllocationsJobName)
	}
	if len(runner.jobs) != 1 || runner.jobs[0] != prefillAllocationsJobName {
		t.Fatalf("started jobs = %v; want [%q]", runner.jobs, prefillAllocationsJobName)
	}
}
