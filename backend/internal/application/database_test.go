package application

import (
	"net/url"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/settings"
)

func TestPostgresConnectionStringEscapesCredentialsAndDatabaseName(t *testing.T) {
	database := settings.DatabaseSettings{
		User:     "user name",
		Password: "p@ss:/?word",
		Host:     "localhost",
		Port:     "5432",
		Name:     "db name",
		SSLMode:  "require",
	}
	parsed, err := url.Parse(postgresConnectionString(database))
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	password, _ := parsed.User.Password()
	if parsed.User.Username() != database.User || password != database.Password {
		t.Errorf("parsed credentials = %q/%q, want configured values", parsed.User.Username(), password)
	}
	if parsed.Path != "/"+database.Name {
		t.Errorf("database path = %q, want %q", parsed.Path, "/"+database.Name)
	}
	if parsed.Query().Get("sslmode") != database.SSLMode {
		t.Errorf("sslmode = %q, want %q", parsed.Query().Get("sslmode"), database.SSLMode)
	}
}

func TestPostgresPoolConfigSetsConnectionLimits(t *testing.T) {
	cfg, err := postgresPoolConfig(settings.DatabaseSettings{
		User: "user", Password: "secret", Host: "localhost", Port: "5432", Name: "db", SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("postgresPoolConfig() error = %v", err)
	}
	if cfg.MaxConns != 10 || cfg.MinConns != 2 {
		t.Errorf("pool limits = %d/%d, want 10/2", cfg.MaxConns, cfg.MinConns)
	}
}
