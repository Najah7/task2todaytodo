package usecase

import (
	"context"
	"testing"
)

func TestListOperationsChecksPermissionBeforeRead(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		repo := &operationRepoFake{permission: true}
		if _, err := NewListOperationsUseCase(repo).Execute(context.Background(), "admin", "cursor", 10); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if repo.permissionCalls != 1 || repo.listCalls != 1 {
			t.Fatalf("permission calls=%d, list calls=%d", repo.permissionCalls, repo.listCalls)
		}
	})
	assertOperationReadDenied(t, func(repo *operationRepoFake, actor string) error {
		_, err := NewListOperationsUseCase(repo).Execute(context.Background(), actor, "cursor", 10)
		return err
	})
}
