package application

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// lockTodoWorkflow serializes transactions that mutate source work or its
// generated daily projection. Every participating UOW acquires this before
// owner row locks so reverse Task↔Todo synchronization has one lock order.
func lockTodoWorkflow(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1414480719)`)
	return err
}
