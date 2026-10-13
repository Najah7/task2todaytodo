package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var (
	ErrRevisionConflict        = errors.New("resource revision does not match")
	ErrPermissionDenied        = errors.New("permission denied")
	ErrFreeProjectLimitReached = errors.New("free plan allows up to 3 active projects")
)

func logUnexpectedProjectFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err == nil {
		return
	}
	for _, expected := range []error{domain.ErrProjectNotFound, domain.ErrProjectIDEmpty, domain.ErrProjectUserIDEmpty, domain.ErrProjectTitleEmpty, domain.ErrProjectProgressInvalid, domain.ErrProjectEndDateBeforeStartDate, domain.ErrProjectTypeEmpty, domain.ErrProjectTypeInvalid, domain.ErrProjectPriorityEmpty, domain.ErrProjectPriorityInvalid, ErrRevisionConflict, ErrPermissionDenied, ErrFreeProjectLimitReached, ErrProjectPatchRequiredFieldNull, ErrInvalidProjectPage, ErrProjectMemberNotFound, ErrProjectMemberRoleNotFound, ErrProjectMemberUserNotFound, ErrProjectMemberUpsertRejected, ErrProjectMemberInvalidInput} {
		if errors.Is(err, expected) {
			return
		}
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	telemetry.OrNop(logger).Error(ctx, "project operation failed", fields...)
}
func logProjectRestoreFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err != nil {
		fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
		telemetry.OrNop(logger).Error(ctx, "failed to restore project", fields...)
	}
}
