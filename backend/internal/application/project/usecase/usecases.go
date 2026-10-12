package usecase

import "github.com/Najah7/task2todaytodo/internal/telemetry"

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

func NewUseCases(repo Repository, tx UnitOfWork, logger telemetry.Logger, readTxs ...UnitOfWork) *UseCases {
	readTx := tx
	if len(readTxs) > 0 && readTxs[0] != nil {
		readTx = readTxs[0]
	}
	return &UseCases{
		Create:       NewCreateProjectUseCase(repo, logger),
		List:         NewListProjectsUseCase(repo, repo, logger),
		Get:          NewGetProjectUseCase(repo, repo, logger),
		Update:       NewUpdateProjectUseCase(repo, repo, logger),
		Delete:       NewDeleteProjectUseCase(tx, logger),
		Members:      &ProjectMemberUseCases{ListMembers: NewListProjectMembersUseCase(readTx, logger), UpsertMember: NewUpsertProjectMemberUseCase(tx, logger), DeleteMember: NewDeleteProjectMemberUseCase(tx, logger)},
		Revisions:    NewListProjectRevisionsUseCase(repo, logger),
		Types:        NewListProjectTypesUseCase(repo, logger),
		Options:      NewListProjectOptionsUseCase(repo, logger),
		ChangeStatus: NewChangeProjectStatusUseCase(tx, repo, logger),
		Restore:      NewRestoreProjectUseCase(tx, repo, logger),
	}
}
