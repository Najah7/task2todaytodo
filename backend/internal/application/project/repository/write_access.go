package repository

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/jackc/pgx/v5"
)

// GetProjectOwnerAndMemberCount returns project owner and actual member rows.
// Invitation rows are intentionally excluded because pending invitations do
// not make a Project shared.
func (r *ProjectRepository) GetProjectOwnerAndMemberCount(ctx context.Context, projectID domain.ProjectID) (string, int64, error) {
	facts, err := r.queries.GetProjectWriteAccessFacts(ctx, string(projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", 0, domain.ErrProjectNotFound
		}
		return "", 0, err
	}
	return facts.OwnerUserID, facts.MemberCount, nil
}
