package usecase

import "context"

type Repositories interface {
	Checkout() CheckoutRepository
	Webhook() WebhookRepository
	PlanChange() PlanChangeRepository
}

type UnitOfWork interface {
	Do(context.Context, func(context.Context, Repositories) error) error
}
