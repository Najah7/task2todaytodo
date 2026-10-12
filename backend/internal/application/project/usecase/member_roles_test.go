package usecase

import (
	"testing"

	authdao "github.com/Najah7/task2todaytodo/internal/application/auth/dao"
)

func TestProjectMemberRoleCapabilitiesUsesRoleCatalogWithOwnerPolicy(t *testing.T) {
	catalog := []authdao.Role{
		{ID: "admin", Permissions: []authdao.Permission{
			{ResourceID: "project", Action: "read", Effect: "allow"},
			{ResourceID: "project", Action: "delete", Effect: "allow"},
			{ResourceID: "task", Action: "create", Effect: "allow"},
			{ResourceID: "project_member", Action: "read", Effect: "allow"},
		}},
		{ID: "editor", Permissions: []authdao.Permission{
			{ResourceID: "project", Action: "read", Effect: "allow"},
			{ResourceID: "project", Action: "update", Effect: "allow"},
			{ResourceID: "task", Action: "create", Effect: "allow"},
			{ResourceID: "project_member", Action: "read", Effect: "allow"},
		}},
		{ID: "viewer", Permissions: []authdao.Permission{
			{ResourceID: "project", Action: "read", Effect: "allow"},
			{ResourceID: "project_member", Action: "read", Effect: "allow"},
		}},
	}
	_, matrix := ProjectMemberRoleCapabilities(catalog)
	byID := make(map[string]ProjectMemberRoleCapability, len(matrix))
	for _, capability := range matrix {
		byID[capability.ID] = capability
	}
	if !byID["task.create"].Admin || !byID["task.create"].Editor || byID["task.create"].Viewer {
		t.Fatalf("task.create should follow catalog grants: %+v", byID["task.create"])
	}
	if byID["project.delete"].Admin || !byID["project.delete"].Owner {
		t.Fatalf("project deletion must remain owner-only: %+v", byID["project.delete"])
	}
	if !byID["project_member.read"].Editor || !byID["project_member.read"].Viewer {
		t.Fatalf("all participants should read the roster: %+v", byID["project_member.read"])
	}
}
