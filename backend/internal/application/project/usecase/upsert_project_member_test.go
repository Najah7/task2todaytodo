package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
)

func TestUpsertProjectMemberRejectsSystemRoleBeforeRepository(t *testing.T) {
	uc := NewUpsertProjectMemberUseCase(nil, nil)
	err := uc.Execute(context.Background(), domain.UserID("actor"), domain.ProjectID("project"), domain.UserID("member"), "sysadmin")
	if !errors.Is(err, ErrProjectMemberInvalidInput) {
		t.Fatalf("Execute error = %v, want invalid member input", err)
	}
}
