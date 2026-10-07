package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

// ReassignProjectMemberTasks is called by Project's transaction-scoped child
// port when a member leaves. SQL writes changed_by so Task history records the
// Project owner as the actor.
func (r TaskRepository) ReassignProjectMemberTasks(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error {
	_, err := r.queries.ReassignProjectMemberTasks(ctx, sqlc.ReassignProjectMemberTasksParams{
		ActorID: string(actor), ProjectID: string(project), MemberID: string(member),
	})
	return err
}
