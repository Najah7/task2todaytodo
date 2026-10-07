package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

var (
	ErrProjectMemberNotFound       = errors.New("project member not found")
	ErrProjectMemberRoleNotFound   = errors.New("project member role not found")
	ErrProjectMemberUserNotFound   = errors.New("project member user not found")
	ErrProjectMemberUpsertRejected = errors.New("project member could not be added")
	ErrProjectMemberInvalidInput   = errors.New("project member identifiers and role are required")
)

func validProjectMemberInput(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func logProjectMemberFailure(logger logging.Logger, ctx context.Context, operation string, err error) {
	for _, expected := range []error{
		ErrProjectMemberNotFound,
		ErrProjectMemberRoleNotFound,
		ErrProjectMemberUserNotFound,
		ErrProjectMemberUpsertRejected,
		ErrProjectMemberInvalidInput,
		ErrPermissionDenied,
		domain.ErrProjectNotFound,
	} {
		if errors.Is(err, expected) {
			return
		}
	}
	logUnexpectedProjectFailure(logger, ctx, operation, err)
}
