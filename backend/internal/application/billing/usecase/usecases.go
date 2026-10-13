package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type UseCases struct {
	GetStatus           *GetStatusUseCase
	StartCheckout       *StartCheckoutUseCase
	CreatePortalSession *CreatePortalSessionUseCase
	ChangePlan          *ChangePlanUseCase
	ProcessWebhook      *ProcessWebhookUseCase
}

type accountReader interface {
	GetAccount(context.Context, string) (dao.Account, error)
	GetUserEmail(context.Context, string) (string, error)
}

type stripeGateway interface {
	CreateCustomer(context.Context, string, string, string) (string, error)
	CreateCheckout(context.Context, string, string, string, string, string, string) (dao.CheckoutReservation, error)
	FindCurrentSubscription(context.Context, string) (dao.Subscription, bool, error)
	GetCheckout(context.Context, string) (dao.CheckoutReservation, error)
	CreatePortal(context.Context, string, string) (string, error)
	RetrieveSubscription(context.Context, string) (dao.Subscription, error)
	UpgradeSubscription(context.Context, string, string, string) (dao.Subscription, error)
	ScheduleDowngrade(context.Context, string, string, string, string) (string, error)
	ReleaseSchedule(context.Context, string) error
}

func NewUseCases(repo accountReader, uow UnitOfWork, stripe stripeGateway, enabled bool, proPriceID, teamPriceID, frontendURL string) UseCases {
	return UseCases{
		GetStatus:           NewGetStatusUseCase(repo, enabled),
		StartCheckout:       NewStartCheckoutUseCase(repo, uow, stripe, enabled, proPriceID, teamPriceID, frontendURL),
		CreatePortalSession: NewCreatePortalSessionUseCase(repo, stripe, enabled, frontendURL),
		ChangePlan:          NewChangePlanUseCase(repo, uow, stripe, enabled, proPriceID, teamPriceID),
		ProcessWebhook:      NewProcessWebhookUseCase(uow, stripe, enabled),
	}
}
