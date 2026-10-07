package usecase

import "github.com/Najah7/task2todaytodo/internal/logging"

type ProjectMemberUseCases struct {
	ListMembers  *ListProjectMembersUseCase
	UpsertMember *UpsertProjectMemberUseCase
	DeleteMember *DeleteProjectMemberUseCase
}
type UseCases struct {
	Create    *CreateProjectUseCase
	List      *ListProjectsUseCase
	Get       *GetProjectUseCase
	Update    *UpdateProjectUseCase
	Delete    *DeleteProjectUseCase
	Members   *ProjectMemberUseCases
	Revisions *ListProjectRevisionsUseCase
	Types     *ListProjectTypesUseCase
}

func NewUseCases(repo Repository, tx UnitOfWork, logger logging.Logger) *UseCases {
	return &UseCases{
		Create:    NewCreateProjectUseCase(repo, logger),
		List:      NewListProjectsUseCase(repo, repo, logger),
		Get:       NewGetProjectUseCase(repo, repo, logger),
		Update:    NewUpdateProjectUseCase(repo, repo, logger),
		Delete:    NewDeleteProjectUseCase(tx, logger),
		Members:   &ProjectMemberUseCases{ListMembers: NewListProjectMembersUseCase(tx, logger), UpsertMember: NewUpsertProjectMemberUseCase(tx, logger), DeleteMember: NewDeleteProjectMemberUseCase(tx, logger)},
		Revisions: NewListProjectRevisionsUseCase(repo, logger),
		Types:     NewListProjectTypesUseCase(repo, logger),
	}
}
