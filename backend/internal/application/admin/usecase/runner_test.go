package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/dao"
)

type backfillBlockingFake struct{ started, release chan struct{} }

func (f backfillBlockingFake) Execute(ctx context.Context, actor string, cutoff time.Time, report func(int, int) error) (int, int, error) {
	if actor != "actor" || cutoff.IsZero() {
		panic("unexpected execution input")
	}
	if err := report(3, 8); err != nil {
		return 3, 8, err
	}
	close(f.started)
	select {
	case <-f.release:
		return 3, 8, nil
	case <-ctx.Done():
		return 3, 8, ctx.Err()
	}
}

type backfillErrorFake struct{ err error }

func (f backfillErrorFake) Execute(context.Context, string, time.Time, func(int, int) error) (int, int, error) {
	return 0, 0, f.err
}

func TestAsyncRunnerRunsOutsideRequestContextAndCompletes(t *testing.T) {
	repo := &operationRepoFake{permission: true, done: make(chan struct{})}
	process, cancelProcess := context.WithCancel(context.Background())
	executor := backfillBlockingFake{started: make(chan struct{}), release: make(chan struct{})}
	runner := NewAsyncRunner(repo, executor, process, time.Second, 10*time.Millisecond)
	uc := NewRunBackfillSchedulesUseCase(repo, runner, &sequentialIDs{}, fixedClock{now: time.Now()}, time.Second)
	request, cancelRequest := context.WithCancel(context.Background())
	if _, err := uc.Execute(request, "actor"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	cancelRequest()
	close(executor.release)
	select {
	case <-repo.done:
	case <-time.After(time.Second):
		t.Fatal("operation did not complete")
	}
	if !repo.completed || repo.processed != 3 || repo.generated != 8 {
		t.Fatalf("completed progress = %v, %d, %d; want true, 3, 8", repo.completed, repo.processed, repo.generated)
	}
	cancelProcess()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestAsyncRunnerStoresSafeFailureAndShutdownWaits(t *testing.T) {
	repo := &operationRepoFake{operation: daoOperation("op"), done: make(chan struct{})}
	process, cancelProcess := context.WithCancel(context.Background())
	runner := NewAsyncRunner(repo, backfillErrorFake{err: errors.New("postgres://secret/raw failure")}, process, time.Second, 10*time.Millisecond)
	runner.StartOperation("op", "lease", "actor", time.Now().UTC())
	select {
	case <-repo.done:
	case <-time.After(time.Second):
		t.Fatal("failure was not persisted")
	}
	if !repo.failed || repo.failureCode != "backfill_failed" || repo.failureMessage != "Schedule backfill failed. Run job again to resume from saved progress." {
		t.Fatalf("failure details = %q, %q", repo.failureCode, repo.failureMessage)
	}
	cancelProcess()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestAsyncRunnerShutdownCancelsActiveExecution(t *testing.T) {
	repo := &operationRepoFake{operation: daoOperation("op"), done: make(chan struct{})}
	executing := make(chan struct{})
	executor := backfillBlockingFake{started: executing, release: make(chan struct{})}
	runner := NewAsyncRunner(repo, executor, context.Background(), time.Second, 10*time.Millisecond)
	runner.StartOperation("op", "lease", "actor", time.Now().UTC())
	select {
	case <-executing:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if !repo.failed || repo.failureCode != "interrupted" {
		t.Fatalf("shutdown failure = %v %q", repo.failed, repo.failureCode)
	}
	if repo.processed != 3 || repo.generated != 8 {
		t.Fatalf("interrupted progress = %d, %d; want 3, 8", repo.processed, repo.generated)
	}
}

func TestAsyncRunnerStopsWhenLeaseIsLost(t *testing.T) {
	repo := &leaseLostRepository{operationRepoFake: &operationRepoFake{operation: daoOperation("op")}}
	executor := backfillObservedCancellation{started: make(chan struct{}), stopped: make(chan struct{})}
	runner := NewAsyncRunner(repo, executor, context.Background(), 30*time.Millisecond, 5*time.Millisecond)
	runner.StartOperation("op", "lease", "actor", time.Now().UTC())
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	select {
	case <-executor.stopped:
	case <-time.After(time.Second):
		t.Fatal("executor did not stop after lease loss")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if repo.completed || repo.failed {
		t.Fatalf("lost-lease operation wrote terminal state: completed=%v failed=%v", repo.completed, repo.failed)
	}
}

func TestAsyncRunnerCancelsExecutionWhenHeartbeatCallTimesOut(t *testing.T) {
	repo := &blockingHeartbeatRepository{
		operationRepoFake: &operationRepoFake{operation: daoOperation("op")},
		started:           make(chan struct{}),
		cancelled:         make(chan struct{}),
	}
	executor := backfillObservedCancellation{started: make(chan struct{}), stopped: make(chan struct{})}
	runner := NewAsyncRunner(repo, executor, context.Background(), 300*time.Millisecond, 20*time.Millisecond)
	runner.StartOperation("op", "lease", "actor", time.Now().UTC())
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	select {
	case <-repo.started:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not start")
	}
	select {
	case <-repo.cancelled:
	case <-time.After(time.Second):
		t.Fatal("heartbeat repository call did not receive a deadline")
	}
	select {
	case <-executor.stopped:
	case <-time.After(time.Second):
		t.Fatal("executor did not stop after uncertain heartbeat")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if repo.completed || repo.failed {
		t.Fatalf("heartbeat timeout wrote terminal state: completed=%v failed=%v", repo.completed, repo.failed)
	}
}

func TestAsyncRunnerShutdownDuringHeartbeatPersistsInterruptedState(t *testing.T) {
	repo := &blockingHeartbeatRepository{
		operationRepoFake: &operationRepoFake{operation: daoOperation("op")},
		started:           make(chan struct{}),
		cancelled:         make(chan struct{}),
	}
	executor := backfillObservedCancellation{started: make(chan struct{}), stopped: make(chan struct{})}
	runner := NewAsyncRunner(repo, executor, context.Background(), 3*time.Second, 500*time.Millisecond)
	runner.StartOperation("op", "lease", "actor", time.Now().UTC())
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	select {
	case <-repo.started:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	select {
	case <-repo.cancelled:
	default:
		t.Fatal("process shutdown did not cancel the in-flight heartbeat call")
	}
	if !repo.failed || repo.failureCode != "interrupted" {
		t.Fatalf("shutdown during heartbeat failure = %v %q; want interrupted", repo.failed, repo.failureCode)
	}
}

type backfillObservedCancellation struct{ started, stopped chan struct{} }

func (f backfillObservedCancellation) Execute(ctx context.Context, _ string, _ time.Time, _ func(int, int) error) (int, int, error) {
	close(f.started)
	<-ctx.Done()
	close(f.stopped)
	return 0, 0, ctx.Err()
}

type leaseLostRepository struct{ *operationRepoFake }

func (*leaseLostRepository) Heartbeat(context.Context, string, string, time.Duration) (bool, error) {
	return false, nil
}

type blockingHeartbeatRepository struct {
	*operationRepoFake
	started   chan struct{}
	cancelled chan struct{}
}

func (repo *blockingHeartbeatRepository) Heartbeat(ctx context.Context, _ string, _ string, _ time.Duration) (bool, error) {
	close(repo.started)
	<-ctx.Done()
	close(repo.cancelled)
	return false, ctx.Err()
}

func daoOperation(id string) dao.Operation { return dao.Operation{ID: id, State: "running"} }
