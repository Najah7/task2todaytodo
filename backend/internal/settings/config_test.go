package settings

import (
	"encoding/base64"
	"log/slog"
	"testing"
)

func TestLoadUsesDefaultsAndEnvironment(t *testing.T) {
	clearBillingEnvironment(t)
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	env := map[string]string{
		"PAGE_TOKEN_KEY": key,
		"SERVER_ADDR":    "127.0.0.1:9090",
		"POSTGRES_USER":  "alice",
	}
	for key, value := range env {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Addr != "127.0.0.1:9090" {
		t.Errorf("Server.Addr = %q, want custom address", cfg.Server.Addr)
	}
	if cfg.Application.Service != defaultService || cfg.Application.Environment != defaultEnvironment || cfg.Application.Version != defaultAppVersion || cfg.Logger.LogLevel != slog.LevelInfo {
		t.Errorf("runtime metadata = %q/%q/%q level %v, want defaults and INFO", cfg.Application.Service, cfg.Application.Environment, cfg.Application.Version, cfg.Logger.LogLevel)
	}
	if cfg.Database.User != "alice" || cfg.Database.Password != "changeme" || cfg.Database.Host != "localhost" || cfg.Database.Port != "5432" || cfg.Database.Name != "task2todaytodo" || cfg.Database.SSLMode != "disable" {
		t.Errorf("Database = %+v, want configured user and existing defaults", cfg.Database)
	}
	if string(cfg.Secrets.PageTokenKey) != "01234567890123456789012345678901" {
		t.Errorf("Secrets.PageTokenKey decoded incorrectly")
	}
	if cfg.Email.Provider != "mailpit" || cfg.Email.SMTPHost != "localhost" || cfg.Email.SMTPPort != 1025 || cfg.Email.FrontendBaseURL != "http://localhost:5173" {
		t.Errorf("Email = %+v, want local Mailpit defaults", cfg.Email)
	}
}

func TestLoadBillingConfiguration(t *testing.T) {
	clearBillingEnvironment(t)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	t.Setenv("STRIPE_PRO_PRICE_ID", "price_pro")
	t.Setenv("STRIPE_TEAM_PRICE_ID", "price_team")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Billing.Enabled || cfg.Billing.ProPriceID != "price_pro" || cfg.Billing.TeamPriceID != "price_team" {
		t.Errorf("Billing configuration = %+v, want configured Pro and Team prices", cfg.Billing)
	}

	t.Setenv("STRIPE_TEAM_PRICE_ID", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted partial Stripe configuration")
	}
}

func TestLoadRejectsInvalidBillingIdentifiers(t *testing.T) {
	clearBillingEnvironment(t)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_example")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_example")
	t.Setenv("STRIPE_PRO_PRICE_ID", "prod_not_a_price")
	t.Setenv("STRIPE_TEAM_PRICE_ID", "price_team")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a Product ID where Price ID is required")
	}
}

func clearBillingEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("PAGE_TOKEN_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	for _, name := range []string{"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRO_PRICE_ID", "STRIPE_TEAM_PRICE_ID"} {
		t.Setenv(name, "")
	}
}

func TestLoadValidatesFrontendURLAndEmailProvider(t *testing.T) {
	t.Setenv("PAGE_TOKEN_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("EMAIL_PROVIDER", "ses")
	t.Setenv("FRONTEND_BASE_URL", "http://app.example.com")
	if _, err := Load(); err == nil || err.Error() != "FRONTEND_BASE_URL must use HTTPS in production" {
		t.Fatalf("Load() error = %v, want HTTPS validation", err)
	}
	t.Setenv("FRONTEND_BASE_URL", "https://app.example.com")
	t.Setenv("ENVIRONMENT", "staging")
	t.Setenv("EMAIL_PROVIDER", "smtp")
	if _, err := Load(); err == nil || err.Error() != "EMAIL_PROVIDER must be mailpit or ses" {
		t.Fatalf("Load() error = %v, want provider validation", err)
	}
}

func TestProductionRequiresSESAndHTTPSEvenForLocalhost(t *testing.T) {
	t.Setenv("PAGE_TOKEN_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("FRONTEND_BASE_URL", "https://app.example.com")
	t.Setenv("EMAIL_PROVIDER", "mailpit")
	if _, err := Load(); err == nil || err.Error() != "EMAIL_PROVIDER must be ses in production" {
		t.Fatalf("Load() error = %v, want production SES requirement", err)
	}
	t.Setenv("EMAIL_PROVIDER", "ses")
	t.Setenv("FRONTEND_BASE_URL", "http://localhost:5173")
	if _, err := Load(); err == nil || err.Error() != "FRONTEND_BASE_URL must use HTTPS in production" {
		t.Fatalf("Load() error = %v, want production HTTPS requirement", err)
	}
}

func TestLoadReadsRuntimeMetadataAndLogLevel(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	env := map[string]string{
		"PAGE_TOKEN_KEY":    key,
		"OTEL_SERVICE_NAME": "worker-api",
		"ENVIRONMENT":       "staging",
		"APP_VERSION":       "2.3.4",
		"LOG_LEVEL":         " warn ",
	}
	for key, value := range env {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Application.Service != "worker-api" || cfg.Application.Environment != "staging" || cfg.Application.Version != "2.3.4" || cfg.Logger.LogLevel != slog.LevelWarn {
		t.Errorf("runtime metadata = %q/%q/%q level %v", cfg.Application.Service, cfg.Application.Environment, cfg.Application.Version, cfg.Logger.LogLevel)
	}
}

func TestLoadResolvesResourceMetadataAndAttributes(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	env := map[string]string{
		"PAGE_TOKEN_KEY":           key,
		"ENVIRONMENT":              "staging",
		"EMAIL_PROVIDER":           "ses",
		"FRONTEND_BASE_URL":        "https://localhost:5173",
		"APP_VERSION":              "2.3.4",
		"OTEL_RESOURCE_ATTRIBUTES": "service.name=resource-api,deployment.environment.name=production,service.version=3.0,custom.path=hello%2Fworld,custom.plus=a+b,custom.spaces=%20%20,custom.value=first,custom.value=last%3Dvalue,invalid.escape= raw%ZZ ,custom.tail=ok",
		"OTEL_SERVICE_NAME":        "  final-api  ",
	}
	for key, value := range env {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Application.Service != "final-api" || cfg.Application.Environment != "production" || cfg.Application.Version != "3.0" {
		t.Errorf("resolved metadata = %q/%q/%q, want OTEL service override and resource attributes", cfg.Application.Service, cfg.Application.Environment, cfg.Application.Version)
	}
	wantAttributes := map[string]string{
		"service.name":                "resource-api",
		"deployment.environment.name": "production",
		"service.version":             "3.0",
		"custom.path":                 "hello/world",
		"custom.plus":                 "a+b",
		"custom.spaces":               "  ",
		"custom.tail":                 "ok",
		"custom.value":                "last=value",
		"invalid.escape":              " raw%ZZ ",
	}
	if len(cfg.Logger.ResourceAttributes) != len(wantAttributes) {
		t.Fatalf("ResourceAttributes = %#v, want %#v", cfg.Logger.ResourceAttributes, wantAttributes)
	}
	for name, want := range wantAttributes {
		if got := cfg.Logger.ResourceAttributes[name]; got != want {
			t.Errorf("ResourceAttributes[%q] = %q, want %q", name, got, want)
		}
	}
	t.Setenv("OTEL_SERVICE_NAME", "")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() without OTEL_SERVICE_NAME error = %v", err)
	}
	if cfg.Application.Service != "resource-api" {
		t.Errorf("Service without OTEL_SERVICE_NAME = %q, want service.name resource attribute", cfg.Application.Service)
	}
}

func TestLoadBlankResourceAttributesDoNotOverrideMetadata(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	env := map[string]string{
		"PAGE_TOKEN_KEY":           key,
		"ENVIRONMENT":              "test",
		"APP_VERSION":              "4.5.6",
		"OTEL_RESOURCE_ATTRIBUTES": "service.name=,deployment.environment.name=  ,service.version=",
		"OTEL_SERVICE_NAME":        "   ",
	}
	for key, value := range env {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Application.Service != defaultService || cfg.Application.Environment != "test" || cfg.Application.Version != "4.5.6" {
		t.Errorf("resolved metadata = %q/%q/%q, want defaults/env values after blank overrides", cfg.Application.Service, cfg.Application.Environment, cfg.Application.Version)
	}
}

func TestLoadRejectsMalformedResourceAttributes(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("PAGE_TOKEN_KEY", key)
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=api,missing-separator")
	_, err := Load()
	if err == nil || err.Error() != "OTEL_RESOURCE_ATTRIBUTES entries must use key=value format" {
		t.Errorf("Load() error = %v, want safe malformed resource attributes error", err)
	}
}

func TestLoadRejectsUnknownLogLevel(t *testing.T) {
	t.Setenv("PAGE_TOKEN_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("LOG_LEVEL", "verbose")
	_, err := Load()

	if err == nil || err.Error() != "LOG_LEVEL must be DEBUG, INFO, WARN, or ERROR" {
		t.Errorf("Load() error = %v, want invalid log level error", err)
	}
}

func TestLoadDefaultServerAddress(t *testing.T) {
	t.Setenv("PAGE_TOKEN_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Addr != defaultServerAddr {
		t.Errorf("Server.Addr = %q, want %q", cfg.Server.Addr, defaultServerAddr)
	}
}

func TestLoadRejectsInvalidPageTokenKey(t *testing.T) {
	for _, value := range []string{"", "not-base64", base64.StdEncoding.EncodeToString([]byte("short"))} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("PAGE_TOKEN_KEY", value)
			_, err := Load()
			if err == nil || err.Error() != "PAGE_TOKEN_KEY must be base64-encoded 32-byte key" {
				t.Errorf("Load() error = %v, want invalid key error", err)
			}
		})
	}
}
