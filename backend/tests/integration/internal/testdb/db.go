//go:build integration

// Package testdb provides the shared, narrowly scoped database setup for
// PostgreSQL integration tests.
package testdb

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const databaseNamePrefix = "task2todaytodo_integration_"

// Open connects to the dedicated database selected by the integration runner.
// It rejects application and legacy audit databases before opening a pool.
func Open(t testing.TB) *pgxpool.Pool {
	return OpenWithApplicationName(t, "")
}

// OpenWithApplicationName opens the test database with an optional PostgreSQL
// application_name, used by concurrency tests to observe lock waits.
func OpenWithApplicationName(t testing.TB, applicationName string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Fatal("INTEGRATION_DATABASE_URL is required for integration tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse INTEGRATION_DATABASE_URL: %v", err)
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, databaseNamePrefix) {
		t.Fatalf("refusing database %q; integration tests require a dedicated %s* database", cfg.ConnConfig.Database, databaseNamePrefix)
	}
	if applicationName != "" {
		cfg.ConnConfig.RuntimeParams["application_name"] = applicationName
	}
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	if err := pool.Ping(t.Context()); err != nil {
		pool.Close()
		t.Fatalf("ping integration database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// RegisterShutdown runs an application cleanup before the database pool cleanup
// registered by Open, using a bounded context so integration tests cannot hang.
func RegisterShutdown(t testing.TB, shutdown func(context.Context) error) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown application: %v", err)
		}
	})
}
