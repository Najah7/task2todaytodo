package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type WebhookRepository interface {
	RecordWebhookEvent(context.Context, string, int64) (bool, error)
	ClearExpiredCheckoutBySession(context.Context, string) error
	GetSnapshotCursorForUpdate(context.Context, string) (dao.SnapshotCursor, error)
	ApplySnapshot(context.Context, string, dao.Subscription, string, string, string, *time.Time, int64) error
}

type webhookStripe interface {
	RetrieveSubscription(context.Context, string) (dao.Subscription, error)
	ReleaseSchedule(context.Context, string) error
}

type ProcessWebhookUseCase struct {
	uow     UnitOfWork
	stripe  webhookStripe
	enabled bool
}

func NewProcessWebhookUseCase(uow UnitOfWork, stripe webhookStripe, enabled bool) *ProcessWebhookUseCase {
	return &ProcessWebhookUseCase{uow: uow, stripe: stripe, enabled: enabled}
}

func (u *ProcessWebhookUseCase) Execute(ctx context.Context, eventID, eventType string, created int64, object map[string]any) error {
	if !u.enabled {
		return ErrDisabled
	}
	if eventType == "checkout.session.expired" {
		if sessionID, _ := object["id"].(string); sessionID != "" {
			return u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
				repo := repositories.Webhook()
				inserted, err := repo.RecordWebhookEvent(ctx, eventID, created)
				if err != nil || !inserted {
					return err
				}
				return repo.ClearExpiredCheckoutBySession(ctx, sessionID)
			})
		}
		return nil
	}
	subscriptionID := ""
	switch eventType {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		subscriptionID = objectString(object["subscription"])
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted", "customer.subscription.paused", "customer.subscription.resumed":
		subscriptionID = objectString(object["id"])
	case "invoice.paid", "invoice.payment_succeeded", "invoice.payment_failed", "invoice.payment_action_required":
		subscriptionID = invoiceSubscriptionID(object)
	default:
		return u.recordEvent(ctx, eventID, created)
	}
	if subscriptionID == "" {
		return u.recordEvent(ctx, eventID, created)
	}
	sub, err := u.stripe.RetrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return err
	}
	if sub.CancelAtPeriodEnd && sub.ScheduleID != "" {
		if err := u.stripe.ReleaseSchedule(ctx, sub.ScheduleID); err != nil {
			return err
		}
		sub, err = u.stripe.RetrieveSubscription(ctx, subscriptionID)
		if err != nil {
			return err
		}
	}
	return u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		repo := repositories.Webhook()
		inserted, err := repo.RecordWebhookEvent(ctx, eventID, created)
		if err != nil || !inserted {
			return err
		}
		cursor, err := repo.GetSnapshotCursorForUpdate(ctx, sub.CustomerID)
		if err != nil {
			return err
		}
		plan, pendingPlan, scheduleID, pendingAt, apply := snapshotValues(cursor, created, sub)
		if !apply {
			return nil
		}
		return repo.ApplySnapshot(ctx, cursor.UserID, sub, plan, pendingPlan, scheduleID, pendingAt, created)
	})
}

func (u *ProcessWebhookUseCase) recordEvent(ctx context.Context, eventID string, created int64) error {
	return u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		_, err := repositories.Webhook().RecordWebhookEvent(ctx, eventID, created)
		return err
	})
}

func objectString(value any) string {
	if id, ok := value.(string); ok {
		return id
	}
	if object, ok := value.(map[string]any); ok {
		id, _ := object["id"].(string)
		return id
	}
	return ""
}

func invoiceSubscriptionID(object map[string]any) string {
	if id := objectString(object["subscription"]); id != "" {
		return id
	}
	parent, _ := object["parent"].(map[string]any)
	details, _ := parent["subscription_details"].(map[string]any)
	return objectString(details["subscription"])
}
