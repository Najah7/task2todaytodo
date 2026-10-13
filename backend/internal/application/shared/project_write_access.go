package shared

import "context"

import "errors"

var ErrTeamSubscriptionRequired = errors.New("Team subscription is required for shared Project writes")

// ProjectWriteAccess contains the persisted facts a writer needs before
// changing a Project or its shared work. MemberCount counts actual membership
// rows; pending invitations do not make a Project shared.
type ProjectWriteAccess struct {
	OwnerUserID     string
	MemberCount     int64
	ActorPlanAccess PlanAccess
	OwnerPlanAccess PlanAccess
}

// ProjectWriteAccessReader reads Project membership and effective plan access
// from the transaction used by the operation being authorized.
type ProjectWriteAccessReader interface {
	GetProjectWriteAccess(context.Context, string, string) (ProjectWriteAccess, error)
}

func (access ProjectWriteAccess) AllowsWrite() bool {
	if access.MemberCount == 0 {
		return true
	}
	return access.ActorPlanAccess.IsTeam() && access.OwnerPlanAccess.IsTeam()
}
