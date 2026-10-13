package rest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
	billingusecase "github.com/Najah7/task2todaytodo/internal/application/billing/usecase"
	stripeadapter "github.com/Najah7/task2todaytodo/internal/port/adapter/stripe"
)

type BillingHandler struct {
	getStatus      *billingusecase.GetStatusUseCase
	startCheckout  *billingusecase.StartCheckoutUseCase
	createPortal   *billingusecase.CreatePortalSessionUseCase
	changePlan     *billingusecase.ChangePlanUseCase
	processWebhook *billingusecase.ProcessWebhookUseCase
	webhookSecret  string
}

func NewBillingHandler(usecases billingusecase.UseCases, webhookSecret string) *BillingHandler {
	return &BillingHandler{getStatus: usecases.GetStatus, startCheckout: usecases.StartCheckout, createPortal: usecases.CreatePortalSession, changePlan: usecases.ChangePlan, processWebhook: usecases.ProcessWebhook, webhookSecret: webhookSecret}
}

type BillingCheckoutRequest struct {
	Plan string `json:"plan"`
}
type BillingURLResponse struct {
	URL string `json:"url"`
}

type BillingStateResponse = dao.State

// Status reads current billing state for the authenticated user.
//
//	@Summary	Get current billing state
//	@ID		GetBillingStatus
//	@Tags		Billing
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	BillingStateResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	503	{object}	ErrResponse
//	@Router		/billing [get]
func (h *BillingHandler) Status(w http.ResponseWriter, r *http.Request) {
	actor, ok := billingActor(w, r)
	if !ok {
		return
	}
	state, err := h.getStatus.Execute(r.Context(), actor)
	if err != nil {
		writeBillingError(w, ErrSpecBillingReadFailed, err)
		return
	}
	WriteJSON(w, http.StatusOK, state)
}

// Checkout creates or reuses a subscription Checkout session for the selected plan.
//
//	@Summary	Start subscription Checkout
//	@ID		StartBillingCheckout
//	@Tags		Billing
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	BillingCheckoutRequest	true	"Plan"
//	@Success	200	{object}	BillingURLResponse
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	409	{object}	ErrResponse
//	@Failure	503	{object}	ErrResponse
//	@Router		/billing/checkout [post]
func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	actor, ok := billingActor(w, r)
	if !ok {
		return
	}
	var request BillingCheckoutRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&request); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecBillingCheckoutFailed, ErrDetailInvalidRequestBody)
		return
	}
	url, err := h.startCheckout.Execute(r.Context(), actor, request.Plan)
	if err != nil {
		writeBillingError(w, ErrSpecBillingCheckoutFailed, err)
		return
	}
	WriteJSON(w, http.StatusOK, BillingURLResponse{URL: url})
}

// ChangePlan upgrades immediately or schedules a downgrade for the next renewal.
//
//	@Summary	Change subscription plan
//	@ID		ChangeBillingPlan
//	@Tags		Billing
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	BillingCheckoutRequest	true	"Target plan"
//	@Success	200	{object}	BillingStateResponse
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	409	{object}	ErrResponse
//	@Failure	503	{object}	ErrResponse
//	@Router		/billing/plan [post]
func (h *BillingHandler) ChangePlan(w http.ResponseWriter, r *http.Request) {
	actor, ok := billingActor(w, r)
	if !ok {
		return
	}
	var request BillingCheckoutRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&request); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecBillingPlanChangeFailed, ErrDetailInvalidRequestBody)
		return
	}
	state, err := h.changePlan.Execute(r.Context(), actor, request.Plan)
	if err != nil {
		writeBillingError(w, ErrSpecBillingPlanChangeFailed, err)
		return
	}
	WriteJSON(w, http.StatusOK, state)
}

// Portal creates a Stripe Customer Portal session for billing self-service.
//
//	@Summary	Open billing portal
//	@ID		CreateBillingPortalSession
//	@Tags		Billing
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	BillingURLResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	503	{object}	ErrResponse
//	@Router		/billing/portal [post]
func (h *BillingHandler) Portal(w http.ResponseWriter, r *http.Request) {
	actor, ok := billingActor(w, r)
	if !ok {
		return
	}
	url, err := h.createPortal.Execute(r.Context(), actor)
	if err != nil {
		writeBillingError(w, ErrSpecBillingPortalFailed, err)
		return
	}
	WriteJSON(w, http.StatusOK, BillingURLResponse{URL: url})
}

// Webhook accepts Stripe's signed event delivery. It intentionally uses the raw request bytes.
func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid request body", http.StatusRequestEntityTooLarge)
		return
	}
	if err := stripeadapter.VerifyWebhookSignature(body, r.Header.Get("Stripe-Signature"), h.webhookSecret, time.Now()); err != nil {
		http.Error(w, "invalid Stripe signature", http.StatusBadRequest)
		return
	}
	var event struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Created int64  `json:"created"`
		Data    struct {
			Object map[string]any `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil || event.ID == "" || event.Type == "" {
		http.Error(w, "invalid Stripe event", http.StatusBadRequest)
		return
	}
	if err := h.processWebhook.Execute(r.Context(), event.ID, event.Type, event.Created, event.Data.Object); err != nil {
		if errors.Is(err, billingusecase.ErrDisabled) {
			http.Error(w, "billing unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Stripe event processing failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func billingActor(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || actor == "" {
		WriteError(w, http.StatusUnauthorized, ErrSpecBillingReadFailed, ErrDetailUnauthorized)
		return "", false
	}
	return actor, true
}

func writeBillingError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, billingusecase.ErrInvalidPlan):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("plan", "invalid_value", "Plan must be pro or team"))
	case errors.Is(err, billingusecase.ErrConflict):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("plan", "conflict", "Billing request conflicts with current subscription"))
	case errors.Is(err, billingusecase.ErrNotFound):
		WriteError(w, http.StatusNotFound, spec, ErrDetailNotFound)
	case errors.Is(err, billingusecase.ErrDisabled):
		WriteError(w, http.StatusServiceUnavailable, spec, NewErrDetail("billing", "unavailable", "Billing is not configured"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}
