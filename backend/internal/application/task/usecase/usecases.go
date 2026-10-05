package usecase

type ProjectUseCases struct {
	Create     *CreateProjectUseCase
	List       *ListProjectsUseCase
	Get        *GetProjectUseCase
	Update     *UpdateProjectUseCase
	Delete     *DeleteProjectUseCase
	CreateTask *CreateTaskInProjectUseCase
	ListTasks  *ListProjectTasksUseCase
	AddTask    *AddTaskToProjectUseCase
	RemoveTask *RemoveTaskFromProjectUseCase
	Members    *ProjectMemberUseCases
	Revisions  *ListProjectRevisionsUseCase
}

type TaskUseCases struct {
	Create        *CreateTaskUseCase
	List          *ListTasksUseCase
	Get           *GetTaskUseCase
	Update        *UpdateTaskUseCase
	Delete        *DeleteTaskUseCase
	Start         *StartTaskUseCase
	Hold          *HoldTaskUseCase
	Wait          *WaitTaskUseCase
	Complete      *CompleteTaskUseCase
	Reopen        *ReopenTaskUseCase
	Assign        *AssignTaskUseCase
	ListAssignees *ListTaskAssigneesUseCase
	Revisions     *ListTaskRevisionsUseCase
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

type TaskScheduleUseCases struct {
	Create          *CreateTaskScheduleUseCase
	List            *ListTaskSchedulesUseCase
	Update          *UpdateTaskScheduleUseCase
	Delete          *DeleteTaskScheduleUseCase
	Complete        *CompleteTaskScheduleUseCase
	Reopen          *ReopenTaskScheduleUseCase
	Skip            *SkipTaskScheduleUseCase
	Restore         *RestoreTaskScheduleUseCase
	Reschedule      *RescheduleTaskScheduleUseCase
	UpdateFrequency *UpdateTaskScheduleFrequencyUseCase
}

type TaskTagUseCases struct {
	Create         *CreateTaskTagUseCase
	List           *ListTaskTagsUseCase
	Rename         *RenameTaskTagUseCase
	Delete         *DeleteTaskTagUseCase
	AddToTask      *AddTagToTaskUseCase
	RemoveFromTask *RemoveTagFromTaskUseCase
}
