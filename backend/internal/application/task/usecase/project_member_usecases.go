package usecase

type ProjectMemberUseCases struct {
	ListMembers  *ListProjectMembersUseCase
	UpsertMember *UpsertProjectMemberUseCase
	DeleteMember *DeleteProjectMemberUseCase
}
