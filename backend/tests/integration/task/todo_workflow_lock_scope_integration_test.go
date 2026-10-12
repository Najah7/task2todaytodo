//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
)

func TestSafeTaskReadDoesNotWaitForTodoWorkflowLockButWriteDoes(t *testing.T) {
	pool := integrationPool(t)
	userID, taskID := seedTask(t, pool, "UTC")
	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	usecases, closeUseCases := application.NewUseCase(store, uow, adapter.NewIDIssuer(), telemetry.Nop())
	testdb.RegisterShutdown(t, closeUseCases)

	initial, err := usecases.Task.Get.Execute(t.Context(), userID, taskID)
	if err != nil {
		t.Fatalf("get initial Task: %v", err)
	}

	lockCtx, cancelLock := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelLock()
	lockTx, err := pool.Begin(lockCtx)
	if err != nil {
		t.Fatalf("begin workflow-lock transaction: %v", err)
	}
	defer func() { _ = lockTx.Rollback(context.Background()) }()
	if _, err := lockTx.Exec(lockCtx, `SELECT pg_advisory_xact_lock(1414480719)`); err != nil {
		t.Fatalf("hold Todo workflow lock: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := usecases.Task.Get.Execute(ctx, userID, taskID)
		readDone <- err
	}()
	select {
	case err := <-readDone:
		if err != nil {
			t.Fatalf("safe Task read failed while workflow lock was held: %v", err)
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("safe Task read waited for the Todo workflow lock")
	}

	newTitle := "workflow write after lock release"
	writeDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := usecases.Task.Update.Execute(ctx, taskusecase.UpdateTaskInput{
			UserID:           userID,
			TaskID:           taskID,
			ExpectedRevision: initial.Revision,
			Title:            taskusecase.PatchField[string]{Present: true, Value: &newTitle},
		})
		writeDone <- err
	}()
	waiterDeadline := time.Now().Add(3 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(t.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_locks
				WHERE locktype = 'advisory' AND NOT granted
				  AND classid::bigint = 0 AND objid::bigint = 1414480719 AND objsubid = 1
			)`).Scan(&waiting); err != nil {
			t.Fatalf("check Todo workflow lock waiters: %v", err)
		}
		if waiting {
			break
		}
		if time.Now().After(waiterDeadline) {
			t.Fatal("Task write did not become an advisory-lock waiter")
		}
		select {
		case err := <-writeDone:
			t.Fatalf("Task write returned before workflow lock release: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}

	if err := lockTx.Commit(lockCtx); err != nil {
		t.Fatalf("release Todo workflow lock: %v", err)
	}
	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("Task write failed after workflow lock release: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Task write stayed blocked after workflow lock release")
	}
	var savedTitle string
	if err := pool.QueryRow(t.Context(), `SELECT title FROM tasks WHERE id=$1`, taskID).Scan(&savedTitle); err != nil {
		t.Fatalf("read Task after workflow write: %v", err)
	}
	if savedTitle != newTitle {
		t.Fatalf("Task title after workflow lock release = %q, want %q", savedTitle, newTitle)
	}
}
