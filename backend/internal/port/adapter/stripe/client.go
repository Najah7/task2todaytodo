package stripe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

const apiBaseURL = "https://api.stripe.com/v1"

type Client struct {
	apiKey         string
	proPriceID     string
	teamPriceID    string
	httpClient     *http.Client
	portalMu       sync.Mutex
	portalConfigID string
}

func NewClient(apiKey, proPriceID, teamPriceID string) *Client {
	return &Client{apiKey: apiKey, proPriceID: proPriceID, teamPriceID: teamPriceID, httpClient: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) CreateCustomer(ctx context.Context, userID, email, idempotencyKey string) (string, error) {
	values := url.Values{}
	values.Set("email", email)
	values.Set("metadata[user_id]", userID)
	var response struct {
		ID string `json:"id"`
	}
	if err := c.request(ctx, http.MethodPost, "/customers", values, idempotencyKey, &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("Stripe customer response had no identifier")
	}
	return response.ID, nil
}

func (c *Client) CreateCheckout(ctx context.Context, customerID, userID, priceID, successURL, cancelURL, idempotencyKey string) (dao.CheckoutReservation, error) {
	values := url.Values{}
	values.Set("mode", "subscription")
	values.Set("customer", customerID)
	values.Set("client_reference_id", userID)
	values.Set("line_items[0][price]", priceID)
	values.Set("line_items[0][quantity]", "1")
	values.Set("success_url", successURL+"&session_id={CHECKOUT_SESSION_ID}")
	values.Set("cancel_url", cancelURL)
	values.Set("metadata[user_id]", userID)
	values.Set("subscription_data[metadata][user_id]", userID)
	// One user has at most one subscription. The database reservation supplies
	// the idempotency key by context to avoid creating concurrent sessions.
	var response struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := c.request(ctx, http.MethodPost, "/checkout/sessions", values, idempotencyKey, &response); err != nil {
		return dao.CheckoutReservation{}, err
	}
	if response.ID == "" || response.URL == "" || response.ExpiresAt == 0 {
		return dao.CheckoutReservation{}, errors.New("Stripe Checkout response was incomplete")
	}
	expires := time.Unix(response.ExpiresAt, 0).UTC()
	return dao.CheckoutReservation{SessionID: response.ID, URL: response.URL, ExpiresAt: &expires}, nil
}

func (c *Client) GetCheckout(ctx context.Context, sessionID string) (dao.CheckoutReservation, error) {
	var response struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := c.request(ctx, http.MethodGet, "/checkout/sessions/"+url.PathEscape(sessionID), nil, "", &response); err != nil {
		return dao.CheckoutReservation{}, err
	}
	expires := time.Unix(response.ExpiresAt, 0).UTC()
	return dao.CheckoutReservation{SessionID: response.ID, URL: response.URL, ExpiresAt: &expires}, nil
}

func (c *Client) CreatePortal(ctx context.Context, customerID, returnURL string) (string, error) {
	configurationID, err := c.portalConfiguration(ctx)
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("customer", customerID)
	values.Set("return_url", returnURL)
	values.Set("configuration", configurationID)
	var response struct {
		URL string `json:"url"`
	}
	if err := c.request(ctx, http.MethodPost, "/billing_portal/sessions", values, "", &response); err != nil {
		return "", err
	}
	if response.URL == "" {
		return "", errors.New("Stripe Portal response had no URL")
	}
	return response.URL, nil
}

func (c *Client) portalConfiguration(ctx context.Context) (string, error) {
	c.portalMu.Lock()
	defer c.portalMu.Unlock()
	if c.portalConfigID != "" {
		return c.portalConfigID, nil
	}
	values := url.Values{}
	values.Set("features[subscription_cancel][enabled]", "true")
	values.Set("features[subscription_cancel][mode]", "at_period_end")
	values.Set("features[subscription_cancel][proration_behavior]", "none")
	values.Set("features[subscription_update][enabled]", "false")
	values.Set("features[invoice_history][enabled]", "true")
	values.Set("features[payment_method_update][enabled]", "true")
	var response struct {
		ID string `json:"id"`
	}
	if err := c.request(ctx, http.MethodPost, "/billing_portal/configurations", values, "task2today-billing-portal-v1", &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("Stripe Portal configuration response had no identifier")
	}
	c.portalConfigID = response.ID
	return response.ID, nil
}

func (c *Client) RetrieveSubscription(ctx context.Context, subscriptionID string) (dao.Subscription, error) {
	var response stripeSubscription
	if err := c.request(ctx, http.MethodGet, "/subscriptions/"+url.PathEscape(subscriptionID), url.Values{"expand[]": {"items.data.price"}}, "", &response); err != nil {
		return dao.Subscription{}, err
	}
	return c.mapSubscription(response)
}

func (c *Client) UpgradeSubscription(ctx context.Context, subscriptionID, currentPriceID, targetPriceID string) (dao.Subscription, error) {
	sub, err := c.RetrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return dao.Subscription{}, err
	}
	if sub.PriceID != currentPriceID {
		return dao.Subscription{}, errors.New("Stripe subscription changed before upgrade")
	}
	targetPlan := dao.Plan("")
	if targetPriceID == c.proPriceID {
		targetPlan = dao.PlanPro
	}
	if targetPriceID == c.teamPriceID {
		targetPlan = dao.PlanTeam
	}
	if (sub.PendingPlan == nil && sub.Plan == targetPlan) || (sub.PendingPlan != nil && *sub.PendingPlan == targetPlan) {
		return sub, nil
	}
	itemID, err := c.subscriptionItemID(ctx, subscriptionID)
	if err != nil {
		return dao.Subscription{}, err
	}
	values := url.Values{}
	values.Set("items[0][id]", itemID)
	values.Set("items[0][price]", targetPriceID)
	values.Set("items[0][quantity]", "1")
	values.Set("proration_behavior", "always_invoice")
	values.Set("payment_behavior", "pending_if_incomplete")
	values.Add("expand[]", "items.data.price")
	values.Add("expand[]", "latest_invoice")
	var response stripeSubscription
	if err := c.request(ctx, http.MethodPost, "/subscriptions/"+url.PathEscape(subscriptionID), values, randomIdempotencyKey("upgrade_"+subscriptionID), &response); err != nil {
		return sub, err
	}
	return c.mapSubscription(response)
}

func (c *Client) ScheduleDowngrade(ctx context.Context, subscriptionID, currentPriceID, targetPriceID, _ string) (string, error) {
	current, err := c.RetrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return "", err
	}
	if current.PriceID != currentPriceID {
		return "", errors.New("Stripe subscription changed before downgrade")
	}
	if current.ScheduleID != "" {
		existing, err := c.retrieveSchedule(ctx, current.ScheduleID)
		if err != nil {
			return "", err
		}
		if scheduleTargetsPrice(existing, targetPriceID) {
			return current.ScheduleID, nil
		}
		if err := c.ReleaseSchedule(ctx, current.ScheduleID); err != nil {
			return "", err
		}
	}
	var created struct {
		ID string `json:"id"`
	}
	values := url.Values{}
	values.Set("from_subscription", subscriptionID)
	if err := c.request(ctx, http.MethodPost, "/subscription_schedules", values, randomIdempotencyKey("schedule_"+subscriptionID), &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("Stripe schedule response had no identifier")
	}
	schedule, err := c.retrieveSchedule(ctx, created.ID)
	if err != nil {
		return "", c.releaseAfterScheduleSetupFailure(ctx, created.ID, err)
	}
	phase := schedule.phaseForCurrentPeriod()
	if phase == nil || phaseStartDate(phase) == 0 || phaseEndDate(phase) == 0 {
		return "", c.releaseAfterScheduleSetupFailure(ctx, created.ID, errors.New("Stripe schedule has no current phase"))
	}
	items, _ := phase["items"].([]any)
	if len(items) == 0 {
		return "", c.releaseAfterScheduleSetupFailure(ctx, created.ID, errors.New("Stripe schedule has no current item"))
	}
	firstItem, _ := items[0].(map[string]any)
	if objectID(firstItem["price"]) != currentPriceID && objectID(firstItem["plan"]) != currentPriceID {
		return "", c.releaseAfterScheduleSetupFailure(ctx, created.ID, errors.New("Stripe subscription price changed during downgrade"))
	}
	form := url.Values{}
	form.Set("end_behavior", "release")
	copySchedulePhase(form, 0, phase)
	form.Set("phases[0][start_date]", strconv.FormatInt(phaseStartDate(phase), 10))
	form.Set("phases[0][end_date]", strconv.FormatInt(phaseEndDate(phase), 10))
	form.Set("phases[0][items][0][price]", currentPriceID)
	form.Set("phases[0][items][0][quantity]", "1")
	form.Set("phases[0][proration_behavior]", "none")
	copySchedulePhase(form, 1, phase)
	form.Set("phases[1][start_date]", strconv.FormatInt(phaseEndDate(phase), 10))
	form.Set("phases[1][items][0][price]", targetPriceID)
	form.Set("phases[1][items][0][quantity]", "1")
	form.Set("phases[1][proration_behavior]", "none")
	var updated stripeSchedule
	if err := c.request(ctx, http.MethodPost, "/subscription_schedules/"+url.PathEscape(created.ID), form, "", &updated); err != nil {
		return "", c.releaseAfterScheduleSetupFailure(ctx, created.ID, err)
	}
	return created.ID, nil
}

func (c *Client) releaseAfterScheduleSetupFailure(ctx context.Context, scheduleID string, cause error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := c.ReleaseSchedule(cleanupCtx, scheduleID); err != nil {
		return errors.Join(cause, fmt.Errorf("release incomplete Stripe schedule: %w", err))
	}
	return cause
}

func scheduleTargetsPrice(schedule stripeSchedule, priceID string) bool {
	if schedule.CurrentPhase == nil {
		return false
	}
	for _, phase := range schedule.Phases {
		if phaseInt(phase["start_date"]) < schedule.CurrentPhase.EndDate {
			continue
		}
		items, _ := phase["items"].([]any)
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			if objectID(item["price"]) == priceID || objectID(item["plan"]) == priceID {
				return true
			}
		}
	}
	return false
}

func randomIdempotencyKey(prefix string) string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return prefix + "_" + hex.EncodeToString(value[:])
}

func (c *Client) ReleaseSchedule(ctx context.Context, scheduleID string) error {
	return c.request(ctx, http.MethodPost, "/subscription_schedules/"+url.PathEscape(scheduleID)+"/release", url.Values{}, "", &map[string]any{})
}

func (c *Client) subscriptionItemID(ctx context.Context, subscriptionID string) (string, error) {
	var response struct {
		Items struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"items"`
	}
	if err := c.request(ctx, http.MethodGet, "/subscriptions/"+url.PathEscape(subscriptionID), nil, "", &response); err != nil {
		return "", err
	}
	if len(response.Items.Data) != 1 || response.Items.Data[0].ID == "" {
		return "", errors.New("Stripe subscription must have one price item")
	}
	return response.Items.Data[0].ID, nil
}

func (c *Client) request(ctx context.Context, method, path string, values url.Values, idempotencyKey string, destination any) error {
	var body io.Reader
	if method == http.MethodGet && len(values) > 0 {
		path += "?" + values.Encode()
	} else if values != nil {
		body = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBaseURL+path, body)
	if err != nil {
		return fmt.Errorf("create Stripe request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call Stripe API: %w", err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 1<<20)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(limited).Decode(&failure)
		if failure.Error.Code != "" {
			return fmt.Errorf("Stripe API rejected request (%s)", failure.Error.Code)
		}
		return fmt.Errorf("Stripe API returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(limited).Decode(destination); err != nil {
		return fmt.Errorf("decode Stripe response: %w", err)
	}
	return nil
}

type stripeSubscription struct {
	ID                 string `json:"id"`
	Created            int64  `json:"created"`
	Customer           string `json:"customer"`
	Status             string `json:"status"`
	CurrentPeriodStart int64  `json:"current_period_start"`
	CurrentPeriodEnd   int64  `json:"current_period_end"`
	CancelAtPeriodEnd  bool   `json:"cancel_at_period_end"`
	PendingUpdate      any    `json:"pending_update"`
	Schedule           any    `json:"schedule"`
	Items              struct {
		Data []struct {
			ID                 string `json:"id"`
			CurrentPeriodStart int64  `json:"current_period_start"`
			CurrentPeriodEnd   int64  `json:"current_period_end"`
			Price              struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

func (c *Client) FindCurrentSubscription(ctx context.Context, customerID string) (dao.Subscription, bool, error) {
	values := url.Values{}
	values.Set("customer", customerID)
	values.Set("status", "all")
	values.Set("limit", "100")
	values.Add("expand[]", "data.items.data.price")
	var response struct {
		Data    []stripeSubscription `json:"data"`
		HasMore bool                 `json:"has_more"`
	}
	if err := c.request(ctx, http.MethodGet, "/subscriptions", values, "", &response); err != nil {
		return dao.Subscription{}, false, err
	}
	for _, candidate := range response.Data {
		switch candidate.Status {
		case "active", "trialing", "past_due", "unpaid", "paused", "incomplete":
			sub, err := c.mapSubscription(candidate)
			return sub, true, err
		}
	}
	if response.HasMore {
		return dao.Subscription{}, false, errors.New("Stripe customer has too many subscriptions to safely verify checkout")
	}
	return dao.Subscription{}, false, nil
}

type stripeSchedule struct {
	CurrentPhase *struct {
		StartDate int64 `json:"start_date"`
		EndDate   int64 `json:"end_date"`
	} `json:"current_phase"`
	Phases []map[string]any `json:"phases"`
}

func (c *Client) retrieveSchedule(ctx context.Context, id string) (stripeSchedule, error) {
	var schedule stripeSchedule
	err := c.request(ctx, http.MethodGet, "/subscription_schedules/"+url.PathEscape(id), nil, "", &schedule)
	return schedule, err
}

func (s stripeSchedule) phaseForCurrentPeriod() map[string]any {
	if s.CurrentPhase == nil {
		return nil
	}
	for _, phase := range s.Phases {
		if phaseInt(phase["start_date"]) == s.CurrentPhase.StartDate && phaseInt(phase["end_date"]) == s.CurrentPhase.EndDate {
			return phase
		}
	}
	return nil
}

func phaseStartDate(phase map[string]any) int64 { return phaseInt(phase["start_date"]) }
func phaseEndDate(phase map[string]any) int64   { return phaseInt(phase["end_date"]) }

func phaseInt(value any) int64 {
	switch value := value.(type) {
	case float64:
		return int64(value)
	case json.Number:
		i, _ := value.Int64()
		return i
	case int64:
		return value
	default:
		return 0
	}
}

func copySchedulePhase(values url.Values, index int, phase map[string]any) {
	for _, key := range []string{"application_fee_percent", "billing_cycle_anchor", "collection_method", "default_payment_method", "default_source", "default_tax_rates", "description", "discounts", "invoice_settings", "metadata", "on_behalf_of", "proration_behavior", "transfer_data", "trial_end"} {
		if value, exists := phase[key]; exists {
			flattenStripeForm(values, fmt.Sprintf("phases[%d][%s]", index, key), value)
		}
	}
	if items, ok := phase["items"].([]any); ok {
		for itemIndex, raw := range items {
			item, _ := raw.(map[string]any)
			for _, key := range []string{"price", "quantity", "tax_rates", "metadata"} {
				if value, exists := item[key]; exists {
					flattenStripeForm(values, fmt.Sprintf("phases[%d][items][%d][%s]", index, itemIndex, key), value)
				}
			}
		}
	}
}

func flattenStripeForm(values url.Values, key string, value any) {
	switch value := value.(type) {
	case nil:
		values.Set(key, "")
	case string:
		values.Set(key, value)
	case float64:
		values.Set(key, strconv.FormatFloat(value, 'f', -1, 64))
	case json.Number:
		values.Set(key, value.String())
	case bool:
		values.Set(key, strconv.FormatBool(value))
	case []any:
		for i, child := range value {
			flattenStripeForm(values, fmt.Sprintf("%s[%d]", key, i), child)
		}
	case map[string]any:
		for childKey, child := range value {
			flattenStripeForm(values, key+"["+childKey+"]", child)
		}
	}
}

func (c *Client) mapSubscription(response stripeSubscription) (dao.Subscription, error) {
	if response.ID == "" || response.Customer == "" {
		return dao.Subscription{}, errors.New("Stripe subscription response was incomplete")
	}
	var plan dao.Plan
	if len(response.Items.Data) > 0 {
		switch response.Items.Data[0].Price.ID {
		case c.proPriceID:
			plan = dao.PlanPro
		case c.teamPriceID:
			plan = dao.PlanTeam
		default:
			return dao.Subscription{}, errors.New("Stripe subscription has an unconfigured Price")
		}
	}
	priceID := ""
	if len(response.Items.Data) > 0 {
		priceID = response.Items.Data[0].Price.ID
	}
	sub := dao.Subscription{ID: response.ID, Created: response.Created, CustomerID: response.Customer, Status: response.Status, Plan: plan, PriceID: priceID, CancelAtPeriodEnd: response.CancelAtPeriodEnd, ScheduleID: objectID(response.Schedule)}
	if pending, ok := response.PendingUpdate.(map[string]any); ok {
		if items, ok := pending["subscription_items"].([]any); ok && len(items) > 0 {
			item, _ := items[0].(map[string]any)
			pendingPrice := objectID(item["price"])
			var pendingPlan dao.Plan
			switch pendingPrice {
			case c.proPriceID:
				pendingPlan = dao.PlanPro
			case c.teamPriceID:
				pendingPlan = dao.PlanTeam
			}
			if pendingPlan != "" {
				sub.PendingPlan = &pendingPlan
			}
		}
	}
	periodStart, periodEnd := response.CurrentPeriodStart, response.CurrentPeriodEnd
	if len(response.Items.Data) > 0 {
		if response.Items.Data[0].CurrentPeriodStart > 0 {
			periodStart = response.Items.Data[0].CurrentPeriodStart
		}
		if response.Items.Data[0].CurrentPeriodEnd > 0 {
			periodEnd = response.Items.Data[0].CurrentPeriodEnd
		}
	}
	if periodStart > 0 {
		value := time.Unix(periodStart, 0).UTC()
		sub.CurrentPeriodStart = &value
	}
	if periodEnd > 0 {
		value := time.Unix(periodEnd, 0).UTC()
		sub.CurrentPeriodEnd = &value
	}
	return sub, nil
}

func objectID(value any) string {
	if id, ok := value.(string); ok {
		return id
	}
	if object, ok := value.(map[string]any); ok {
		id, _ := object["id"].(string)
		return id
	}
	return ""
}

func checkoutIdempotencyKey(ctx context.Context) string {
	key, _ := ctx.Value(checkoutKeyContext{}).(string)
	return key
}

type checkoutKeyContext struct{}

func WithCheckoutIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, checkoutKeyContext{}, key)
}
