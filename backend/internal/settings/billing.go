package settings

import (
	"fmt"
	"strings"
)

type BillingSettings struct {
	Enabled       bool
	StripeAPIKey  string
	WebhookSecret string
	ProPriceID    string
	TeamPriceID   string
}

func loadBillingSettings(value func(string, string) string) (BillingSettings, error) {
	settings := BillingSettings{
		StripeAPIKey:  value("STRIPE_SECRET_KEY", ""),
		WebhookSecret: value("STRIPE_WEBHOOK_SECRET", ""),
		ProPriceID:    value("STRIPE_PRO_PRICE_ID", ""),
		TeamPriceID:   value("STRIPE_TEAM_PRICE_ID", ""),
	}
	configured := 0
	for _, field := range []string{settings.StripeAPIKey, settings.WebhookSecret, settings.ProPriceID, settings.TeamPriceID} {
		if strings.TrimSpace(field) != "" {
			configured++
		}
	}
	if configured == 0 {
		return settings, nil
	}
	if configured != 4 {
		return BillingSettings{}, fmt.Errorf("Stripe billing requires STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET, STRIPE_PRO_PRICE_ID, and STRIPE_TEAM_PRICE_ID")
	}
	if !(strings.HasPrefix(settings.StripeAPIKey, "sk_test_") || strings.HasPrefix(settings.StripeAPIKey, "sk_live_")) || !strings.HasPrefix(settings.WebhookSecret, "whsec_") {
		return BillingSettings{}, fmt.Errorf("Stripe billing credentials have invalid format")
	}
	if !strings.HasPrefix(settings.ProPriceID, "price_") || !strings.HasPrefix(settings.TeamPriceID, "price_") || settings.ProPriceID == settings.TeamPriceID {
		return BillingSettings{}, fmt.Errorf("Stripe billing price identifiers are invalid")
	}
	settings.Enabled = true
	return settings, nil
}
