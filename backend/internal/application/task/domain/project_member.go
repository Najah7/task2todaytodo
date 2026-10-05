package domain

import (
	"errors"
	"strings"
)

var ErrProjectMemberInvalid = errors.New("project member requires project, user, role, and actor IDs")

// ProjectMember is the project's explicit membership grant. Its role is a
// globally managed RBAC role; project members cannot define permissions.
type ProjectMember struct {
	ProjectID ProjectID
	UserID    UserID
	RoleID    string
	AddedBy   UserID
}

func NewProjectMember(projectID ProjectID, userID UserID, roleID string, addedBy UserID) (ProjectMember, error) {
	member := ProjectMember{ProjectID: projectID, UserID: userID, RoleID: strings.TrimSpace(roleID), AddedBy: addedBy}
	if member.ProjectID == "" || member.UserID == "" || member.RoleID == "" || member.AddedBy == "" {
		return ProjectMember{}, ErrProjectMemberInvalid
	}
	return member, nil
}
