package usecase

type ScheduleUseCases struct {
	Revisions            *ListScheduleRevisionsUseCase
	Create               *CreateScheduleUseCase
	List                 *ListSchedulesUseCase
	ListProject          *ListProjectSchedulesUseCase
	Get                  *GetScheduleUseCase
	Update               *UpdateScheduleUseCase
	Delete               *DeleteScheduleUseCase
	Complete             *CompleteScheduleUseCase
	Reopen               *ReopenScheduleUseCase
	Skip                 *SkipScheduleUseCase
	Restore              *RestoreScheduleUseCase
	Reschedule           *RescheduleScheduleUseCase
	UpdateFrequency      *UpdateScheduleFrequencyUseCase
	SetProject           *SetScheduleProjectUseCase
	RemoveFromProject    *RemoveScheduleFromProjectUseCase
	Assign               *AssignScheduleUseCase
	ListAssignees        *ListScheduleAssigneesUseCase
	ListTags             *ListScheduleTagsUseCase
	ListTagsForSchedules *ListScheduleTagsForSchedulesUseCase
	AddTag               *AddTagToScheduleUseCase
	RemoveTag            *RemoveTagFromScheduleUseCase
}
