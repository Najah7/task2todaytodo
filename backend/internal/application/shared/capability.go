package shared

// ResourceKind and PermissionAction are the structured parts of a project
// membership capability. Database policy evaluation also checks the rule
// effect, so application callers never pass dotted permission names.
type ResourceKind string
type PermissionAction string

type Capability struct {
	Resource ResourceKind
	Action   PermissionAction
}

const (
	ResourceProject            ResourceKind = "project"
	ResourceTask               ResourceKind = "task"
	ResourceActionItem         ResourceKind = "action_item"
	ResourceSchedule           ResourceKind = "schedule"
	ResourceTaskAssignment     ResourceKind = "task_assignment"
	ResourceScheduleAssignment ResourceKind = "schedule_assignment"
	ResourceOccurrence         ResourceKind = "occurrence"
	ResourceProjectMember      ResourceKind = "project_member"
	ResourceDeletedHistory     ResourceKind = "deleted_history"
)

const (
	ActionCreate PermissionAction = "create"
	ActionRead   PermissionAction = "read"
	ActionUpdate PermissionAction = "update"
	ActionDelete PermissionAction = "delete"
)

func ProjectRead() Capability      { return Capability{ResourceProject, ActionRead} }
func ProjectUpdate() Capability    { return Capability{ResourceProject, ActionUpdate} }
func ProjectDelete() Capability    { return Capability{ResourceProject, ActionDelete} }
func TaskRead() Capability         { return Capability{ResourceTask, ActionRead} }
func TaskCreate() Capability       { return Capability{ResourceTask, ActionCreate} }
func TaskUpdate() Capability       { return Capability{ResourceTask, ActionUpdate} }
func TaskDelete() Capability       { return Capability{ResourceTask, ActionDelete} }
func AssignmentUpdate() Capability { return Capability{ResourceTaskAssignment, ActionUpdate} }
func ScheduleAssignmentUpdate() Capability {
	return Capability{ResourceScheduleAssignment, ActionUpdate}
}
func OccurrenceUpdate() Capability    { return Capability{ResourceOccurrence, ActionUpdate} }
func ProjectMemberRead() Capability   { return Capability{ResourceProjectMember, ActionRead} }
func ProjectMemberCreate() Capability { return Capability{ResourceProjectMember, ActionCreate} }
func ProjectMemberUpdate() Capability { return Capability{ResourceProjectMember, ActionUpdate} }
func ProjectMemberDelete() Capability { return Capability{ResourceProjectMember, ActionDelete} }
func DeletedHistoryRead() Capability {
	return Capability{ResourceDeletedHistory, ActionRead}
}
func ActionItemCreate() Capability { return Capability{ResourceActionItem, ActionCreate} }
func ActionItemRead() Capability   { return Capability{ResourceActionItem, ActionRead} }
func ActionItemUpdate() Capability { return Capability{ResourceActionItem, ActionUpdate} }
func ActionItemDelete() Capability { return Capability{ResourceActionItem, ActionDelete} }
func ScheduleCreate() Capability   { return Capability{ResourceSchedule, ActionCreate} }
func ScheduleRead() Capability     { return Capability{ResourceSchedule, ActionRead} }
func ScheduleUpdate() Capability   { return Capability{ResourceSchedule, ActionUpdate} }
func ScheduleDelete() Capability   { return Capability{ResourceSchedule, ActionDelete} }
