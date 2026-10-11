//go:build integration

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

func seedTask(t *testing.T, pool *pgxpool.Pool, timezone string) (domain.UserID, domain.TaskID) {
	t.Helper()
	ctx := t.Context()
	userID := domain.UserID(ulid.Make().String())
	taskID := domain.TaskID(ulid.Make().String())
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, first_name, last_name, email, password, timezone)
		VALUES ($1, 'Test', 'User', $2, 'not-used', $3)
	`, string(userID), string(userID)+"@example.test", timezone); err != nil {
		t.Fatalf("insert integration test user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks (id, user_id, title, priority, status)
		VALUES ($1, $2, 'Integration task', 'low', 'open')
	`, string(taskID), string(userID)); err != nil {
		t.Fatalf("insert integration test task: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM tasks WHERE id = $1`, string(taskID))
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, string(userID))
	})
	return userID, taskID
}
