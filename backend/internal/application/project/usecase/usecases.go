package usecase

import "github.com/Najah7/task2todaytodo/internal/telemetry"
import "github.com/Najah7/task2todaytodo/internal/application/shared"

type ProjectMemberUseCases struct {
	ListMembers  *ListProjectMembersUseCase
	UpsertMember *UpsertProjectMemberUseCase
	DeleteMember *DeleteProjectMemberUseCase
}

type ProjectInvitationUseCases struct {
	Create  *CreateProjectInvitationUseCase
	Resend  *ResendProjectInvitationUseCase
	Revoke  *RevokeProjectInvitationUseCase
	Preview *PreviewProjectInvitationUseCase
}

type UseCases struct {
	Create       *CreateProjectUseCase
	List         *ListProjectsUseCase
	Get          *GetProjectUseCase
	Update       *UpdateProjectUseCase
	Delete       *DeleteProjectUseCase
	Members      *ProjectMemberUseCases
	Invitations  *ProjectInvitationUseCases
	Revisions    *ListProjectRevisionsUseCase
	Types        *ListProjectTypesUseCase
	Options      *ListProjectOptionsUseCase
	ChangeStatus *ChangeProjectStatusUseCase
	Restore      *RestoreProjectUseCase
}

func NewUseCases(repo Repository, tx UnitOfWork, logger telemetry.Logger, invitationSender ProjectInvitationSender, frontendURL string, readTxs ...UnitOfWork) *UseCases {
	return newUseCases(repo, tx, logger, invitationSender, frontendURL, nil, readTxs...)
}

func NewUseCasesWithAccess(repo Repository, tx UnitOfWork, logger telemetry.Logger, invitationSender ProjectInvitationSender, frontendURL string, access shared.PlanAccessReader, readTxs ...UnitOfWork) *UseCases {
	return newUseCases(repo, tx, logger, invitationSender, frontendURL, access, readTxs...)
}

func newUseCases(repo Repository, tx UnitOfWork, logger telemetry.Logger, invitationSender ProjectInvitationSender, frontendURL string, access shared.PlanAccessReader, readTxs ...UnitOfWork) *UseCases {
	readTx := tx
	if len(readTxs) > 0 && readTxs[0] != nil {
		readTx = readTxs[0]
	}
	return &UseCases{
		Create:  NewCreateProjectUseCase(tx, access, logger),
		List:    NewListProjectsUseCase(repo, repo, logger, readTx),
		Get:     NewGetProjectUseCase(repo, repo, logger, readTx),
		Update:  NewUpdateProjectUseCase(repo, repo, logger, tx),
		Delete:  NewDeleteProjectUseCase(tx, logger),
		Members: &ProjectMemberUseCases{ListMembers: NewListProjectMembersUseCase(readTx, logger), UpsertMember: NewUpsertProjectMemberUseCase(tx, logger), DeleteMember: NewDeleteProjectMemberUseCase(tx, logger)},
		Invitations: &ProjectInvitationUseCases{
			Create:  NewCreateProjectInvitationUseCase(tx, invitationSender, frontendURL, logger),
			Resend:  NewResendProjectInvitationUseCase(tx, invitationSender, frontendURL, logger),
			Revoke:  NewRevokeProjectInvitationUseCase(tx, logger),
			Preview: NewPreviewProjectInvitationUseCase(repo, logger),
		},
		Revisions:    NewListProjectRevisionsUseCase(repo, logger),
		Types:        NewListProjectTypesUseCase(repo, logger),
		Options:      NewListProjectOptionsUseCase(repo, logger),
		ChangeStatus: NewChangeProjectStatusUseCase(tx, repo, logger, access),
		Restore:      NewRestoreProjectUseCase(tx, repo, logger, access),
	}
}
