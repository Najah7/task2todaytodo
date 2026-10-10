package usecase

import "github.com/Najah7/task2todaytodo/internal/logging"

type ProjectMemberUseCases struct {
	ListMembers  *ListProjectMembersUseCase
	UpsertMember *UpsertProjectMemberUseCase
	DeleteMember *DeleteProjectMemberUseCase
}
type UseCases struct {
	Create       *CreateProjectUseCase
	List         *ListProjectsUseCase
	Get          *GetProjectUseCase
	Update       *UpdateProjectUseCase
	Delete       *DeleteProjectUseCase
	Members      *ProjectMemberUseCases
	Revisions    *ListProjectRevisionsUseCase
	Types        *ListProjectTypesUseCase
	Options      *ListProjectOptionsUseCase
	ChangeStatus *ChangeProjectStatusUseCase
	Restore      *RestoreProjectUseCase
}

func NewUseCases(repo Repository, tx UnitOfWork, logger logging.Logger) *UseCases {
	return NewUseCasesWithProgressReader(repo, tx, repo, logger)
}

func NewUseCasesWithProgressReader(repo Repository, tx UnitOfWork, progress ProjectProgressReader, logger logging.Logger) *UseCases {
	return &UseCases{
		Create:       NewCreateProjectUseCase(repo, logger),
		List:         NewListProjectsUseCase(repo, progress, logger),
		Get:          NewGetProjectUseCase(repo, progress, logger),
		Update:       NewUpdateProjectUseCase(repo, progress, logger),
		Delete:       NewDeleteProjectUseCase(tx, logger),
		Members:      &ProjectMemberUseCases{ListMembers: NewListProjectMembersUseCase(tx, logger), UpsertMember: NewUpsertProjectMemberUseCase(tx, logger), DeleteMember: NewDeleteProjectMemberUseCase(tx, logger)},
		Revisions:    NewListProjectRevisionsUseCase(repo, logger),
		Types:        NewListProjectTypesUseCase(repo, logger),
		Options:      NewListProjectOptionsUseCase(repo, logger),
		ChangeStatus: NewChangeProjectStatusUseCase(tx, progress, logger),
		Restore:      NewRestoreProjectUseCase(tx, progress, logger),
	}
}
