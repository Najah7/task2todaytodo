package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type operationRepoFake struct {
	operation       dao.Operation
	permission      bool
	permissionErr   error
	permissionCalls int
	createCalls     int
	getCalls        int
	listCalls       int
	created         bool
	token           string
	starts          int
	completed       bool
	failed          bool
	done            chan struct{}
	failureCode     string
	failureMessage  string
}

func (r *operationRepoFake) HasPermission(context.Context, string, shared.Capability) (bool, error) {
	r.permissionCalls++
	return r.permission, r.permissionErr
}

func (r *operationRepoFake) CreateOrGetActive(_ context.Context, op dao.Operation, token string, _ time.Duration) (dao.Operation, bool, string, error) {
	r.createCalls++
	created := r.operation.ID == ""
	if created {
		r.operation, r.token = op, token
	}
	return r.operation, created, r.token, nil
}
func (r *operationRepoFake) Get(context.Context, string) (dao.Operation, error) {
	r.getCalls++
	return r.operation, nil
}
func (r *operationRepoFake) List(context.Context, string, int) ([]dao.Operation, error) {
	r.listCalls++
	return nil, nil
}
func (r *operationRepoFake) Heartbeat(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (r *operationRepoFake) Progress(context.Context, string, string, int64, int64, time.Duration) (bool, error) {
	return true, nil
}
func (r *operationRepoFake) Succeed(context.Context, string, string, int64, int64) error {
	r.completed = true
	if r.done != nil {
		close(r.done)
	}
	return nil
}
func (r *operationRepoFake) Fail(_ context.Context, _, _ string, code, message string) error {
	r.failed = true
	r.failureCode, r.failureMessage = code, message
	if r.done != nil {
		close(r.done)
	}
	return nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type sequentialIDs struct{ n int }

func (i *sequentialIDs) Generate() string { i.n++; return "id" }

type runnerFake struct{ started int }

func (r *runnerFake) StartOperation(string, string, string, time.Time) { r.started++ }

func TestRunBackfillPersistsBeforeStartingAndReusesActiveOperation(t *testing.T) {
	now := time.Date(2026, 10, 11, 1, 2, 3, 0, time.FixedZone("JST", 9*60*60))
	repo, runner := &operationRepoFake{permission: true}, &runnerFake{}
	uc := NewRunBackfillSchedulesUseCase(repo, runner, &sequentialIDs{}, fixedClock{now: now}, time.Minute)
	first, err := uc.Execute(context.Background(), "actor")
	if err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	if !first.Cutoff.Equal(now.UTC()) || first.JobName != backfillJobName {
		t.Fatalf("operation = %+v", first)
	}
	if runner.started != 1 {
		t.Fatalf("runner starts = %d, want 1", runner.started)
	}
	second, err := uc.Execute(context.Background(), "actor")
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if second.ID != first.ID || runner.started != 1 {
		t.Fatalf("second = %+v, runner starts = %d", second, runner.started)
	}
}

func TestRunBackfillRequiresPermissionBeforeCreatingOrStarting(t *testing.T) {
	for _, test := range []struct {
		name          string
		actor         string
		allowed       bool
		permissionErr error
		wantErr       error
	}{
		{name: "empty actor", actor: "", allowed: true, wantErr: ErrPermissionDenied},
		{name: "denied", actor: "ordinary-user", wantErr: ErrPermissionDenied},
		{name: "permission error", actor: "ordinary-user", permissionErr: errors.New("permission database failure"), wantErr: errors.New("permission database failure")},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, runner := &operationRepoFake{permission: test.allowed, permissionErr: test.permissionErr}, &runnerFake{}
			uc := NewRunBackfillSchedulesUseCase(repo, runner, &sequentialIDs{}, fixedClock{now: time.Now()}, time.Minute)
			_, err := uc.Execute(context.Background(), test.actor)
			if err == nil || err.Error() != test.wantErr.Error() {
				t.Fatalf("Execute() error = %v, want %v", err, test.wantErr)
			}
			if repo.createCalls != 0 || runner.started != 0 {
				t.Fatalf("denied request touched operation: create calls=%d, runner starts=%d", repo.createCalls, runner.started)
			}
			wantPermissionCalls := 1
			if test.actor == "" {
				wantPermissionCalls = 0
			}
			if repo.permissionCalls != wantPermissionCalls {
				t.Fatalf("permission calls = %d, want %d", repo.permissionCalls, wantPermissionCalls)
			}
		})
	}
}
