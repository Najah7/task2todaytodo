package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type operationRepoFake struct {
	operation       dao.Operation
	permission      bool
	permissionErr   error
	permissionCalls int
	getCalls        int
	listCalls       int
}

func (r *operationRepoFake) HasPermission(context.Context, string, shared.Capability) (bool, error) {
	r.permissionCalls++
	return r.permission, r.permissionErr
}

func (r *operationRepoFake) Get(context.Context, string) (dao.Operation, error) {
	r.getCalls++
	return r.operation, nil
}

func (r *operationRepoFake) List(context.Context, string, int) ([]dao.Operation, error) {
	r.listCalls++
	return nil, nil
}
