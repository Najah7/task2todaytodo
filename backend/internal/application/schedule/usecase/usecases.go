package usecase

type ScheduleUseCases struct {
	CreateBlueprint           *CreateBlueprintUseCase
	GetBlueprint              *GetBlueprintUseCase
	ListBlueprintRevisions    *ListBlueprintRevisionsUseCase
	OccurrenceRevisions       *ListOccurrenceRevisionsUseCase
	UpdateBlueprint           *UpdateBlueprintUseCase
	DeleteBlueprintFollowing  *DeleteBlueprintFollowingUseCase
	ListOccurrences           *ListOccurrencesUseCase
	GetOccurrence             *GetOccurrenceUseCase
	UpdateOccurrence          *UpdateOccurrenceUseCase
	CompleteOccurrence        *SetOccurrenceStateUseCase
	ReopenOccurrence          *SetOccurrenceStateUseCase
	SkipOccurrence            *SetOccurrenceStateUseCase
	RestoreOccurrence         *SetOccurrenceStateUseCase
	DeleteOccurrence          *SetOccurrenceStateUseCase
	RescheduleOccurrence      *RescheduleOccurrenceUseCase
	Backfill                  ScheduleBackfill
	ProjectBlueprintLifecycle ProjectBlueprintLifecycle
	ListAssignees             *ListScheduleAssigneesUseCase
	ListBlueprintTags         *ListBlueprintTagsUseCase
	AddTagToBlueprint         *AddTagToBlueprintUseCase
	RemoveTagFromBlueprint    *RemoveTagFromBlueprintUseCase
}
