package application

import (
	"context"

	billingrepository "github.com/Najah7/task2todaytodo/internal/application/billing/repository"
	billingusecase "github.com/Najah7/task2todaytodo/internal/application/billing/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BillingUOW struct {
	pool *pgxpool.Pool
	repo *billingrepository.Repository
	run  transactionRunner
}

func NewBillingUOW(pool *pgxpool.Pool, repo *billingrepository.Repository) billingusecase.UnitOfWork {
	return &BillingUOW{pool: pool, repo: repo, run: RunInTx}
}

func (u *BillingUOW) Do(ctx context.Context, fn func(context.Context, billingusecase.Repositories) error) error {
	return u.run(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, billingRepositories{repo: u.repo.WithTx(tx)})
	})
}

type billingRepositories struct{ repo *billingrepository.Repository }

func (r billingRepositories) Checkout() billingusecase.CheckoutRepository     { return r.repo }
func (r billingRepositories) Webhook() billingusecase.WebhookRepository       { return r.repo }
func (r billingRepositories) PlanChange() billingusecase.PlanChangeRepository { return r.repo }
