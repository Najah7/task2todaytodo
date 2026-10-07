package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

var (
	ErrRevisionConflict = errors.New("resource revision does not match")
	ErrPermissionDenied = errors.New("permission denied")
)

func logUnexpectedProjectFailure(logger logging.Logger, ctx context.Context, operation string, err error) {
	if err == nil {
		return
	}
	for _, expected := range []error{domain.ErrProjectNotFound, domain.ErrProjectIDEmpty, domain.ErrProjectUserIDEmpty, domain.ErrProjectTitleEmpty, domain.ErrProjectProgressInvalid, domain.ErrProjectEndDateBeforeStartDate, domain.ErrProjectTypeEmpty, domain.ErrProjectTypeInvalid, domain.ErrProjectPriorityEmpty, domain.ErrProjectPriorityInvalid, ErrRevisionConflict, ErrPermissionDenied, ErrProjectPatchRequiredFieldNull, ErrInvalidProjectPage, ErrProjectMemberNotFound, ErrProjectMemberRoleNotFound, ErrProjectMemberUserNotFound, ErrProjectMemberUpsertRejected, ErrProjectMemberInvalidInput} {
		if errors.Is(err, expected) {
			return
		}
	}
	fields := append([]any{"operation", operation}, logging.ErrorFields(err)...)
	logging.OrNop(logger).Error(ctx, "project operation failed", fields...)
}
func logProjectRestoreFailure(logger logging.Logger, ctx context.Context, operation string, err error) {
	if err != nil {
		fields := append([]any{"operation", operation}, logging.ErrorFields(err)...)
		logging.OrNop(logger).Error(ctx, "failed to restore project", fields...)
	}
}
