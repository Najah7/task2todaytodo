package config

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const defaultServerAddr = ":8080"
const (
	defaultService     = "api"
	defaultEnvironment = "development"
	defaultAppVersion  = "1.0.0"
)

// Config contains validated process settings required to start the API.
type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	Secrets     Secrets
	Application ApplicationConfig
}

// Load reads and validates API settings from the process environment.
func Load() (Config, error) {
	return LoadFromEnv(os.Getenv)
}

// LoadFromEnv reads and validates API settings through getenv.
func LoadFromEnv(getenv func(string) string) (Config, error) {
	value := func(key, fallback string) string {
		if value := getenv(key); value != "" {
			return value
		}
		return fallback
	}

	encodedKey := getenv("PAGE_TOKEN_KEY")
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return Config{}, fmt.Errorf("PAGE_TOKEN_KEY must be base64-encoded 32-byte key")
	}
	level, err := parseLogLevel(value("LOG_LEVEL", "INFO"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Server: ServerConfig{
			Addr: value("SERVER_ADDR", defaultServerAddr),
		},
		Database: DatabaseConfig{
			User:     value("POSTGRES_USER", "changeme"),
			Password: value("POSTGRES_PASSWORD", "changeme"),
			Host:     value("POSTGRES_HOST", "localhost"),
			Port:     value("POSTGRES_PORT", "5432"),
			Name:     value("POSTGRES_DB", "task2todaytodo"),
			SSLMode:  value("POSTGRES_SSLMODE", "disable"),
		},
		Secrets: Secrets{
			PageTokenKey: key,
		},
		Application: ApplicationConfig{
			Service:     value("OTEL_SERVICE_NAME", defaultService),
			Environment: value("ENVIRONMENT", defaultEnvironment),
			Version:     value("APP_VERSION", defaultAppVersion),
			LogLevel:    level,
		},
	}, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be DEBUG, INFO, WARN, or ERROR")
	}
}
