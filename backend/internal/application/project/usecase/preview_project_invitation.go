package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ProjectInvitationPreview struct {
	Invitation dao.ProjectInvitation
	Status     string
}

type PreviewProjectInvitationUseCase struct {
	repo   Repository
	logger telemetry.Logger
	now    func() time.Time
}

func NewPreviewProjectInvitationUseCase(repo Repository, logger telemetry.Logger) *PreviewProjectInvitationUseCase {
	return &PreviewProjectInvitationUseCase{repo: repo, logger: telemetry.OrNop(logger), now: time.Now}
}

func (uc *PreviewProjectInvitationUseCase) Execute(ctx context.Context, rawToken string) (ProjectInvitationPreview, error) {
	hash, err := parseProjectInvitationToken(rawToken)
	if err != nil {
		return ProjectInvitationPreview{}, err
	}
	invitation, err := uc.repo.GetProjectInvitationByTokenHash(ctx, hash)
	if err != nil {
		return ProjectInvitationPreview{}, err
	}
	return ProjectInvitationPreview{Invitation: invitation, Status: invitationStatus(invitation, uc.now().UTC())}, nil
}
