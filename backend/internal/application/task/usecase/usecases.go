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

type TodoItemUseCases struct {
	Create          *CreateTodoItemUseCase
	List            *ListTodoItemsUseCase
	Update          *UpdateTodoItemUseCase
	Delete          *DeleteTodoItemUseCase
	Complete        *CompleteTodoItemUseCase
	Reopen          *ReopenTodoItemUseCase
	Skip            *SkipTodoItemUseCase
	Restore         *RestoreTodoItemUseCase
	Reorder         *ReorderTodoItemUseCase
	UpdateFrequency *UpdateTodoItemFrequencyUseCase
}

type TaskTagUseCases struct {
	AddToTask      *AddTagToTaskUseCase
	RemoveFromTask *RemoveTagFromTaskUseCase
}
