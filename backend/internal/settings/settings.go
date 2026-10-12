package settings

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

const defaultServerAddr = ":8080"
const (
	defaultService     = "api"
	defaultEnvironment = "development"
	defaultAppVersion  = "1.0.0"
)

// Settings contains validated process settings required to start the API.
type Settings struct {
	Server      ServerSettings
	Database    DatabaseSettings
	Secrets     Secrets
	Email       EmailSettings
	Application ApplicationSettings
	Logger      LoggerSettings
}

// Load reads and validates API settings through getenv.
func Load() (Settings, error) {
	value := func(key, fallback string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return fallback
	}

	encodedKey := os.Getenv("PAGE_TOKEN_KEY")
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != 32 {
		return Settings{}, fmt.Errorf("PAGE_TOKEN_KEY must be base64-encoded 32-byte key")
	}
	level, err := parseLogLevel(value("LOG_LEVEL", "INFO"))
	if err != nil {
		return Settings{}, err
	}
	resourceAttributes, err := parseResourceAttributes(os.Getenv("OTEL_RESOURCE_ATTRIBUTES"))
	if err != nil {
		return Settings{}, err
	}

	service := defaultService
	environment := value("ENVIRONMENT", defaultEnvironment)
	version := value("APP_VERSION", defaultAppVersion)
	if resolved := resourceAttributes["service.name"]; resolved != "" {
		service = resolved
	}
	if resolved := resourceAttributes["deployment.environment.name"]; resolved != "" {
		environment = resolved
	}
	if resolved := resourceAttributes["service.version"]; resolved != "" {
		version = resolved
	}
	if resolved := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); resolved != "" {
		service = resolved
	}
	email, err := loadEmailSettings(value, environment)
	if err != nil {
		return Settings{}, err
	}

	return Settings{
		Server: ServerSettings{
			Addr: value("SERVER_ADDR", defaultServerAddr),
		},
		Database: DatabaseSettings{
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
		Email: email,
		Application: ApplicationSettings{
			Service:     service,
			Environment: environment,
			Version:     version,
		},
		Logger: LoggerSettings{
			ResourceAttributes: resourceAttributes,
			LogLevel:           level,
		},
	}, nil
}

func parseResourceAttributes(raw string) (map[string]string, error) {
	attributes := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return attributes, nil
	}
	for _, pair := range strings.Split(raw, ",") {
		key, rawValue, found := strings.Cut(pair, "=")
		if !found {
			return nil, fmt.Errorf("OTEL_RESOURCE_ATTRIBUTES entries must use key=value format")
		}
		key = strings.TrimSpace(key)
		value, err := url.PathUnescape(strings.TrimSpace(rawValue))
		if err != nil {
			// Match the SDK: invalid escapes retain the untrimmed original value.
			value = rawValue
		}
		attributes[key] = value
	}
	return attributes, nil
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
