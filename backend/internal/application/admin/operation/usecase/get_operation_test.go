package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestGetOperationChecksPermissionBeforeRead(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		repo := &operationRepoFake{permission: true}
		if _, err := NewGetOperationUseCase(repo).Execute(context.Background(), "admin", "op"); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if repo.permissionCalls != 1 || repo.getCalls != 1 {
			t.Fatalf("permission calls=%d, get calls=%d", repo.permissionCalls, repo.getCalls)
		}
	})
	assertOperationReadDenied(t, func(repo *operationRepoFake, actor string) error {
		_, err := NewGetOperationUseCase(repo).Execute(context.Background(), actor, "op")
		return err
	})
}

func assertOperationReadDenied(t *testing.T, execute func(*operationRepoFake, string) error) {
	t.Helper()
	for _, test := range []struct {
		name          string
		actor         string
		allowed       bool
		permissionErr error
		wantErr       error
	}{
		{name: "empty actor", actor: "", allowed: true, wantErr: ErrPermissionDenied},
		{name: "denied", actor: "ordinary-user", wantErr: ErrPermissionDenied},
		{name: "permission error", actor: "ordinary-user", permissionErr: errors.New("permission database failure")},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &operationRepoFake{permission: test.allowed, permissionErr: test.permissionErr}
			err := execute(repo, test.actor)
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("Execute() error = %v, want %v", err, test.wantErr)
			}
			if test.permissionErr != nil && !errors.Is(err, test.permissionErr) {
				t.Fatalf("Execute() error = %v, want permission error %v", err, test.permissionErr)
			}
			if err == nil {
				t.Fatal("Execute() error = nil, want permission rejection")
			}
			wantPermissionCalls := 1
			if test.actor == "" {
				wantPermissionCalls = 0
			}
			if repo.permissionCalls != wantPermissionCalls || repo.getCalls != 0 || repo.listCalls != 0 {
				t.Fatalf("permission calls=%d, get calls=%d, list calls=%d", repo.permissionCalls, repo.getCalls, repo.listCalls)
			}
		})
	}
}
