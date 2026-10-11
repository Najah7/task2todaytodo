package usecase

type TaskUseCases struct {
	Create            *CreateTaskUseCase
	List              *ListTasksUseCase
	Get               *GetTaskUseCase
	Update            *UpdateTaskUseCase
	Delete            *DeleteTaskUseCase
	Start             *StartTaskUseCase
	Hold              *HoldTaskUseCase
	Wait              *WaitTaskUseCase
	Complete          *CompleteTaskUseCase
	Reopen            *ReopenTaskUseCase
	Assign            *AssignTaskUseCase
	ListAssignees     *ListTaskAssigneesUseCase
	CreateInProject   *CreateTaskInProjectUseCase
	ListByProject     *ListProjectTasksUseCase
	AddToProject      *AddTaskToProjectUseCase
	RemoveFromProject *RemoveTaskFromProjectUseCase
	Revisions         *ListTaskRevisionsUseCase
}

type ActionItemUseCases struct {
	Create   *CreateActionItemUseCase
	List     *ListActionItemsUseCase
	Update   *UpdateActionItemUseCase
	Delete   *DeleteActionItemUseCase
	Complete *CompleteActionItemUseCase
	Reopen   *ReopenActionItemUseCase
	Reorder  *ReorderActionItemUseCase
}
