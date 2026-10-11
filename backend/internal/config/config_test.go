package config

import (
	"encoding/base64"
	"log/slog"
	"testing"
)

func TestLoadFromEnvUsesDefaultsAndEnvironment(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	env := map[string]string{
		"PAGE_TOKEN_KEY": key,
		"SERVER_ADDR":    "127.0.0.1:9090",
		"POSTGRES_USER":  "alice",
	}
	cfg, err := LoadFromEnv(func(name string) string { return env[name] })
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Server.Addr != "127.0.0.1:9090" {
		t.Errorf("Server.Addr = %q, want custom address", cfg.Server.Addr)
	}
	if cfg.Application.Service != defaultService || cfg.Application.Environment != defaultEnvironment || cfg.Application.Version != defaultAppVersion || cfg.Application.LogLevel != slog.LevelInfo {
		t.Errorf("runtime metadata = %q/%q/%q level %v, want defaults and INFO", cfg.Application.Service, cfg.Application.Environment, cfg.Application.Version, cfg.Application.LogLevel)
	}
	if cfg.Database.User != "alice" || cfg.Database.Password != "changeme" || cfg.Database.Host != "localhost" || cfg.Database.Port != "5432" || cfg.Database.Name != "task2todaytodo" || cfg.Database.SSLMode != "disable" {
		t.Errorf("Database = %+v, want configured user and existing defaults", cfg.Database)
	}
	if string(cfg.Secrets.PageTokenKey) != "01234567890123456789012345678901" {
		t.Errorf("Secrets.PageTokenKey decoded incorrectly")
	}
}

func TestLoadFromEnvReadsRuntimeMetadataAndLogLevel(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	env := map[string]string{
		"PAGE_TOKEN_KEY":    key,
		"OTEL_SERVICE_NAME": "worker-api",
		"ENVIRONMENT":       "staging",
		"APP_VERSION":       "2.3.4",
		"LOG_LEVEL":         " warn ",
	}
	cfg, err := LoadFromEnv(func(name string) string { return env[name] })
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Application.Service != "worker-api" || cfg.Application.Environment != "staging" || cfg.Application.Version != "2.3.4" || cfg.Application.LogLevel != slog.LevelWarn {
		t.Errorf("runtime metadata = %q/%q/%q level %v", cfg.Application.Service, cfg.Application.Environment, cfg.Application.Version, cfg.Application.LogLevel)
	}
}

func TestLoadFromEnvRejectsUnknownLogLevel(t *testing.T) {
	_, err := LoadFromEnv(func(name string) string {
		switch name {
		case "PAGE_TOKEN_KEY":
			return base64.StdEncoding.EncodeToString(make([]byte, 32))
		case "LOG_LEVEL":
			return "verbose"
		default:
			return ""
		}
	})
	if err == nil || err.Error() != "LOG_LEVEL must be DEBUG, INFO, WARN, or ERROR" {
		t.Errorf("LoadFromEnv() error = %v, want invalid log level error", err)
	}
}

func TestLoadFromEnvDefaultServerAddress(t *testing.T) {
	cfg, err := LoadFromEnv(func(name string) string {
		if name == "PAGE_TOKEN_KEY" {
			return base64.StdEncoding.EncodeToString(make([]byte, 32))
		}
		return ""
	})
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Server.Addr != defaultServerAddr {
		t.Errorf("Server.Addr = %q, want %q", cfg.Server.Addr, defaultServerAddr)
	}
}

func TestLoadFromEnvRejectsInvalidPageTokenKey(t *testing.T) {
	for _, value := range []string{"", "not-base64", base64.StdEncoding.EncodeToString([]byte("short"))} {
		t.Run(value, func(t *testing.T) {
			_, err := LoadFromEnv(func(name string) string {
				if name == "PAGE_TOKEN_KEY" {
					return value
				}
				return ""
			})
			if err == nil || err.Error() != "PAGE_TOKEN_KEY must be base64-encoded 32-byte key" {
				t.Errorf("LoadFromEnv() error = %v, want invalid key error", err)
			}
		})
	}
}
