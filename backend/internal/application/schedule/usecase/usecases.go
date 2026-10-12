package usecase

type ScheduleUseCases struct {
	CreateRule           *CreateRuleUseCase
	GetRule              *GetRuleUseCase
	ListRuleRevisions    *ListRuleRevisionsUseCase
	OccurrenceRevisions  *ListOccurrenceRevisionsUseCase
	UpdateRule           *UpdateRuleUseCase
	DeleteRuleFollowing  *DeleteRuleFollowingUseCase
	ListOccurrences      *ListOccurrencesUseCase
	GetOccurrence        *GetOccurrenceUseCase
	UpdateOccurrence     *UpdateOccurrenceUseCase
	CompleteOccurrence   *SetOccurrenceStateUseCase
	ReopenOccurrence     *SetOccurrenceStateUseCase
	SkipOccurrence       *SetOccurrenceStateUseCase
	RestoreOccurrence    *SetOccurrenceStateUseCase
	DeleteOccurrence     *SetOccurrenceStateUseCase
	RescheduleOccurrence *RescheduleOccurrenceUseCase
	Backfill             ScheduleBackfill
	ProjectRuleLifecycle ProjectRuleLifecycle
	ListAssignees        *ListScheduleAssigneesUseCase
}
