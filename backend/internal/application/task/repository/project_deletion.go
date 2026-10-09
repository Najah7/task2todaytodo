package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
)

func (r TaskRepository) DeleteProjectTasksByActor(ctx context.Context, actorID, projectID string) error {
	_, err := r.queries.DeleteProjectTasksByActor(ctx, sqlc.DeleteProjectTasksByActorParams{
		ActorID:   actorID,
		ProjectID: projectID,
	})
	if err != nil {
		return err
	}
	// Use a new statement snapshot after any task-row lock waits so ActionItems
	// committed by an in-flight create are included in Project deletion.
	_, err = r.queries.DeleteProjectActionItemsByActor(ctx, projectID)
	return err
}
