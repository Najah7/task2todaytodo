package usecase

import authdao "github.com/Najah7/task2todaytodo/internal/application/auth/dao"

type ProjectMemberRoleOption struct {
	ID         string
	Name       string
	Assignable bool
}

type ProjectMemberRoleCapability struct {
	ID, Name                     string
	Owner, Admin, Editor, Viewer bool
}

// ProjectMemberRoleCapabilities derives standard grants from the role catalog.
// The owner inherits the admin catalog and receives only the project-owner
// capabilities that are intentionally outside the stored member role grants.
func ProjectMemberRoleCapabilities(catalog []authdao.Role) ([]ProjectMemberRoleOption, []ProjectMemberRoleCapability) {
	roles := []ProjectMemberRoleOption{
		{ID: "owner", Name: "Owner"}, {ID: "admin", Name: "Administrator"},
		{ID: "editor", Name: "Editor"}, {ID: "viewer", Name: "Viewer"},
	}
	labels := []ProjectMemberRoleCapability{
		{ID: "project.read", Name: "View project"}, {ID: "project.update", Name: "Edit project"}, {ID: "project.delete", Name: "Delete project"},
		{ID: "task.read", Name: "View tasks"}, {ID: "task.create", Name: "Create tasks"}, {ID: "task.update", Name: "Edit tasks"}, {ID: "task.delete", Name: "Delete tasks"},
		{ID: "action_item.read", Name: "View action items"}, {ID: "action_item.create", Name: "Create action items"}, {ID: "action_item.update", Name: "Edit action items"}, {ID: "action_item.delete", Name: "Delete action items"},
		{ID: "schedule.read", Name: "View schedules"}, {ID: "schedule.create", Name: "Create schedules"}, {ID: "schedule.update", Name: "Edit schedules"}, {ID: "schedule.delete", Name: "Delete schedules"},
		{ID: "task_assignment.update", Name: "Reassign tasks"}, {ID: "schedule_assignment.update", Name: "Reassign schedules"}, {ID: "occurrence.update", Name: "Update schedule occurrences"},
		{ID: "project_member.read", Name: "View members and invitations"}, {ID: "project_member.invite", Name: "Invite members"},
		{ID: "project_member.manage_basic", Name: "Manage editor and viewer roles"}, {ID: "project_member.manage_admin", Name: "Manage administrator roles"},
	}
	capabilityIndex := make(map[string]int, len(labels))
	capabilities := make([]ProjectMemberRoleCapability, len(labels))
	for index, label := range labels {
		capabilityIndex[label.ID] = index
		capabilities[index] = label
	}
	roleAllows := map[string]map[string]bool{"admin": {}, "editor": {}, "viewer": {}}
	for _, role := range catalog {
		if _, ok := roleAllows[role.ID]; !ok {
			continue
		}
		for _, permission := range role.Permissions {
			id := permission.ResourceID + "." + permission.Action
			if _, ok := capabilityIndex[id]; !ok {
				continue
			}
			if permission.Effect == "deny" {
				roleAllows[role.ID][id] = false
			} else if permission.Effect == "allow" {
				roleAllows[role.ID][id] = true
			}
		}
	}
	for index := range capabilities {
		capability := &capabilities[index]
		capability.Admin = roleAllows["admin"][capability.ID]
		capability.Editor = roleAllows["editor"][capability.ID]
		capability.Viewer = roleAllows["viewer"][capability.ID]
		capability.Owner = capability.Admin
	}
	for i := range capabilities {
		switch capabilities[i].ID {
		case "project.delete":
			capabilities[i].Owner = true
			capabilities[i].Admin = false
			capabilities[i].Editor = false
			capabilities[i].Viewer = false
		case "project_member.manage_admin":
			capabilities[i].Owner = true
		case "project_member.invite":
			capabilities[i].Owner = true
			capabilities[i].Admin = true
		case "project_member.manage_basic":
			capabilities[i].Owner = true
			capabilities[i].Admin = true
		}
	}
	return roles, capabilities
}

func CanInviteProjectMembers(actorRole string) bool {
	return actorRole == "owner" || actorRole == "admin"
}

func AssignableProjectMemberRoles(actorRole string) []ProjectMemberRoleOption {
	roles := []ProjectMemberRoleOption{
		{ID: "admin", Name: "Administrator", Assignable: actorRole == "owner"},
		{ID: "editor", Name: "Editor", Assignable: actorRole == "owner" || actorRole == "admin"},
		{ID: "viewer", Name: "Viewer", Assignable: actorRole == "owner" || actorRole == "admin"},
	}
	return roles
}

func CanManageProjectMemberRole(actorRole, targetRole string) bool {
	if actorRole == "owner" {
		return targetRole == "admin" || targetRole == "editor" || targetRole == "viewer"
	}
	return actorRole == "admin" && (targetRole == "editor" || targetRole == "viewer")
}
