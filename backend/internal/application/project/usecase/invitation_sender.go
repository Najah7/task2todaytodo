package usecase

import (
	"context"
	"time"
)

const ProjectInvitationLifetime = 7 * 24 * time.Hour

type ProjectInvitationSender interface {
	SendProjectInvitation(ctx context.Context, to, frontendURL, rawToken, projectTitle, roleName string, lifetime time.Duration) error
}
